// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/paused"
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
	Recorder events.EventRecorder

	// Provider is the driver that will be used to create & delete the interface.
	Provider provider.ProviderFunc

	// HeartbeatInterval is the duration after which the controller requeues the reconciliation,
	// regardless of changes.
	HeartbeatInterval time.Duration
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=devices/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
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
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) { //nolint:gocyclo
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.Device)
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

	if isPaused, requeue, err := paused.EnsureCondition(ctx, r.Client, obj, obj); isPaused || requeue || err != nil {
		return ctrl.Result{Requeue: requeue}, err
	}

	conn, err := deviceutil.GetDeviceConnection(ctx, r, obj)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to obtain device connection: %w", err)
	}

	orig := obj.DeepCopy()
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition, v1alpha1.ReachableCondition) {
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
			r.Recorder.Eventf(obj, nil, "Warning", "Unsupported", "Reconcile", "Provider does not support provisioning")
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
		r.Recorder.Eventf(obj, nil, "Normal", "ProvisioningStarted", "Reconcile", "Device provisioning has started")
		return ctrl.Result{}, nil

	case v1alpha1.DevicePhaseProvisioning:
		if obj.Spec.Provisioning == nil {
			log.Info("Provisioning configuration was removed, resetting device into pending phase")
			if activeProv := obj.GetActiveProvisioning(); activeProv != nil {
				activeProv.EndTime = metav1.Now()
			}
			obj.Status.Phase = v1alpha1.DevicePhasePending
			r.Recorder.Eventf(obj, nil, "Warning", "ProvisioningAborted", "Reconcile", "Provisioning configuration was removed, resetting device into pending phase")
			return ctrl.Result{}, nil
		}
		activeProv := obj.GetActiveProvisioning()
		if activeProv == nil {
			log.Info("Device has not made a provisioning request yet")
			return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
		}
		if activeProv.StartTime.Add(time.Hour).Before(time.Now()) {
			activeProv.EndTime = metav1.Now()
			activeProv.Error = "provisioning timed out"
			obj.Status.Phase = v1alpha1.DevicePhaseFailed
			r.Recorder.Eventf(obj, nil, "Warning", "ProvisioningFailed", "Reconcile", "Device provisioning has timed out")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 20 * time.Minute}, nil

	case v1alpha1.DevicePhaseProvisioned:
		// Finalize the provisioning, either wait for the reboot to be initiated/completed
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
		if activeProv.StartTime.Add(time.Hour).Before(time.Now()) {
			activeProv.EndTime = metav1.Now()
			activeProv.Error = "post-provisioning checks timed out"
			obj.Status.Phase = v1alpha1.DevicePhaseFailed
			r.Recorder.Eventf(obj, nil, "Warning", "ProvisioningFailed", "Reconcile", "Device post-provisioning checks have timed out")
			return ctrl.Result{}, nil
		}
		log.Info("Device provisioning completed, running post provisioning checks")
		prov, _ := r.Provider().(provider.ProvisioningProvider)
		if ok := prov.VerifyProvisioned(ctx, conn, obj); !ok {
			return ctrl.Result{RequeueAfter: r.HeartbeatInterval}, nil
		}
		activeProv.EndTime = metav1.Now()
		r.Recorder.Eventf(obj, nil, "Normal", "Provisioned", "Reconcile", "Device provisioning has completed successfully")
		obj.Status.Phase = v1alpha1.DevicePhaseRunning
		return ctrl.Result{}, nil

	case v1alpha1.DevicePhaseRunning:
		if prov, ok := r.Provider().(provider.DeviceProvider); ok {
			if err := r.reconcile(ctx, obj, prov, conn); err != nil {
				log.Error(err, "Failed to reconcile resource")
				return ctrl.Result{}, err
			}
		} else {
			if err := r.reconcileMinimal(ctx, obj, conn); err != nil {
				return ctrl.Result{}, err
			}
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

	if err := r.reconcileMaintenance(ctx, obj, conn); err != nil {
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	return ctrl.Result{RequeueAfter: r.HeartbeatInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.HeartbeatInterval == 0 {
		return errors.New("heartbeat interval must not be 0")
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
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// Watches enqueues Devices when an Interface is created or deleted,
		// since Interface.Spec.Name is immutable and only create/delete events
		// can change the device's port summary.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToDevices),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		Complete(r)
}

func (r *DeviceReconciler) reconcile(ctx context.Context, device *v1alpha1.Device, prov provider.DeviceProvider, conn *deviceutil.Connection) (reterr error) {
	if err := prov.Connect(ctx, conn); err != nil {
		conditions.Set(device, metav1.Condition{
			Type:    v1alpha1.ReachableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.UnreachableReason,
			Message: fmt.Sprintf("Failed to connect to device: %v", err),
		})
		conditions.Set(device, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.UnreachableReason,
			Message: "Device is not reachable",
		})
		return nil
	}
	defer func() {
		if err := prov.Disconnect(ctx, conn); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conditions.Set(device, metav1.Condition{
		Type:    v1alpha1.ReachableCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReachableReason,
		Message: "Device is reachable",
	})

	// Reboot-gated queries: only fetch hardware info and ports when the device
	// has rebooted since the last observed reboot time, or on first connection.
	lastReboot, err := prov.GetLastRebootTime(ctx)
	if err != nil {
		return fmt.Errorf("failed to get last reboot time: %w", err)
	}

	if device.Status.LastRebootTime.IsZero() || lastReboot.After(device.Status.LastRebootTime.Time) {
		info, err := prov.GetDeviceInfo(ctx)
		if err != nil {
			return fmt.Errorf("failed to get device info: %w", err)
		}
		device.Status.Hostname = info.Hostname
		device.Status.Manufacturer = info.Manufacturer
		device.Status.Model = info.Model
		device.Status.SerialNumber = info.SerialNumber
		device.Status.FirmwareVersion = info.FirmwareVersion
		device.Status.LastRebootTime = metav1.NewTime(lastReboot)

		ports, err := prov.ListPorts(ctx)
		if err != nil {
			return fmt.Errorf("failed to list device ports: %w", err)
		}
		device.Status.Ports = make([]v1alpha1.DevicePort, len(ports))
		for i, p := range ports {
			device.Status.Ports[i] = v1alpha1.DevicePort{
				Name:                p.ID,
				Type:                p.Type,
				SupportedSpeedsGbps: p.SupportedSpeedsGbps,
				Transceiver:         p.Transceiver,
			}
			slices.Sort(device.Status.Ports[i].SupportedSpeedsGbps)
		}

		log := ctrl.LoggerFrom(ctx)
		if device.Labels == nil {
			device.Labels = map[string]string{}
		}
		if serial := strings.ToLower(device.Status.SerialNumber); serial != "" {
			serial = sanitizeLabelValue(serial)
			if device.Labels[v1alpha1.DeviceSerialLabel] == "" {
				device.Labels[v1alpha1.DeviceSerialLabel] = serial
			} else if !strings.EqualFold(device.Labels[v1alpha1.DeviceSerialLabel], serial) {
				log.Info("Device serial label does not match observed device serial number", "labelSerial", device.Labels[v1alpha1.DeviceSerialLabel], "observedSerial", serial)
			}
		}
	}

	// Always rebuild InterfaceRef mappings from the local Interface list.
	interfaces := new(v1alpha1.InterfaceList)
	if err := r.List(ctx, interfaces, client.InNamespace(device.Namespace), client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name}); err != nil {
		return fmt.Errorf("failed to list interface resources for device: %w", err)
	}

	m := make(map[string]string) // port ID => Interface resource name
	for _, intf := range interfaces.Items {
		m[intf.Spec.Name] = intf.Name
	}

	for i := range device.Status.Ports {
		portName := device.Status.Ports[i].Name
		var newRef *v1alpha1.LocalObjectReference
		if name, ok := m[portName]; ok {
			newRef = &v1alpha1.LocalObjectReference{Name: name}
		}
		device.Status.Ports[i].InterfaceRef = newRef
	}

	device.Status.PortSummary = PortSummary(device.Status.Ports)

	conditions.Set(device, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Device is healthy",
	})

	return nil
}

func (r *DeviceReconciler) reconcileMinimal(ctx context.Context, device *v1alpha1.Device, conn *deviceutil.Connection) (reterr error) {
	prov := r.Provider()
	if err := prov.Connect(ctx, conn); err != nil {
		conditions.Set(device, metav1.Condition{
			Type:    v1alpha1.ReachableCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.UnreachableReason,
			Message: fmt.Sprintf("Failed to connect to device: %v", err),
		})
		conditions.Set(device, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionUnknown,
			Reason:  v1alpha1.UnreachableReason,
			Message: "Device is not reachable",
		})
		return nil
	}
	defer func() {
		if err := prov.Disconnect(ctx, conn); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	conditions.Set(device, metav1.Condition{
		Type:    v1alpha1.ReachableCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReachableReason,
		Message: "Device is reachable",
	})
	conditions.Set(device, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ReadyReason,
		Message: "Device is healthy",
	})

	return nil
}

func (r *DeviceReconciler) reconcileMaintenance(ctx context.Context, obj *v1alpha1.Device, conn *deviceutil.Connection) error {
	action, ok := obj.Annotations[v1alpha1.DeviceMaintenanceAnnotation]
	if !ok {
		return nil
	}
	switch action {
	case v1alpha1.DeviceMaintenanceReboot:
		prov, ok := r.Provider().(provider.MaintenanceProvider)
		if !ok {
			r.Recorder.Eventf(obj, nil, "Warning", "MaintenanceUnsupported", "Maintenance", "Provider does not support maintenance operation: %s", action)
			return nil
		}
		// Reboot triggers a device restart. The device remains in its current phase
		// and will resume normal operation after the reboot completes.
		r.Recorder.Eventf(obj, nil, "Normal", "RebootRequested", "Maintenance", "Device reboot has been requested")
		if err := prov.Reboot(ctx, conn); err != nil {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.MaintenanceFailedReason,
				Message: fmt.Sprintf("Failed to reboot device: %v", err),
			})
			r.Recorder.Eventf(obj, nil, "Warning", "RebootFailed", "Maintenance", "Device reboot has failed: %v", err)
			return fmt.Errorf("failed to reboot device: %w", err)
		}

	case v1alpha1.DeviceMaintenanceFactoryReset:
		prov, ok := r.Provider().(provider.MaintenanceProvider)
		if !ok {
			r.Recorder.Eventf(obj, nil, "Warning", "MaintenanceUnsupported", "Maintenance", "Provider does not support maintenance operation: %s", action)
			return nil
		}
		// FactoryReset erases all device configuration and returns it to its original state.
		// After completion, the device phase is reset to Pending to restart the lifecycle.
		r.Recorder.Eventf(obj, nil, "Normal", "FactoryResetRequested", "Maintenance", "Device factory reset has been requested")
		if err := prov.FactoryReset(ctx, conn); err != nil {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.MaintenanceFailedReason,
				Message: fmt.Sprintf("Failed to factory reset device: %v", err),
			})
			r.Recorder.Eventf(obj, nil, "Warning", "FactoryResetFailed", "Maintenance", "Device factory reset has failed: %v", err)
			return fmt.Errorf("failed to reset device to factory defaults: %w", err)
		}
		obj.Status.Phase = v1alpha1.DevicePhasePending

	case v1alpha1.DeviceMaintenanceReprovision:
		prov, ok := r.Provider().(provider.ProvisioningProvider)
		if !ok {
			r.Recorder.Eventf(obj, nil, "Warning", "MaintenanceUnsupported", "Maintenance", "Provider does not support provisioning operation: %s", action)
			return nil
		}
		// Reprovision prepares the device for re-provisioning without a full factory reset.
		// The provider initiates the provisioning process, then the phase is reset to Pending.
		r.Recorder.Eventf(obj, nil, "Normal", "ReprovisionRequested", "Maintenance", "Device reprovisioning has been requested")
		if err := prov.Reprovision(ctx, conn); err != nil {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.MaintenanceFailedReason,
				Message: fmt.Sprintf("Failed to prepare device for reprovisioning: %v", err),
			})
			r.Recorder.Eventf(obj, nil, "Warning", "ReprovisionFailed", "Maintenance", "Device reprovisioning preparation has failed: %v", err)
			return fmt.Errorf("failed to prepare device for reprovisioning: %w", err)
		}
		obj.Status.Phase = v1alpha1.DevicePhasePending

	case v1alpha1.DeviceMaintenanceResetPhase:
		// Reset phase is a soft reset that only changes the device phase to Pending without
		// performing any device-side operations. This is useful for recovering from terminal
		// states (e.g., Failed) after manual intervention.
		r.Recorder.Eventf(obj, nil, "Normal", "PhaseReset", "Maintenance", "Device phase has been reset to Pending")
		obj.Status.Phase = v1alpha1.DevicePhasePending

	default:
		r.Recorder.Eventf(obj, nil, "Warning", "UnknownMaintenanceAction", "Maintenance", "Unknown maintenance action: %s", action)
		return reconcile.TerminalError(fmt.Errorf("unknown maintenance action: %s", action))
	}

	// Only remove the annotation after the operation succeeds so that
	// failed actions are retried on the next reconciliation.
	delete(obj.Annotations, v1alpha1.DeviceMaintenanceAnnotation)
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
			log.V(2).Info("Enqueuing Device for reconciliation", "Device", klog.KObj(&dev))
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

	log.V(2).Info("Enqueuing Device for reconciliation")
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
		fmt.Fprintf(&sb, "%d/%d (%s)", u.Used, u.Total, u.Type)
	}

	return sb.String()
}

// Jitter returns a randomized duration within +/- 10% of the given duration.
func Jitter(d time.Duration) time.Duration {
	r := rand.Float64() // #nosec G404
	return time.Duration(float64(d) * (0.9 + 0.2*r))
}

var invalidLabelChars = regexp.MustCompile(`[^A-Za-z0-9_.\-]`)

// sanitizeLabelValue ensures the serial number is a valid Kubernetes label [1]
// value (max 63 chars, alphanumeric/hyphen/underscore/dot, no leading or
// trailing separators). The provider returns the raw device serial which may
// contain characters not allowed in labels.
//
// [1]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
func sanitizeLabelValue(s string) string {
	s = invalidLabelChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_.")
	return s[:min(len(s), 63)]
}
