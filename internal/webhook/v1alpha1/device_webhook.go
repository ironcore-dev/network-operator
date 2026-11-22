// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	api "github.com/ironcore-dev/network-operator/api/core/v1alpha1"
)

// SetupDeviceWebhookWithManager registers the Device validator.
func SetupDeviceWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &api.Device{}).
		WithValidator(&DeviceCustomValidator{}).Complete()
}

// +kubebuilder:webhook:path=/validate-networking-metal-ironcore-dev-v1alpha1-device,mutating=false,failurePolicy=Fail,sideEffects=None,groups=networking.metal.ironcore.dev,resources=devices,verbs=create;update,versions=v1alpha1,name=device-v1alpha1.kb.io,admissionReviewVersions=v1

// DeviceCustomValidator restricts Secret and ConfigMap references to the object's namespace.
type DeviceCustomValidator struct{}

func (v *DeviceCustomValidator) ValidateCreate(_ context.Context, obj *api.Device) (admission.Warnings, error) {
	return nil, validateReferenceNamespaces(obj)
}

func (v *DeviceCustomValidator) ValidateUpdate(ctx context.Context, _ *api.Device, obj *api.Device) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, obj)
}

func (v *DeviceCustomValidator) ValidateDelete(_ context.Context, _ *api.Device) (admission.Warnings, error) {
	return nil, nil
}

func validateReferenceNamespaces(obj client.Object) error {
	var errs field.ErrorList
	check := func(namespace string, path *field.Path) {
		if namespace != "" && namespace != obj.GetNamespace() {
			errs = append(errs, field.Forbidden(path.Child("namespace"), "must match the referencing object's namespace"))
		}
	}
	checkTemplate := func(src *api.TemplateSource, path *field.Path) {
		if src.SecretRef != nil {
			check(src.SecretRef.Namespace, path.Child("secretRef"))
		}
		if src.ConfigMapRef != nil {
			check(src.ConfigMapRef.Namespace, path.Child("configMapRef"))
		}
	}
	spec := field.NewPath("spec")
	var kind string
	switch obj := obj.(type) {
	case *api.Device:
		kind = "Device"
		endpoint := spec.Child("endpoint")
		if obj.Spec.Endpoint.SecretRef != nil {
			check(obj.Spec.Endpoint.SecretRef.Namespace, endpoint.Child("secretRef"))
		}
		if tls := obj.Spec.Endpoint.TLS; tls != nil {
			check(tls.CA.Namespace, endpoint.Child("tls", "ca"))
			if tls.Certificate != nil {
				check(tls.Certificate.SecretRef.Namespace, endpoint.Child("tls", "certificate", "secretRef"))
			}
		}
		if obj.Spec.Provisioning != nil {
			checkTemplate(&obj.Spec.Provisioning.BootScript, spec.Child("provisioning", "bootScript"))
		}
	case *api.Certificate:
		kind = "Certificate"
		check(obj.Spec.SecretRef.Namespace, spec.Child("secretRef"))
	case *api.ConfigBackup:
		kind = "ConfigBackup"
		if s3 := obj.Spec.S3; s3 != nil {
			check(s3.CredentialsSecretRef.Namespace, spec.Child("s3", "credentialsSecretRef"))
			if s3.Encryption != nil {
				check(s3.Encryption.KeySecret.Namespace, spec.Child("s3", "encryption", "keySecret"))
			}
		}
	case *api.ConsoleConnection:
		kind = "ConsoleConnection"
		check(obj.Spec.Endpoint.SecretRef.Namespace, spec.Child("endpoint", "secretRef"))
	case *api.User:
		kind = "User"
		check(obj.Spec.Password.SecretKeyRef.Namespace, spec.Child("password", "secretKeyRef"))
		if obj.Spec.SSHPublicKey != nil {
			check(obj.Spec.SSHPublicKey.SecretKeyRef.Namespace, spec.Child("sshPublicKey", "secretKeyRef"))
		}
	case *api.AAA:
		kind = "AAA"
		for i := range obj.Spec.ServerGroups {
			group := &obj.Spec.ServerGroups[i]
			for j := range group.Servers {
				server := &group.Servers[j]
				path := spec.Child("serverGroups").Index(i).Child("servers").Index(j)
				if server.TACACS != nil {
					check(server.TACACS.KeySecretRef.Namespace, path.Child("tacacs", "keySecretRef"))
				}
				if server.RADIUS != nil {
					check(server.RADIUS.KeySecretRef.Namespace, path.Child("radius", "keySecretRef"))
				}
			}
		}
	case *api.Banner:
		kind = "Banner"
		checkTemplate(&obj.Spec.Message, spec.Child("message"))
	default:
		return fmt.Errorf("unsupported reference validation type %T", obj)
	}
	if len(errs) != 0 {
		return apierrors.NewInvalid(api.GroupVersion.WithKind(kind).GroupKind(), obj.GetName(), errs)
	}
	return nil
}
