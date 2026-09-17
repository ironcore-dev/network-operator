// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"golang.org/x/crypto/ssh"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/apistatus"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

const DefaultConsoleTimeout = 30 * time.Second

// ConsoleConnectionReconciler reconciles a ConsoleConnection object.
type ConsoleConnectionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string

	// Recorder is used to record events for the controller.
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=consoleconnections,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=consoleconnections/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.metal.ironcore.dev,resources=consoleconnections/finalizers,verbs=update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

func (r *ConsoleConnectionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Reconciling resource")

	obj := new(v1alpha1.ConsoleConnection)
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

	device, err := deviceutil.GetDeviceByName(ctx, r, obj.Namespace, obj.Spec.DeviceRef.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	res, err := r.reconcile(ctx, obj, device)
	if err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, apistatus.WrapTerminalError(err)
	}

	return res, nil
}

func (r *ConsoleConnectionReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	labelSelector := metav1.LabelSelector{}
	if r.WatchFilterValue != "" {
		labelSelector.MatchLabels = map[string]string{v1alpha1.WatchLabel: r.WatchFilterValue}
	}

	filter, err := predicate.LabelSelectorPredicate(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to create label selector predicate: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &v1alpha1.ConsoleConnection{}, v1alpha1.DeviceRefIndexKey, func(obj client.Object) []string {
		o := obj.(*v1alpha1.ConsoleConnection)
		return []string{o.Spec.DeviceRef.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ConsoleConnection{}).
		Named("consoleconnection").
		WithEventFilter(filter).
		// Watches enqueues Probes when their referenced Device is created, deleted or updated.
		Watches(
			&v1alpha1.Device{},
			handler.EnqueueRequestsFromMapFunc(r.deviceToConsoleConnections),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldDevice := e.ObjectOld.(*v1alpha1.Device)
					newDevice := e.ObjectNew.(*v1alpha1.Device)
					return oldDevice.Status.Hostname != newDevice.Status.Hostname || oldDevice.Status.SerialNumber != newDevice.Status.SerialNumber
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			}),
		).
		// Watches enqueues ConsoleConnection for referenced Secret resources.
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToConsoleConnections),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *ConsoleConnectionReconciler) reconcile(ctx context.Context, obj *v1alpha1.ConsoleConnection, device *v1alpha1.Device) (res ctrl.Result, reterr error) {
	if obj.Labels == nil {
		obj.Labels = make(map[string]string)
	}
	obj.Labels[v1alpha1.DeviceLabel] = device.Name

	if !controllerutil.HasControllerReference(obj) {
		if err := controllerutil.SetOwnerReference(device, obj, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
			return ctrl.Result{}, err
		}
	}

	var schedule cron.Schedule
	if obj.Spec.Schedule != "" {
		var err error
		schedule, err = cron.ParseStandard(obj.Spec.Schedule)
		if err != nil {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.ScheduleInvalidReason,
				Message: err.Error(),
			})
			return ctrl.Result{}, reconcile.TerminalError(err)
		}

		// Determine the last check time. If no checks have been performed yet,
		// use the creation timestamp of the resource.
		last := obj.CreationTimestamp.UTC()
		if obj.Status.LastCheckTime != nil {
			last = obj.Status.LastCheckTime.UTC()
		}

		// If the next scheduled check is in the future, requeue until that time.
		// Otherwise, continue to check now.
		if now, next := time.Now().UTC(), schedule.Next(last); next.After(now) {
			obj.Status.NextCheckTime = &metav1.Time{Time: next}
			r.Recorder.Eventf(obj, nil, "Normal", "Scheduled", "Reconcile", "Next console check scheduled at %s", next.Format(time.RFC3339))
			return ctrl.Result{RequeueAfter: next.Sub(now)}, nil
		}

		defer func() {
			if reterr != nil {
				return
			}
			next := schedule.Next(time.Now().UTC())
			obj.Status.NextCheckTime = &metav1.Time{Time: next}
			r.Recorder.Eventf(obj, nil, "Normal", "Scheduled", "Reconcile", "Next console check scheduled at %s", next.Format(time.RFC3339))
			res.RequeueAfter = time.Until(next)
		}()
	}

	if schedule == nil && obj.Status.LastCheckTime != nil {
		r.Recorder.Eventf(obj, nil, "Normal", "CheckCompleted", "Reconcile", "One-shot check already completed at %s", obj.Status.LastCheckTime.String())
		return ctrl.Result{}, nil
	}

	c := clientutil.NewClient(r, obj.Namespace)
	user, pass, err := c.BasicAuth(ctx, &obj.Spec.Endpoint.SecretRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			conditions.Set(obj, metav1.Condition{
				Type:    v1alpha1.ReadyCondition,
				Status:  metav1.ConditionFalse,
				Reason:  v1alpha1.SecretNotFoundReason,
				Message: fmt.Sprintf("Secret %q not found", obj.Spec.Endpoint.SecretRef.Name),
			})
			return ctrl.Result{}, reconcile.TerminalError(err)
		}
		return ctrl.Result{}, err
	}

	timeout := obj.Spec.Timeout.Duration
	if timeout == 0 {
		timeout = DefaultConsoleTimeout
	}

	match, err := r.buildMatcher(obj, device)
	if err != nil {
		conditions.Set(obj, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotReadyReason,
			Message: err.Error(),
		})
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	reason, message := r.check(ctx, obj, string(user), string(pass), timeout, match)
	now := metav1.Now()
	obj.Status.LastCheckTime = &now

	status := metav1.ConditionFalse
	if reason == v1alpha1.ConsoleVerifiedReason {
		status = metav1.ConditionTrue
	}
	conditions.Set(obj, metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  status,
		Reason:  reason,
		Message: message,
	})

	eventType := "Warning"
	if status == metav1.ConditionTrue {
		eventType = "Normal"
	}
	r.Recorder.Eventf(obj, nil, eventType, reason, "Reconcile", message)

	return ctrl.Result{}, nil
}

// buildMatcher constructs a function that checks whether the console output matches the expected string or regex.
// If no explicit expectation is set, it defaults to matching the device's hostname or serial number.
func (r *ConsoleConnectionReconciler) buildMatcher(obj *v1alpha1.ConsoleConnection, device *v1alpha1.Device) (func(string) bool, error) {
	if obj.Spec.Verification.Expect != nil {
		if obj.Spec.Verification.Expect.String != nil {
			s := *obj.Spec.Verification.Expect.String
			return func(output string) bool { return strings.Contains(output, s) }, nil
		}
		if obj.Spec.Verification.Expect.Regex != nil {
			re, err := regexp.Compile(*obj.Spec.Verification.Expect.Regex)
			if err != nil {
				return nil, fmt.Errorf("invalid expect regex: %w", err)
			}
			return re.MatchString, nil
		}
	}
	// Default: match device hostname or serial number.
	hostname, serial := device.Status.Hostname, device.Status.SerialNumber
	if hostname == "" && serial == "" {
		return nil, errors.New("device has no hostname or serial number in status; set spec.verification.expect explicitly")
	}
	return func(output string) bool {
		return (hostname != "" && strings.Contains(output, hostname)) || (serial != "" && strings.Contains(output, serial))
	}, nil
}

func (r *ConsoleConnectionReconciler) check(ctx context.Context, obj *v1alpha1.ConsoleConnection, user, pass string, timeout time.Duration, match func(string) bool) (reason, message string) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // console servers rarely have known host keys
		Timeout:         timeout,
	}

	conn, err := ssh.Dial("tcp", obj.Spec.Endpoint.Address, config)
	if err != nil {
		if isAuthError(err) {
			return v1alpha1.ConsoleServerAuthFailureReason, fmt.Sprintf("Authentication failed: %v", err)
		}
		return v1alpha1.ConsoleServerUnreachableReason, fmt.Sprintf("Could not reach console server: %v", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return v1alpha1.ConsoleServerUnreachableReason, fmt.Sprintf("Could not open SSH session: %v", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return v1alpha1.ConsoleServerUnreachableReason, fmt.Sprintf("Could not attach to session output: %v", err)
	}

	if err := session.Shell(); err != nil {
		return v1alpha1.ConsoleServerUnreachableReason, fmt.Sprintf("Could not start shell: %v", err)
	}

	stdin, err := session.StdinPipe()
	if err == nil {
		switch obj.Spec.Verification.Strategy {
		case v1alpha1.ConsoleVerificationSendCRLF:
			_, _ = stdin.Write([]byte("\r\n")) //nolint:errcheck // best-effort stimulus on serial line
		case v1alpha1.ConsoleVerificationSendChar:
			if obj.Spec.Verification.Char != nil {
				_, _ = stdin.Write([]byte(*obj.Spec.Verification.Char)) //nolint:errcheck // best-effort stimulus on serial line
			}
		case v1alpha1.ConsoleVerificationWait:
			// Do nothing.
		}
	}

	// Read output until timeout or match.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	buf := make([]byte, 4096)
	var output strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
				if match(output.String()) {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	select {
	case <-done:
		// Reader finished — either matched or stream ended.
	case <-ctx.Done():
		// Timeout reached.
	}

	received := output.String()
	if received == "" {
		return v1alpha1.ConsoleDeadReason, "No output received on serial connection"
	}
	if match(received) {
		return v1alpha1.ConsoleVerifiedReason, "Console connection verified"
	}
	return v1alpha1.ConsoleAliveReason, "Received output but expected string not matched"
}

func isAuthError(err error) bool {
	// Network-level errors (dial timeout, connection refused) are not auth failures.
	if _, ok := errors.AsType[*net.OpError](err); ok { //nolint:errcheck // second return is the typed error, unused
		return false
	}
	// ssh.Dial returns a plain error for auth failures; if we got past
	// the network layer, treat it as an auth failure.
	return true
}

// deviceToConsoleConnections is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for ConsoleConnections when their referenced Device's gets created or deleted.
func (r *ConsoleConnectionReconciler) deviceToConsoleConnections(ctx context.Context, obj client.Object) []ctrl.Request {
	device, ok := obj.(*v1alpha1.Device)
	if !ok {
		panic(fmt.Sprintf("expected a Device but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Device", klog.KObj(device))

	list := new(v1alpha1.ConsoleConnectionList)
	if err := r.List(
		ctx, list,
		client.InNamespace(device.Namespace),
		client.MatchingFields{v1alpha1.DeviceRefIndexKey: device.Name},
	); err != nil {
		log.Error(err, "Failed to list ConsoleConnections")
		return nil
	}

	requests := make([]ctrl.Request, 0, len(list.Items))
	for _, i := range list.Items {
		log.V(2).Info("Enqueuing ConsoleConnection for reconciliation", "ConsoleConnection", klog.KObj(&i))
		requests = append(requests, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      i.Name,
				Namespace: i.Namespace,
			},
		})
	}

	return requests
}

// secretToConsoleConnections is a [handler.MapFunc] to be used to enqueue requests for reconciliation
// for a ConsoleConnection to update when one of its referenced Secrets gets updated.
func (r *ConsoleConnectionReconciler) secretToConsoleConnections(ctx context.Context, obj client.Object) []ctrl.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		panic(fmt.Sprintf("expected a Secret but got a %T", obj))
	}

	log := ctrl.LoggerFrom(ctx, "Secret", klog.KObj(secret))

	list := new(v1alpha1.ConsoleConnectionList)
	if err := r.List(ctx, list, client.InNamespace(secret.Namespace)); err != nil {
		log.Error(err, "Failed to list ConsoleConnections")
		return nil
	}

	var requests []ctrl.Request
	for _, c := range list.Items {
		if c.Spec.Endpoint.SecretRef.Name == secret.Name && c.Namespace == secret.Namespace {
			log.V(2).Info("Enqueuing ConsoleConnection for reconciliation", "ConsoleConnection", klog.KObj(&c))
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Name:      c.Name,
					Namespace: c.Namespace,
				},
			})
		}
	}

	return requests
}
