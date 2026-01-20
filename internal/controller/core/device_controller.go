// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

// DeviceReconciler reconciles a Device object
type DeviceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder record.EventRecorder

	// Provider is the driver that will be used to create & delete the interface.
	Provider provider.ProviderFunc

	// RequeueInterval is the duration after which the controller should requeue the reconciliation,
	// regardless of changes.
	RequeueInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;update;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := new(v1alpha1.Device)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("Resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get resource")
		return ctrl.Result{}, err
	}

	prov, ok := r.Provider().(provider.DeviceProvider)
	if !ok {
		err := errors.New("provider does not implement DeviceProvider interface")
		log.Error(err, "failed to reconcile resource")
		return ctrl.Result{}, err
	}
	conn, err := deviceutil.GetDeviceConnection(ctx, r, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to obtain device connection: %w", err)
	}

	orig := obj.DeepCopy()

	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.ObjectMeta, obj.ObjectMeta) {
			// pass obj.DeepCopy() to avoid Patch() modifying obj and interfering with status update below
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

	switch obj.Status.Phase {
	case v1alpha1.DevicePhasePending:
		if obj.Spec.Provisioning == nil {
			// Skip provisioning if no provisioning configuration is provided.
			obj.Status.Phase = v1alpha1.DevicePhaseRunning
			return ctrl.Result{}, nil
		}

		if _, ok := r.Provider().(provider.ProvisioningProvider); !ok {
			// Skip provisioning if the provider does not support it.
			log.Info("Provider does not support provisioning, skipping")
			obj.Status.Phase = v1alpha1.DevicePhaseFailed
			r.Recorder.Event(obj, "Warning", "Unsupported", "Provider does not support provisioning")
			return ctrl.Result{}, reconcile.TerminalError(errors.New("provider does not support provisioning"))
		}

		log.Info("Device is in pending phase, starting provisioning")
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ProvisioningReason,
			Message: "Device is being provisioned",
		})
		obj.Status.Phase = v1alpha1.DevicePhaseProvisioning
		r.Recorder.Event(obj, "Normal", "ProvisioningStarted", "Device provisioning has started")
		return ctrl.Result{}, nil

	case v1alpha1.DevicePhaseProvisioning:
		annotations := obj.GetAnnotations()
		delete(annotations, v1alpha1.DeviceMaintenanceAnnotation)
		obj.SetAnnotations(annotations)
		activeProv := obj.GetActiveProvisioning()
		if activeProv == nil {
			log.Info("Device has not made a provisioning request yet")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		if activeProv.StartTime.Add(time.Hour).After(time.Now()) {
			obj.Status.Phase = v1alpha1.DevicePhaseFailed
			r.Recorder.Event(obj, "Warning", "ProvisioningFailed", "Device provisioning has timed out")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 20 * time.Minute}, nil

	case v1alpha1.DevicePhaseProvisioned:
		// we will finalize the provisioning here, either wait for the reboot to be initiated/completed
		// or run post-provisioning checks instantly if no reboot is required
		activeProv := obj.GetActiveProvisioning()
		if activeProv == nil {
			err := errors.New("device went into provisioning completed phase, but no active provisioning found")
			log.Error(err, "Failed to finalize provisioning")
			return ctrl.Result{}, reconcile.TerminalError(err)
		}
		if !activeProv.RebootTime.IsZero() && activeProv.RebootTime.Time.Add(time.Minute).After(time.Now()) {
			log.Info("Device is rebooting, requeuing")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		log.Info("Device provisioning completed, running post provisioning checks")
		prov, _ := r.Provider().(provider.ProvisioningProvider)
		if ok := prov.VerifyProvisioned(ctx, conn, obj); !ok {
			return ctrl.Result{RequeueAfter: r.RequeueInterval}, nil
		}
		activeProv.EndTime = metav1.Now()
		r.Recorder.Event(obj, "Normal", "Provisioned", "Device provisioning has completed successfully")
		obj.Status.Phase = v1alpha1.DevicePhaseRunning

	case v1alpha1.DevicePhaseRunning:
		if err := r.reconcile(ctx, obj, prov, conn); err != nil {
			log.Error(err, "Failed to reconcile resource")
			return ctrl.Result{}, err
		}

	case v1alpha1.DevicePhaseFailed:
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotReadyReason,
			Message: "Device provisioning has failed",
		})

	default:
		log.Info("Device is in an unknown phase, resetting to active")
		obj.Status.Phase = v1alpha1.DevicePhaseRunning
	}

	if err := r.reconcileMaintenance(ctx, obj, prov, conn); err != nil {
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.RequeueInterval == 0 {
		return errors.New("requeue interval must not be 0")
	}

	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Device{}).
		Named("device").
		WithEventFilter(filter).
		// Watches enqueues Devices for referenced Secret resources.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToDevices),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		// Watches enqueues Devices for referenced ConfigMap resources.
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.configMapToDevices),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		// Watches enqueues Devices for contained Interface resources.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToDevices),
			builder.WithPredicates(predicate.Or(predicate.ResourceVersionChangedPredicate{}, predicate.LabelChangedPredicate{})),
		).
		Complete(r)
}

func (r *DeviceReconciler) reconcile(ctx context.Context, device *v1alpha1.Device, prov provider.DeviceProvider, conn *deviceutil.Connection) (reterr error) {
	if err := prov.Connect(ctx, conn); err != nil {
		conditions.Set(device, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.UnreachableReason,
			Message: fmt.Sprintf("Failed to connect to provider: %v", err),
		})
		return fmt.Errorf("failed to connect to provider: %w", err)
	}
	defer func() {
		if err := prov.Disconnect(ctx, conn); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	ports, err := prov.ListPorts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list device ports: %w", err)
	}

	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(device.Namespace), client.MatchingLabels{v1alpha1.DeviceLabel: device.Name}); err != nil {
		return fmt.Errorf("failed to list interface resources for device: %w", err)
	}

	m := make(map[string]string) // ID => Resource Name
	for _, intf := range interfaces.Items {
		m[intf.Spec.Name] = intf.Name
	}

	device.Status.Ports = make([]v1alpha1.DevicePort, len(ports))
	n := int32(0)
	for i, p := range ports {
		var ref *v1alpha1.LocalObjectReference
		if name, ok := m[p.ID]; ok {
			ref = &v1alpha1.LocalObjectReference{Name: name}
			n++
		}
		device.Status.Ports[i] = v1alpha1.DevicePort{
			Name:                p.ID,
			Type:                p.Type,
			SupportedSpeedsGbps: p.SupportedSpeedsGbps,
			Transceiver:         p.Transceiver,
			InterfaceRef:        ref,
		}
		slices.Sort(device.Status.Ports[i].SupportedSpeedsGbps)
	}

	device.Status.PostSummary = PortSummary(device.Status.Ports)

	info, err := prov.GetDeviceInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device details: %w", err)
	}

	device.Status.Manufacturer = info.Manufacturer
	device.Status.Model = info.Model
	device.Status.SerialNumber = info.SerialNumber
	device.Status.FirmwareVersion = info.FirmwareVersion

	conditions.Set(device, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Device is healthy",
	})

	return nil
}

func (r *DeviceReconciler) reconcileMaintenance(ctx context.Context, obj *v1alpha1.Device, prov provider.DeviceProvider, conn *deviceutil.Connection) error {
	action, ok := obj.Annotations[v1alpha1.DeviceMaintenanceAnnotation]
	if !ok {
		return nil
	}
	delete(obj.Annotations, v1alpha1.DeviceMaintenanceAnnotation)
	switch action {
	case v1alpha1.DeviceMaintenanceReboot:
		r.Recorder.Event(obj, "Normal", "RebootRequested", "Device reboot has been requested")
		if err := prov.Reboot(ctx, conn); err != nil {
			return fmt.Errorf("failed to reboot device: %w", err)
		}

	case v1alpha1.DeviceMaintenanceFactoryReset:
		r.Recorder.Event(obj, "Normal", "FactoryResetRequested", "Device factory reset has been requested")
		if err := prov.FactoryReset(ctx, conn); err != nil {
			return fmt.Errorf("failed to reset device to factory defaults: %w", err)
		}

	case v1alpha1.DeviceMaintenanceReprovision:
		r.Recorder.Event(obj, "Normal", "ReprovisioningRequested", "Device reprovisioning has been requested. Preparing the device.")
		if err := prov.Reprovision(ctx, conn); err != nil {
			return fmt.Errorf("failed to reset device to factory defaults: %w", err)
		}
		obj.Status.Phase = v1alpha1.DevicePhasePending
	case v1alpha1.DeviceMaintenanceResetPhaseToProvisioning:
		r.Recorder.Event(obj, "Normal", "ResetPhaseToProvisioningRequested", "Device phase reset to Pending has been requested.")
		obj.Status.Phase = v1alpha1.DevicePhasePending
	default:
		return fmt.Errorf("unknown device action: %s", action)
	}
	return nil
}

// secretToDevices is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Device to update when one of its referenced Secrets gets updated.
func (r *DeviceReconciler) secretToDevices(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("Expected a Secret but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Secret", klog.KObj(secret))

	devices := new(v1alpha1.DeviceList)
	if err := r.List(ctx, devices); err != nil {
		log.Error(err, "Failed to list Devices")
		return nil
	}

	requests := []ctrl.Request{}
	for _, dev := range devices.Items {
		if slices.ContainsFunc(dev.GetSecretRefs(), func(ref v1alpha1.SecretReference) bool {
			return ref.Name == secret.Name && ref.Namespace == secret.Namespace
		}) {
			log.Info("Enqueuing Device for reconciliation", "Device", klog.KObj(&dev))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      dev.Name,
					Namespace: dev.Namespace,
				},
			})
		}
	}

	return requests
}

// configMapToDevices is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Device to update when one of its referenced ConfigMaps gets updated.
func (r *DeviceReconciler) configMapToDevices(ctx context.Context, obj client.Object) []ctrl.Request {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		panic(fmt.Sprintf("Expected a ConfigMap but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "ConfigMap", klog.KObj(cm))

	devices := new(v1alpha1.DeviceList)
	if err := r.List(ctx, devices); err != nil {
		log.Error(err, "Failed to list Devices")
		return nil
	}

	requests := []ctrl.Request{}
	for _, dev := range devices.Items {
		if slices.ContainsFunc(dev.GetConfigMapRefs(), func(ref v1alpha1.ConfigMapReference) bool {
			return ref.Name == cm.Name && ref.Namespace == cm.Namespace
		}) {
			log.Info("Enqueuing Device for reconciliation", "Device", klog.KObj(&dev))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      dev.Name,
					Namespace: dev.Namespace,
				},
			})
		}
	}

	return requests
}

// interfaceToDevices is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a Device to update when one of its contained Interfaces gets updated.
func (r *DeviceReconciler) interfaceToDevices(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("Expected a Interface but got a %T", obj))
	}

	if intf.GetLabels()[v1alpha1.DeviceLabel] != intf.Spec.DeviceRef.Name {
		// If the device label is not set (yet), we skip the event.
		return nil
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(intf), "Device", klog.KRef(intf.Namespace, intf.Spec.DeviceRef.Name))

	dev := new(v1alpha1.Device)
	if err := r.Get(ctx, client.ObjectKey{Namespace: intf.Namespace, Name: intf.Spec.DeviceRef.Name}, dev); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Referenced Device not found, skipping")
			return nil
		}
		log.Error(err, "Failed to get referenced Device")
		return nil
	}

	log.Info("Enqueuing Device for reconciliation")
	return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(dev)}}
}

// PortSummary returns a summary string of the given ports in the format: "used/total (type), used/total (type), ..."
func PortSummary(ports []v1alpha1.DevicePort) string {
	type usage struct {
		Type  string
		Used  int
		Total int
	}

	var g []*usage
	for _, p := range ports {
		// As we assume a relatively small number of port types, we use a simple
		// linear search instead of a map for grouping/lookup.
		u := &usage{Type: p.Type}
		idx := slices.IndexFunc(g, func(u *usage) bool { return u.Type == p.Type })
		if idx >= 0 {
			u = g[idx]
		}

		u.Total++
		if p.InterfaceRef != nil {
			u.Used++
		}

		if idx < 0 {
			g = append(g, u)
		}
	}

	slices.SortFunc(g, func(a, b *usage) int {
		return cmp.Compare(a.Type, b.Type)
	})

	var sb strings.Builder
	for i, u := range g {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d/%d (%s)", u.Used, u.Total, u.Type))
	}

	return sb.String()
}

// Jitter returns a randomized duration within +/- 10% of the given duration.
func Jitter(d time.Duration) time.Duration {
	r := rand.Float64() // #nosec G404
	return time.Duration(float64(d) * (0.9 + 0.2*r))
}
