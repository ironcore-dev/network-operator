// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

type Object interface {
	client.Object
	metav1.ObjectMetaAccessor
	conditions.Setter

	GetDeviceRef() v1alpha1.LocalObjectReference
	GetProviderConfigRef() *v1alpha1.TypedLocalObjectReference
	GetStatus() any
}

// Reconciler is a specialized version of Reconciler that acts on instances of [Object].
// Depending on whether the object is being created/updated or deleted, either Reconcile
// or Finalize will be called.
type Reconciler[O Object, P provider.Provider] interface {
	Reconcile(context.Context, *TypedScope[O, P]) error
	Finalize(context.Context, *TypedScope[O, P]) error
}

type reconciler[O Object, P provider.Provider] struct {
	client.Client

	// Provider is the driver that will be used to create & delete the accesscontrollist.
	Provider provider.ProviderFunc

	// Reconciler is the actual reconciler that will be called by this generic reconciler.
	Reconciler Reconciler[O, P]
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details about the method shape, read up here:
// - https://ahmet.im/blog/controller-pitfalls/#reconcile-method-shape
func (r *reconciler[O, P]) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling resource")

	obj := reflect.New(reflect.TypeOf(*new(O)).Elem()).Interface().(O)
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

	prov, ok := r.Provider().(P)
	if !ok {
		cond := obj.GetConditions()
		if meta.SetStatusCondition(&cond, metav1.Condition{
			Type:    v1alpha1.ReadyCondition,
			Status:  metav1.ConditionFalse,
			Reason:  v1alpha1.NotImplementedReason,
			Message: "Provider does not implement provider.Provider",
		}) {
			obj.SetConditions(cond)
			return ctrl.Result{}, r.Status().Update(ctx, obj)
		}
		return ctrl.Result{}, nil
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
			s, err := r.GetScope(ctx, obj, prov)
			if err != nil {
				log.Error(err, "Failed to get scope for resource")
				return ctrl.Result{}, err
			}
			if err := r.Reconciler.Finalize(ctx, s); err != nil {
				log.Error(err, "Failed to finalize resource")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(obj, v1alpha1.FinalizerName)
			if err := r.Update(ctx, obj); err != nil {
				log.Error(err, "Failed to remove finalizer from resource")
				return ctrl.Result{}, err
			}
		}
		log.Info("Resource is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(obj, v1alpha1.FinalizerName) {
		controllerutil.AddFinalizer(obj, v1alpha1.FinalizerName)
		if err := r.Update(ctx, obj); err != nil {
			log.Error(err, "Failed to add finalizer to resource")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to resource")
		return ctrl.Result{}, nil
	}

	orig := obj.DeepCopyObject().(O)
	if conditions.InitializeConditions(obj, v1alpha1.ReadyCondition) {
		log.Info("Initializing status conditions")
		return ctrl.Result{}, r.Status().Update(ctx, obj)
	}

	// Always attempt to update the metadata/status after reconciliation
	defer func() {
		if !equality.Semantic.DeepEqual(orig.GetObjectMeta(), obj.GetObjectMeta()) {
			if err := r.Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update resource metadata")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
			return
		}

		if !equality.Semantic.DeepEqual(orig.GetStatus(), obj.GetStatus()) {
			if err := r.Status().Patch(ctx, obj, client.MergeFrom(orig)); err != nil {
				log.Error(err, "Failed to update status")
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}
	}()

	s, err := r.GetScope(ctx, obj, prov)
	if err != nil {
		log.Error(err, "Failed to get scope for resource")
		return ctrl.Result{}, err
	}

	if err := r.Reconciler.Reconcile(ctx, s); err != nil {
		log.Error(err, "Failed to reconcile resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *reconciler[O, P]) GetScope(ctx context.Context, obj O, prov P) (*TypedScope[O, P], error) {
	device, err := deviceutil.GetDeviceByName(ctx, r, obj.GetNamespace(), obj.GetDeviceRef().Name)
	if err != nil {
		return nil, err
	}

	conn, err := deviceutil.GetDeviceConnection(ctx, r, device)
	if err != nil {
		return nil, err
	}

	var cfg *provider.ProviderConfig
	if ref := obj.GetProviderConfigRef(); ref != nil {
		cfg, err = provider.GetProviderConfig(ctx, r, obj.GetNamespace(), ref)
		if err != nil {
			return nil, err
		}
	}

	return &TypedScope[O, P]{
		Device:         device,
		Connection:     conn,
		Resource:       obj,
		Provider:       prov,
		ProviderConfig: cfg,
	}, nil
}

// AsReconciler creates a [reconcile.Reconciler] based on the given [Reconciler].
func AsReconciler[T Object, P provider.Provider](c client.Client, p provider.ProviderFunc, rec Reconciler[T, P]) reconcile.Reconciler {
	return &reconciler[T, P]{
		Client:     c,
		Provider:   p,
		Reconciler: rec,
	}
}

// TypedScope holds the different objects that are read and used during the reconcile.
type TypedScope[T client.Object, P provider.Provider] struct {
	Device         *v1alpha1.Device
	Connection     *deviceutil.Connection
	Resource       T
	Provider       P
	ProviderConfig *provider.ProviderConfig
}
