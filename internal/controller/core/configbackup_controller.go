// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/objectstorage"
	"github.com/ironcore-dev/network-operator/internal/paused"
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// ConfigBackupReconciler reconciles a ConfigBackup object.
type ConfigBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the config backup.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker

	// ObjectStorage is an optional pre-configured object storage client for Remote backups.
	// If set, it is used instead of creating one from the spec credentials.
	ObjectStorage ObjectStorage
}

// ObjectStorage defines the operations needed for remote config backups.
type ObjectStorage interface {
	PutObject(ctx context.Context, obj *objectstorage.Object) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]objectstorage.Object, error)
	DeleteObjects(ctx context.Context, bucket string, keys ...string) error
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=configbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=configbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *ConfigBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.ConfigBackup)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.V(3).Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.ConfigBackupProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.ConfigBackupProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if isPaused, requeue, err := paused.EnsureCondition(ctx, r.Client, device, obj); isPaused || requeue || err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "configbackup-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityDefault)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "configbackup-controller"); err != nil {
			log.Error(err, "Failed to release device lock")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return ctrl.Result{}, err
	}

	var cfg *provider.ProviderConfig
	if obj.Spec.ProviderConfigRef != nil {
		cfg, err = provider.GetProviderConfig(ctx, r, obj.Namespace, obj.Spec.ProviderConfigRef)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	s := &configBackupScope{
		Device:         device,
		ConfigBackup:   obj,
		Connection:     conn,
		ProviderConfig: cfg,
		Provider:       prov,
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
			if err := r.finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, v1alpha1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.V(3).Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.V(1).Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopy()
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.V(1).Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			// Pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
			if err := r.Patch(ctx, obj.DeepCopy(), client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
		if !equality.Semantic.DeepEqual(orig.Status, obj.Status) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	res, err := r.reconcile(ctx, s)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, apistatus.WrapTerminalError(err)
	}

	return res, nil
}

func (r *ConfigBackupReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.ConfigBackup{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.ConfigBackup)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ConfigBackup{}).
		Named("configbackup").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.AccessControlListDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.ConfigBackupsForProviderConfig),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
		// Watches enqueues ConfigBackups for updates in referenced Device resources.
		// Triggers on create, delete, and update events when the device's effective pause state changes.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToConfigBackups),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					return paused.DevicePausedChanged(e.ObjectOld, e.ObjectNew)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues ConfigBackups when a referenced S3 credentials Secret is created or updated.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.configBackupsForSecret),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

type configBackupScope struct {
	Device         *v1alpha1.Device
	ConfigBackup   *v1alpha1.ConfigBackup
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.ConfigBackupProvider
}

func (r *ConfigBackupReconciler) reconcile(ctx context.Context, s *configBackupScope) (res ctrl.Result, reterr error) { //nolint:gocyclo
	if s.ConfigBackup.Labels == nil {
		s.ConfigBackup.Labels = make(map[string]string)
	}
	s.ConfigBackup.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the ConfigBackup is owned by the Device.
	if !controllerutil.HasControllerReference(s.ConfigBackup) {
		if err := controllerutil.SetOwnerReference(s.Device, s.ConfigBackup, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	var store ObjectStorage
	var err error
	if s.ConfigBackup.Spec.Type == v1alpha1.ConfigBackupTypeRemote {
		store, err = r.objectStorageClient(ctx, s)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	var schedule cron.Schedule
	if s.ConfigBackup.Spec.Schedule != "" {
		schedule, err = cron.ParseStandard(s.ConfigBackup.Spec.Schedule)
		if err != nil {
			conditions.Set(s.ConfigBackup, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ScheduleInvalidReason,
				Message: err.Error(),
			})
			return ctrl.Result{}, reconcile.TerminalError(err)
		}

		// Determine the last backup time. If no backups have been created yet,
		// use the creation timestamp of the resource.
		last := s.ConfigBackup.CreationTimestamp.UTC()
		if s.ConfigBackup.Status.LastBackup != nil {
			last = s.ConfigBackup.Status.LastBackup.Timestamp.UTC()
		}

		// If the next scheduled backup is in the future, requeue until that time.
		// Otherwise, continue to create a backup now.
		if now, next := time.Now().UTC(), schedule.Next(last); next.After(now) {
			s.ConfigBackup.Status.NextScheduledBackup = metav1.NewTime(next)
			r.Recorder.Eventf(s.ConfigBackup, nil, "Normal", "Scheduled", "Reconcile", "Next backup scheduled at %s", next.Format(time.RFC3339))
			return ctrl.Result{RequeueAfter: next.Sub(now)}, nil
		}

		// Update the next scheduled backup time after the backup is created.
		defer func() {
			if reterr != nil {
				return
			}
			next := schedule.Next(time.Now().UTC())
			s.ConfigBackup.Status.NextScheduledBackup = metav1.NewTime(next)
			r.Recorder.Eventf(s.ConfigBackup, nil, "Normal", "Scheduled", "Reconcile", "Next backup scheduled at %s", next.Format(time.RFC3339))
			res.RequeueAfter = time.Until(next)
		}()
	}

	if err := s.Provider.Connect(ctx, s.Connection); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := s.Provider.Disconnect(ctx, s.Connection); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	req := &provider.ConfigBackupRequest{
		ConfigBackup:   s.ConfigBackup,
		ProviderConfig: s.ProviderConfig,
	}

	// Refresh storage and backup inventory status regardless of whether a new backup is created.
	defer func() {
		if s.ConfigBackup.Spec.Type == v1alpha1.ConfigBackupTypeStartup {
			s.ConfigBackup.Status.TotalBackups = new(int32(1))
			s.ConfigBackup.Status.TotalSizeBytes = nil
			s.ConfigBackup.Status.Storage = nil
			return
		}

		var inventory *provider.ConfigBackupInventory
		switch s.ConfigBackup.Spec.Type {
		case v1alpha1.ConfigBackupTypeRemote:
			inventory, err = r.ListRemoteConfigBackups(ctx, store, s)
		default:
			inventory, err = s.Provider.ListConfigBackups(ctx, req)
		}
		if err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, fmt.Errorf("failed to list backups: %w", err)})
			return
		}

		s.ConfigBackup.Status.TotalBackups = new(int32(len(inventory.Backups))) // #nosec G115

		s.ConfigBackup.Status.TotalSizeBytes = nil
		if total, ok := inventory.TotalBackupSizeBytes(); ok {
			s.ConfigBackup.Status.TotalSizeBytes = &total
		}

		// Update the oldest backup timestamp from the inventory.
		// This accounts for backups that may have been created outside of this controller.
		s.ConfigBackup.Status.OldestBackupTimestamp = metav1.Time{}
		for _, b := range inventory.Backups {
			if !b.CreatedAt.IsZero() && (s.ConfigBackup.Status.OldestBackupTimestamp.IsZero() || b.CreatedAt.Before(s.ConfigBackup.Status.OldestBackupTimestamp.Time)) {
				s.ConfigBackup.Status.OldestBackupTimestamp = metav1.NewTime(b.CreatedAt)
			}
		}

		// Update the storage status from the inventory. If no storage information is available, set it to nil.
		s.ConfigBackup.Status.Storage = nil
		if inventory.TotalBytes != nil || inventory.UsedBytes != nil || inventory.FreeBytes != nil {
			s.ConfigBackup.Status.Storage = &v1alpha1.ConfigBackupStorageStatus{}
			s.ConfigBackup.Status.Storage.TotalBytes = inventory.TotalBytes
			s.ConfigBackup.Status.Storage.UsedBytes = inventory.UsedBytes
			s.ConfigBackup.Status.Storage.FreeBytes = inventory.FreeBytes
			s.ConfigBackup.Status.Storage.FreePercent = inventory.FreePercent()
			s.ConfigBackup.Status.Storage.ThresholdBreached = nil
			if s.ConfigBackup.Spec.StorageThreshold != nil {
				breached := inventory.ThresholdBreached(s.ConfigBackup.Spec.StorageThreshold)
				s.ConfigBackup.Status.Storage.ThresholdBreached = &breached
			}
		}
	}()

	if schedule == nil && s.ConfigBackup.Status.LastBackup != nil {
		// One-Shot backup has already been performed, no further action is needed.
		r.Recorder.Eventf(s.ConfigBackup, nil, "Normal", "BackupCompleted", "Reconcile", "One-shot backup already completed at %s", s.ConfigBackup.Status.LastBackup.Timestamp.String())
		return ctrl.Result{}, nil
	}

	var inventory *provider.ConfigBackupInventory
	switch s.ConfigBackup.Spec.Type {
	case v1alpha1.ConfigBackupTypeRemote:
		inventory, err = r.ListRemoteConfigBackups(ctx, store, s)
	default:
		inventory, err = s.Provider.ListConfigBackups(ctx, req)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list backups: %w", err)
	}

	if inventory.ThresholdBreached(s.ConfigBackup.Spec.StorageThreshold) {
		conditions.Set(s.ConfigBackup, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.StorageThresholdExceededReason,
			Message: "Configured storage threshold prevents creating a new backup",
		})
		r.Recorder.Eventf(s.ConfigBackup, nil, "Warning", "StorageThresholdExceeded", "Reconcile", "Configured storage threshold prevents creating a new backup")
		return ctrl.Result{}, nil
	}

	now := metav1.Now()
	s.ConfigBackup.Status.LastAttemptTime = now

	var file *provider.ConfigBackupFile
	switch s.ConfigBackup.Spec.Type {
	case v1alpha1.ConfigBackupTypeRemote:
		file, err = r.CreateRemoteConfigBackup(ctx, store, s)
	default:
		file, err = s.Provider.CreateConfigBackup(ctx, req)
	}
	if err != nil {
		r.Recorder.Eventf(s.ConfigBackup, nil, "Warning", "BackupFailed", "Reconcile", "Failed to create backup: %v", err)
		return ctrl.Result{}, err
	}
	if s.ConfigBackup.Status.LastBackup == nil && s.ConfigBackup.Status.OldestBackupTimestamp.IsZero() {
		s.ConfigBackup.Status.OldestBackupTimestamp = now
	}
	s.ConfigBackup.Status.LastBackup = &v1alpha1.ConfigBackupRunStatus{
		Timestamp:          now,
		Duration:           metav1.Duration{Duration: time.Since(now.Time)},
		ObservedGeneration: s.ConfigBackup.Generation,
	}
	if file != nil {
		s.ConfigBackup.Status.LastBackup.SizeBytes = file.SizeBytes
		s.ConfigBackup.Status.LastBackup.Filepath = file.Path
		if file.SizeBytes != nil {
			configBackupSizeBytes.WithLabelValues(string(s.ConfigBackup.Spec.Type)).Observe(float64(*file.SizeBytes))
		}
	}

	r.Recorder.Eventf(s.ConfigBackup, nil, "Normal", "BackupSuccessful", "Reconcile", "Backup completed successfully")

	// Check if the retention policy is set and if the number of backups exceeds the retention limit.
	// If so, delete the oldest backups to comply with the retention policy.
	// Note: We add 1 to the length of inventory.Backups because we just created a new backup.
	total := int32(len(inventory.Backups) + 1) // #nosec G115
	if s.ConfigBackup.Spec.Retention != nil && total > s.ConfigBackup.Spec.Retention.KeepLast {
		// Sort the backups by creation time in ascending order (oldest first).
		sort.Slice(inventory.Backups, func(i, j int) bool {
			return inventory.Backups[i].CreatedAt.Before(inventory.Backups[j].CreatedAt)
		})
		// Delete the oldest backups that exceed the retention limit.
		backupsToDelete := inventory.Backups[:total-s.ConfigBackup.Spec.Retention.KeepLast]
		switch s.ConfigBackup.Spec.Type {
		case v1alpha1.ConfigBackupTypeRemote:
			err = r.DeleteRemoteConfigBackups(ctx, store, s, backupsToDelete...)
		default:
			err = s.Provider.DeleteConfigBackups(ctx, backupsToDelete...)
		}
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete old backups: %w", err)
		}
	}

	conditions.Set(s.ConfigBackup, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.BackupSuccessfulReason,
		Message: "Backup completed successfully",
	})

	return ctrl.Result{}, nil
}

const (
	// S3AccessKeyID is the Secret key for the access key ID in an S3 credentials Secret.
	S3AccessKeyID = "accessKeyID"
	// S3SecretAccessKey is the Secret key for the secret access key in an S3 credentials Secret.
	S3SecretAccessKey = "secretAccessKey"
)

// objectStorageClient resolves S3 credentials from the referenced Secret and returns an object storage client.
func (r *ConfigBackupReconciler) objectStorageClient(ctx context.Context, s *configBackupScope) (ObjectStorage, error) {
	if r.ObjectStorage != nil {
		return r.ObjectStorage, nil
	}
	ref := s.ConfigBackup.Spec.S3.CredentialsSecretRef
	ns := ref.Namespace
	if ns == "" {
		ns = s.ConfigBackup.Namespace
	}
	var secret corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ns}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.ConfigBackup, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.SecretNotFoundReason,
				Message: fmt.Sprintf("S3 credentials secret %s/%s not found", ns, ref.Name),
			})
			return nil, reconcile.TerminalError(fmt.Errorf("S3 credentials secret %s/%s not found", ns, ref.Name))
		}
		return nil, fmt.Errorf("failed to get S3 credentials secret %s/%s: %w", ns, ref.Name, err)
	}
	accessKeyID, ok := secret.Data[S3AccessKeyID]
	if !ok {
		return nil, reconcile.TerminalError(fmt.Errorf("secret %s/%s missing key %q", ns, ref.Name, S3AccessKeyID))
	}
	secretAccessKey, ok := secret.Data[S3SecretAccessKey]
	if !ok {
		return nil, reconcile.TerminalError(fmt.Errorf("secret %s/%s missing key %q", ns, ref.Name, S3SecretAccessKey))
	}
	return objectstorage.NewClient(objectstorage.Options{
		Endpoint:        s.ConfigBackup.Spec.S3.Endpoint,
		Region:          s.ConfigBackup.Spec.S3.Region,
		AccessKeyID:     string(accessKeyID),
		SecretAccessKey: string(secretAccessKey),
	}), nil
}

// CreateRemoteConfigBackup fetches the running config from the device and uploads it to S3.
func (r *ConfigBackupReconciler) CreateRemoteConfigBackup(ctx context.Context, store ObjectStorage, s *configBackupScope) (*provider.ConfigBackupFile, error) {
	data, err := s.Provider.RunningConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get running config: %w", err)
	}
	now := time.Now().UTC()
	key := fmt.Sprintf(
		"%sconfigbackup-%s-%s-%s",
		s.ConfigBackup.Spec.Path,
		s.ConfigBackup.Namespace,
		s.ConfigBackup.Name,
		now.Format("20060102T150405Z"),
	)
	if err := store.PutObject(ctx, &objectstorage.Object{
		Bucket: s.ConfigBackup.Spec.S3.Bucket,
		Key:    key,
		Body:   data,
	}); err != nil {
		return nil, fmt.Errorf("failed to upload backup to S3: %w", err)
	}
	return &provider.ConfigBackupFile{
		Path:      fmt.Sprintf("s3://%s/%s", s.ConfigBackup.Spec.S3.Bucket, key),
		SizeBytes: new(int64(len(data))),
		CreatedAt: now,
	}, nil
}

// ListRemoteConfigBackups lists backup objects from S3 and returns them as a ConfigBackupInventory.
// Storage fields (TotalBytes, UsedBytes, FreeBytes) are nil since S3 does not expose free-space information.
func (r *ConfigBackupReconciler) ListRemoteConfigBackups(ctx context.Context, store ObjectStorage, s *configBackupScope) (*provider.ConfigBackupInventory, error) {
	objects, err := store.ListObjects(ctx, s.ConfigBackup.Spec.S3.Bucket, s.ConfigBackup.Spec.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote backups: %w", err)
	}
	backups := make([]*provider.ConfigBackupFile, len(objects))
	for i := range objects {
		backups[i] = &provider.ConfigBackupFile{
			Path:      objects[i].Key,
			SizeBytes: &objects[i].Size,
			CreatedAt: objects[i].LastModified,
		}
	}
	return &provider.ConfigBackupInventory{Backups: backups}, nil
}

// DeleteRemoteConfigBackups deletes the specified backup objects from S3.
func (r *ConfigBackupReconciler) DeleteRemoteConfigBackups(ctx context.Context, store ObjectStorage, s *configBackupScope, files ...*provider.ConfigBackupFile) error {
	if len(files) == 0 {
		return nil
	}
	keys := make([]string, len(files))
	for i, f := range files {
		keys[i] = f.Path
	}
	return store.DeleteObjects(ctx, s.ConfigBackup.Spec.S3.Bucket, keys...)
}

func (r *ConfigBackupReconciler) finalize(_ context.Context, _ *configBackupScope) (reterr error) {
	return nil
}

// deviceToConfigBackups is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for ConfigBackups when their referenced Device's effective pause state changes.
func (r *ConfigBackupReconciler) deviceToConfigBackups(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.ConfigBackupList)
	if err := r.List(
		ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list ConfigBackups")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing ConfigBackup for reconciliation", "ConfigBackup", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

// ConfigBackupsForProviderConfig is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a ConfigBackup to update when one of its referenced provider configurations gets updated.
func (r *ConfigBackupReconciler) ConfigBackupsForProviderConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.ConfigBackupList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list ConfigBackups")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Enqueuing ConfigBackup for reconciliation", "ConfigBackup", klog.KObj(&m))
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      m.Name,
					Namespace: m.Namespace,
				},
			})
		}
	}

	return requests
}

// configBackupsForSecret is a [handler.MapFunc] that enqueues reconciliation requests
// for ConfigBackups that reference the given Secret as their S3 credentials source.
func (r *ConfigBackupReconciler) configBackupsForSecret(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Secret", klog.KObj(obj))

	list := &v1alpha1.ConfigBackupList{}
	if err := r.List(ctx, list); err != nil {
		log.Error(err, "Failed to list ConfigBackups")
		return nil
	}

	var requests []reconcile.Request
	for _, m := range list.Items {
		for _, ref := range m.GetSecretRefs() {
			if ref.Name == obj.GetName() && ref.Namespace == obj.GetNamespace() {
				log.V(2).Info("Enqueuing ConfigBackup for reconciliation", "ConfigBackup", klog.KObj(&m))
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      m.Name,
						Namespace: m.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}
