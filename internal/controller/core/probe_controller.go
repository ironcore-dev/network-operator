// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
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
	"github.com/ironcore-dev/network-operator/internal/provider"
	"github.com/ironcore-dev/network-operator/internal/resourcelock"
)

// ProbeReconciler reconciles a Probe object.
type ProbeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	// More info: https://book.kubebuilder.io/reference/raising-events
	Recorder events.EventRecorder

	// Provider is the driver that will be used to execute probe assertions.
	Provider provider.ProviderFunc

	// Locker is used to synchronize operations on resources targeting the same device.
	Locker *resourcelock.ResourceLocker
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=probes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=probes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *ProbeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.Probe)
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

	prov, ok := r.Provider().(provider.ProbeProvider)
	if !ok {
		if meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.ProbeProvider",
		}) {
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	if reachable := conditions.Get(device, v1alpha1.ReachableCondition); reachable != nil && reachable.Status == metav1.ConditionFalse {
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.UnreachableReason,
			Message: "Referenced Device is not reachable",
		})
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	if err := r.Locker.AcquireLock(ctx, device.Name, "probe-controller"); err != nil {
		if errors.Is(err, resourcelock.ErrLockAlreadyHeld) {
			log.V(3).Info("Device is already locked, requeuing reconciliation")
			return ctrl.Result{RequeueAfter: Jitter(time.Second), Priority: new(LockWaitPriorityDefault)}, nil
		}
		log.Error(err, "Failed to acquire device lock")
		return ctrl.Result{}, err
	}
	defer func() {
		if err := r.Locker.ReleaseLock(ctx, device.Name, "probe-controller"); err != nil {
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

	s := &probeScope{
		Device:         device,
		Probe:          obj,
		Connection:     conn,
		ProviderConfig: cfg,
		Provider:       prov,
	}

	if !obj.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
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

func (r *ProbeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.Probe{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.Probe)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Probe{}).
		Named("probe").
		WithEventFilter(filter)

	for _, gvk := range v1alpha1.ProbeListDependencies {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)

		bldr = bldr.Watches(
			obj,
			handler.EnqueueRequestsFromMapFunc(r.probesForProviderConfig),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
	}

	return bldr.
		// Watches enqueues Probes when their referenced Device is created, deleted, or changes reachability.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToProbes),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldDevice := e.ObjectOld.(*v1alpha1.Device)
					newDevice := e.ObjectNew.(*v1alpha1.Device)
					oldReachable := conditions.Get(oldDevice, v1alpha1.ReachableCondition)
					newReachable := conditions.Get(newDevice, v1alpha1.ReachableCondition)
					return oldReachable == nil || newReachable == nil || oldReachable.Status != newReachable.Status
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Probes when a referenced Interface's ready state changes.
		Watches(
			&v1alpha1.Interface{},
			handler.EnqueueRequestsFromMapFunc(r.interfaceToProbes),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldIntf := e.ObjectOld.(*v1alpha1.Interface)
					newIntf := e.ObjectNew.(*v1alpha1.Interface)
					return conditions.IsReady(oldIntf) != conditions.IsReady(newIntf)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Probes when a referenced VLAN's ready state changes.
		Watches(
			&v1alpha1.VLAN{},
			handler.EnqueueRequestsFromMapFunc(r.vlanToProbes),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldVLAN := e.ObjectOld.(*v1alpha1.VLAN)
					newVLAN := e.ObjectNew.(*v1alpha1.VLAN)
					return conditions.IsReady(oldVLAN) != conditions.IsReady(newVLAN)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues Probes when a referenced VRF's ready state changes.
		Watches(
			&v1alpha1.VRF{},
			handler.EnqueueRequestsFromMapFunc(r.vrfToProbes),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldVRF := e.ObjectOld.(*v1alpha1.VRF)
					newVRF := e.ObjectNew.(*v1alpha1.VRF)
					return conditions.IsReady(oldVRF) != conditions.IsReady(newVRF)
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		Complete(r)
}

type probeScope struct {
	Device         *v1alpha1.Device
	Probe          *v1alpha1.Probe
	Connection     *deviceutil.Connection
	ProviderConfig *provider.ProviderConfig
	Provider       provider.ProbeProvider
}

// AssertionError indicates the probe executed successfully but the assertion was not met.
type AssertionError struct {
	Message string
}

func (e *AssertionError) Error() string { return e.Message }

func (r *ProbeReconciler) reconcile(ctx context.Context, s *probeScope) (res ctrl.Result, reterr error) {
	if s.Probe.Labels == nil {
		s.Probe.Labels = make(map[string]string)
	}
	s.Probe.Labels[v1alpha1.DeviceLabel] = s.Device.Name

	// Ensure the Probe is owned by the Device.
	if !controllerutil.HasControllerReference(s.Probe) {
		if err := controllerutil.SetOwnerReference(s.Device, s.Probe, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	var schedule cron.Schedule
	if s.Probe.Spec.Schedule != "" {
		schedule, reterr = cron.ParseStandard(s.Probe.Spec.Schedule)
		if reterr != nil {
			conditions.Set(s.Probe, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ScheduleInvalidReason,
				Message: reterr.Error(),
			})
			return ctrl.Result{}, reconcile.TerminalError(reterr)
		}

		// Determine the last run time. If no probe has been executed yet,
		// use the creation timestamp of the resource.
		last := s.Probe.CreationTimestamp.UTC()
		if s.Probe.Status.LastRunTime != nil {
			last = s.Probe.Status.LastRunTime.UTC()
		}

		// If the next scheduled run is in the future, requeue until that time.
		// Otherwise, continue to execute the probe now.
		if now, next := time.Now().UTC(), schedule.Next(last); next.After(now) {
			s.Probe.Status.NextRunTime = &metav1.Time{Time: next}
			r.Recorder.Eventf(s.Probe, nil, "Normal", "Scheduled", "Reconcile", "Next probe scheduled at %s", next.Format(time.RFC3339))
			return ctrl.Result{RequeueAfter: next.Sub(now)}, nil
		}

		// Update the next scheduled run time after the probe has executed.
		defer func() {
			if reterr != nil {
				return
			}
			next := schedule.Next(time.Now().UTC())
			s.Probe.Status.NextRunTime = &metav1.Time{Time: next}
			r.Recorder.Eventf(s.Probe, nil, "Normal", "Scheduled", "Reconcile", "Next probe scheduled at %s", next.Format(time.RFC3339))
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

	if schedule == nil && s.Probe.Status.LastRunTime != nil {
		// One-shot probe has already been executed, no further action is needed.
		r.Recorder.Eventf(s.Probe, nil, "Normal", "ProbeCompleted", "Reconcile", "One-shot probe already completed at %s", s.Probe.Status.LastRunTime.String())
		return ctrl.Result{}, nil
	}

	message, err := r.executeProbe(ctx, s)

	now := metav1.Now()
	s.Probe.Status.LastRunTime = &now

	// Clear probe-type-specific status from previous runs.
	if s.Probe.Spec.Type != v1alpha1.ProbeTypePing {
		s.Probe.Status.Ping = nil
	}

	if err != nil {
		if assertionErr, ok := errors.AsType[*AssertionError](err); ok {
			// Assertion failed — probe ran but condition was not met.
			conditions.Set(s.Probe, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ProbeFailedReason,
				Message: assertionErr.Message,
			})
			r.Recorder.Eventf(s.Probe, nil, "Warning", "ProbeFailed", "Reconcile", "Probe assertion failed: %s", assertionErr.Message)
			return ctrl.Result{}, nil
		}
		// Execution error — could not run the probe.
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.ProbeErrorReason,
			Message: err.Error(),
		})
		r.Recorder.Eventf(s.Probe, nil, "Warning", "ProbeError", "Reconcile", "Failed to execute probe: %v", err)
		return ctrl.Result{}, err
	}

	conditions.Set(s.Probe, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  v1alpha1.ProbeSuccessfulReason,
		Message: message,
	})
	r.Recorder.Eventf(s.Probe, nil, "Normal", "ProbeSuccessful", "Reconcile", "Probe completed successfully")

	return ctrl.Result{}, nil
}

// executeProbe dispatches to the appropriate provider method based on probe type.
func (r *ProbeReconciler) executeProbe(ctx context.Context, s *probeScope) (string, error) { //nolint:gocyclo
	spec := s.Probe.Spec
	var err error

	switch spec.Type {
	case v1alpha1.ProbeTypePing:
		req := &provider.PingRequest{
			Address:        spec.Ping.Address.String(),
			ProviderConfig: s.ProviderConfig,
		}
		if spec.Ping.SourceInterface != nil {
			req.SourceInterface, err = r.resolveInterfaceName(ctx, s, spec.Ping.SourceInterface)
			if err != nil {
				return "", err
			}
		}
		if spec.Ping.VRF != nil {
			req.VRF, err = r.resolveVRFName(ctx, s, spec.Ping.VRF)
			if err != nil {
				return "", err
			}
		}
		if spec.Ping.Count != nil {
			req.Count = *spec.Ping.Count
		}
		if spec.Ping.PacketSize != nil {
			req.PacketSize = *spec.Ping.PacketSize
		}
		if spec.Ping.Timeout != nil {
			req.Timeout = spec.Ping.Timeout.Duration
		}
		stats, err := s.Provider.Ping(ctx, req)
		if err != nil {
			return "", err
		}

		// Write ping stats into status.
		s.Probe.Status.Ping = &v1alpha1.PingProbeResult{
			Sent:     stats.Sent,
			Received: stats.Received,
		}
		if stats.MinTime > 0 {
			s.Probe.Status.Ping.MinTime = &metav1.Duration{Duration: stats.MinTime}
		}
		if stats.AvgTime > 0 {
			s.Probe.Status.Ping.AvgTime = &metav1.Duration{Duration: stats.AvgTime}
		}
		if stats.MaxTime > 0 {
			s.Probe.Status.Ping.MaxTime = &metav1.Duration{Duration: stats.MaxTime}
		}

		if stats.Received != stats.Sent {
			return "", &AssertionError{Message: fmt.Sprintf("%d/%d packets received", stats.Received, stats.Sent)}
		}

		message := fmt.Sprintf("%d/%d packets received", stats.Received, stats.Sent)
		if stats.AvgTime > 0 {
			message += fmt.Sprintf(", avg %s", stats.AvgTime)
		}

		return message, nil

	case v1alpha1.ProbeTypeMACTableEntry:
		req := &provider.MACTableRequest{
			ProviderConfig: s.ProviderConfig,
		}
		if spec.MACTableEntry.VLAN != nil {
			req.VLAN, err = r.resolveVLANID(ctx, s, spec.MACTableEntry.VLAN)
			if err != nil {
				return "", err
			}
		}
		entries, err := s.Provider.GetMACTable(ctx, req)
		if err != nil {
			return "", err
		}
		targetMAC := spec.MACTableEntry.MACAddress
		if !slices.ContainsFunc(entries, func(entry provider.MACTableEntry) bool {
			return strings.EqualFold(entry.MACAddress, targetMAC)
		}) {
			return "", &AssertionError{Message: fmt.Sprintf("MAC %s not found in MAC table", targetMAC)}
		}
		return fmt.Sprintf("MAC %s found", targetMAC), nil

	case v1alpha1.ProbeTypeRoutePresence:
		req := &provider.RouteTableRequest{
			ProviderConfig: s.ProviderConfig,
		}
		if spec.RoutePresence.VRF != nil {
			req.VRF, err = r.resolveVRFName(ctx, s, spec.RoutePresence.VRF)
			if err != nil {
				return "", err
			}
		}
		routes, err := s.Provider.GetRouteTable(ctx, req)
		if err != nil {
			return "", err
		}
		targetPrefix := spec.RoutePresence.Prefix.String()
		if !slices.ContainsFunc(routes, func(route provider.RouteEntry) bool {
			return route.Prefix == targetPrefix
		}) {
			return "", &AssertionError{Message: fmt.Sprintf("prefix %s not found in routing table", targetPrefix)}
		}
		return fmt.Sprintf("prefix %s found", targetPrefix), nil

	case v1alpha1.ProbeTypeVTEPPeerConnectivity:
		peers, err := s.Provider.GetVTEPPeers(ctx, &provider.VTEPPeersRequest{
			ProviderConfig: s.ProviderConfig,
		})
		if err != nil {
			return "", err
		}
		expected := spec.VTEPPeerConnectivity.ExpectedPeers
		var missing []string
		for _, ep := range expected {
			if !slices.ContainsFunc(peers, func(peer provider.VTEPPeer) bool {
				return peer.PeerIP == ep && peer.OperStatus
			}) {
				missing = append(missing, ep)
			}
		}
		if len(missing) > 0 {
			return "", &AssertionError{Message: fmt.Sprintf("VTEP peers not up: %v", missing)}
		}
		return fmt.Sprintf("%d/%d expected VTEP peers up", len(expected), len(expected)), nil

	default:
		return "", reconcile.TerminalError(fmt.Errorf("unsupported probe type: %s", spec.Type))
	}
}

// resolveInterfaceName resolves an InterfaceSource to the device interface name.
// If the source uses an InterfaceRef, the referenced Interface is fetched and validated
// for existence, same-device ownership, and readiness. On failure, the Ready condition
// is set to False on the Probe and a terminal error is returned.
func (r *ProbeReconciler) resolveInterfaceName(ctx context.Context, s *probeScope, src *v1alpha1.InterfaceSource) (string, error) {
	if src.Name != "" {
		return src.Name, nil
	}
	if src.InterfaceRef == nil {
		return "", nil
	}

	intf := new(v1alpha1.Interface)
	key := client.ObjectKey{Namespace: s.Probe.Namespace, Name: src.InterfaceRef.Name}
	if err := r.Get(ctx, key, intf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Probe, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.InterfaceNotFoundReason,
				Message: fmt.Sprintf("referenced Interface %q not found", src.InterfaceRef.Name),
			})
			return "", reconcile.TerminalError(fmt.Errorf("referenced Interface %q not found", src.InterfaceRef.Name))
		}
		return "", fmt.Errorf("failed to get referenced Interface %q: %w", src.InterfaceRef.Name, err)
	}

	if intf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced Interface %q belongs to device %q, not %q", intf.Name, intf.Spec.DeviceRef.Name, s.Device.Name),
		})
		return "", reconcile.TerminalError(fmt.Errorf("referenced Interface %q belongs to device %q, not %q", intf.Name, intf.Spec.DeviceRef.Name, s.Device.Name))
	}

	if !conditions.IsReady(intf) {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("referenced Interface %q is not yet ready", intf.Name),
		})
		return "", reconcile.TerminalError(fmt.Errorf("referenced Interface %q is not yet ready", intf.Name))
	}

	return intf.Spec.Name, nil
}

// resolveVLANID resolves a VLANSource to the VLAN ID.
// If the source uses a VLANRef, the referenced VLAN is fetched and validated
// for existence, same-device ownership, and readiness. On failure, the Ready condition
// is set to False on the Probe and a terminal error is returned.
func (r *ProbeReconciler) resolveVLANID(ctx context.Context, s *probeScope, src *v1alpha1.VLANSource) (int16, error) {
	if src.ID != nil {
		return *src.ID, nil
	}
	if src.VLANRef == nil {
		return 0, nil
	}

	vlan := new(v1alpha1.VLAN)
	key := client.ObjectKey{Namespace: s.Probe.Namespace, Name: src.VLANRef.Name}
	if err := r.Get(ctx, key, vlan); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Probe, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VLANNotFoundReason,
				Message: fmt.Sprintf("referenced VLAN %q not found", src.VLANRef.Name),
			})
			return 0, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q not found", src.VLANRef.Name))
		}
		return 0, fmt.Errorf("failed to get referenced VLAN %q: %w", src.VLANRef.Name, err)
	}

	if vlan.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VLAN %q belongs to device %q, not %q", vlan.Name, vlan.Spec.DeviceRef.Name, s.Device.Name),
		})
		return 0, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q belongs to device %q, not %q", vlan.Name, vlan.Spec.DeviceRef.Name, s.Device.Name))
	}

	if !conditions.IsReady(vlan) {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("referenced VLAN %q is not yet ready", vlan.Name),
		})
		return 0, reconcile.TerminalError(fmt.Errorf("referenced VLAN %q is not yet ready", vlan.Name))
	}

	return vlan.Spec.ID, nil
}

// resolveVRFName resolves a VRFSource to the device VRF name.
// If the source uses a VRFRef, the referenced VRF is fetched and validated
// for existence, same-device ownership, and readiness. On failure, the Ready condition
// is set to False on the Probe and a terminal error is returned.
func (r *ProbeReconciler) resolveVRFName(ctx context.Context, s *probeScope, src *v1alpha1.VRFSource) (string, error) {
	if src.Name != "" {
		return src.Name, nil
	}
	if src.VRFRef == nil {
		return "", nil
	}

	vrf := new(v1alpha1.VRF)
	key := client.ObjectKey{Namespace: s.Probe.Namespace, Name: src.VRFRef.Name}
	if err := r.Get(ctx, key, vrf); err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(s.Probe, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.VRFNotFoundReason,
				Message: fmt.Sprintf("referenced VRF %q not found", src.VRFRef.Name),
			})
			return "", reconcile.TerminalError(fmt.Errorf("referenced VRF %q not found", src.VRFRef.Name))
		}
		return "", fmt.Errorf("failed to get referenced VRF %q: %w", src.VRFRef.Name, err)
	}

	if vrf.Spec.DeviceRef.Name != s.Device.Name {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.CrossDeviceReferenceReason,
			Message: fmt.Sprintf("referenced VRF %q belongs to device %q, not %q", vrf.Name, vrf.Spec.DeviceRef.Name, s.Device.Name),
		})
		return "", reconcile.TerminalError(fmt.Errorf("referenced VRF %q belongs to device %q, not %q", vrf.Name, vrf.Spec.DeviceRef.Name, s.Device.Name))
	}

	if !conditions.IsReady(vrf) {
		conditions.Set(s.Probe, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.WaitingForDependenciesReason,
			Message: fmt.Sprintf("referenced VRF %q is not yet ready", vrf.Name),
		})
		return "", reconcile.TerminalError(fmt.Errorf("referenced VRF %q is not yet ready", vrf.Name))
	}

	return vrf.Spec.Name, nil
}

// deviceToProbes is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for Probes when their referenced Device's effective pause state changes.
func (r *ProbeReconciler) deviceToProbes(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.ProbeList)
	if err := r.List(
		ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list Probes")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing Probe for reconciliation", "Probe", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

func (r *ProbeReconciler) probesForProviderConfig(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx, "Object", klog.KObj(obj))

	list := &v1alpha1.ProbeList{}
	if err := r.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
		log.Error(err, "Failed to list Probes")
		return nil
	}

	gkv := obj.GetObjectKind().GroupVersionKind()

	var requests []reconcile.Request
	for _, m := range list.Items {
		if m.Spec.ProviderConfigRef != nil &&
			m.Spec.ProviderConfigRef.Name == obj.GetName() &&
			m.Spec.ProviderConfigRef.Kind == gkv.Kind &&
			m.Spec.ProviderConfigRef.APIVersion == gkv.GroupVersion().Identifier() {
			log.V(2).Info("Enqueuing Probe for reconciliation", "Probe", klog.KObj(&m))
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

// interfaceToProbes is a [handler.MapFunc] to enqueue Probes for reconciliation
// when a referenced Interface's ready state changes.
func (r *ProbeReconciler) interfaceToProbes(ctx context.Context, obj client.Object) []ctrl.Request {
	intf, ok := obj.(*v1alpha1.Interface)
	if !ok {
		panic(fmt.Sprintf("expected an Interface but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Interface", klog.KObj(intf))

	list := new(v1alpha1.ProbeList)
	if err := r.List(ctx, list, client.InNamespace(intf.Namespace)); err != nil {
		log.Error(err, "Failed to list Probes")
		return nil
	}

	var requests []ctrl.Request
	for _, p := range list.Items {
		if slices.Contains(p.GetInterfaceReferences(), intf.Name) {
			log.V(2).Info("Enqueuing Probe for reconciliation", "Probe", klog.KObj(&p))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      p.Name,
					Namespace: p.Namespace,
				},
			})
		}
	}

	return requests
}

// vlanToProbes is a [handler.MapFunc] to enqueue Probes for reconciliation
// when a referenced VLAN's ready state changes.
func (r *ProbeReconciler) vlanToProbes(ctx context.Context, obj client.Object) []ctrl.Request {
	vlan, ok := obj.(*v1alpha1.VLAN)
	if !ok {
		panic(fmt.Sprintf("expected a VLAN but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VLAN", klog.KObj(vlan))

	list := new(v1alpha1.ProbeList)
	if err := r.List(ctx, list, client.InNamespace(vlan.Namespace)); err != nil {
		log.Error(err, "Failed to list Probes")
		return nil
	}

	var requests []ctrl.Request
	for _, p := range list.Items {
		if slices.Contains(p.GetVLANReferences(), vlan.Name) {
			log.V(2).Info("Enqueuing Probe for reconciliation", "Probe", klog.KObj(&p))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      p.Name,
					Namespace: p.Namespace,
				},
			})
		}
	}

	return requests
}

// vrfToProbes is a [handler.MapFunc] to enqueue Probes for reconciliation
// when a referenced VRF's ready state changes.
func (r *ProbeReconciler) vrfToProbes(ctx context.Context, obj client.Object) []ctrl.Request {
	vrf, ok := obj.(*v1alpha1.VRF)
	if !ok {
		panic(fmt.Sprintf("expected a VRF but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "VRF", klog.KObj(vrf))

	list := new(v1alpha1.ProbeList)
	if err := r.List(ctx, list, client.InNamespace(vrf.Namespace)); err != nil {
		log.Error(err, "Failed to list Probes")
		return nil
	}

	var requests []ctrl.Request
	for _, p := range list.Items {
		if slices.Contains(p.GetVRFReferences(), vrf.Name) {
			log.V(2).Info("Enqueuing Probe for reconciliation", "Probe", klog.KObj(&p))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      p.Name,
					Namespace: p.Namespace,
				},
			})
		}
	}

	return requests
}
