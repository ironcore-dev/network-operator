// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeviceSpec defines the desired state of Device.
type DeviceSpec struct {
	// Endpoint contains the connection information for the device.
	// +required
	Endpoint Endpoint `json:"endpoint"`

	// Provisioning is an optional configuration for the device provisioning process.
	// It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.
	// +optional
	Provisioning *Provisioning `json:"provisioning,omitempty"`
}

// Endpoint contains the connection information for the device.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.secretRef) || has(self.secretRef)", message="SecretRef is required once set"
type Endpoint struct {
	// Address is the management address of the device provided in IP:Port format.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}:\d{1,5}$`
	// +required
	Address string `json:"address"`

	// SecretRef is name of the authentication secret for the device containing the username and password.
	// The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: 'username' and 'password'.
	// +optional
	SecretRef *SecretReference `json:"secretRef,omitempty"`

	// Transport credentials for grpc connection to the switch.
	// +optional
	TLS *TLS `json:"tls,omitempty"`
}

type TLS struct {
	// The CA certificate to verify the server's identity.
	// +required
	CA SecretKeySelector `json:"ca"`

	// The client certificate and private key to use for mutual TLS authentication.
	// Leave empty if mTLS is not desired.
	// +optional
	Certificate *CertificateSource `json:"certificate,omitempty"`
}

// Provisioning defines the configuration for device bootstrap.
type Provisioning struct {
	// Image defines the image to be used for provisioning the device.
	// +required
	Image Image `json:"image"`

	// BootScript defines the script delivered by a TFTP server to the device during bootstrapping.
	// +optional
	BootScript TemplateSource `json:"bootScript"`
}

// ChecksumType defines the type of checksum used for image verification.
// +kubebuilder:validation:Enum=SHA256;MD5
type ChecksumType string

const (
	ChecksumTypeSHA256 ChecksumType = "SHA256"
	ChecksumTypeMD5    ChecksumType = "MD5" //nolint: usestdlibvars
)

type Image struct {
	// URL is the location of the image to be used for provisioning.
	// +required
	URL string `json:"url"`

	// Checksum is the checksum of the image for verification.
	// +required
	// kubebuilder:validation:MinLength=1
	Checksum string `json:"checksum"`

	// ChecksumType is the type of the checksum (e.g., sha256, md5).
	// +required
	// +kubebuilder:default=MD5
	ChecksumType ChecksumType `json:"checksumType"`
}

// TemplateSource defines a source for template content.
// It can be provided inline, or as a reference to a Secret or ConfigMap.
//
// +kubebuilder:validation:XValidation:rule="[has(self.inline), has(self.secretRef), has(self.configMapRef)].filter(x, x).size() == 1",message="exactly one of 'inline', 'secretRef', or 'configMapRef' must be specified"
type TemplateSource struct {
	// Inline template content
	// +optional
	// +kubebuilder:validation:MinLength=1
	Inline *string `json:"inline,omitempty"`

	// Reference to a Secret containing the template
	// +optional
	SecretRef *SecretKeySelector `json:"secretRef,omitempty"`

	// Reference to a ConfigMap containing the template
	// +optional
	ConfigMapRef *ConfigMapKeySelector `json:"configMapRef,omitempty"`
}

// CertificateSource represents a source for the value of a certificate.
type CertificateSource struct {
	// Secret containing the certificate.
	// The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'.
	// +required
	SecretRef SecretReference `json:"secretRef,omitempty"`
}

// DeviceStatus defines the observed state of Device.
type DeviceStatus struct {
	// Phase represents the current phase of the Device.
	// +kubebuilder:default=Pending
	// +required
	Phase DevicePhase `json:"phase,omitempty"`

	// Manufacturer is the manufacturer of the Device.
	// +optional
	Manufacturer string `json:"manufacturer,omitempty"`

	// Model is the model identifier of the Device.
	// +optional
	Model string `json:"model,omitempty"`

	// SerialNumber is the serial number of the Device.
	// +optional
	SerialNumber string `json:"serialNumber,omitempty"`

	// FirmwareVersion is the firmware version running on the Device.
	// +optional
	FirmwareVersion string `json:"firmwareVersion,omitempty"`

	// Provisioning is the list of provisioning attempts for the Device.
	//+listType=map
	//+listMapKey=startTime
	//+patchStrategy=merge
	//+patchMergeKey=startTime
	//+optional
	Provisioning []ProvisioningInfo `json:"provisioning,omitempty"`

	// Ports is the list of ports on the Device.
	// +optional
	Ports []DevicePort `json:"ports,omitempty"`

	// PostSummary shows a summary of the port configured, grouped by type, e.g. "1/4 (10g), 3/64 (100g)".
	// +optional
	PostSummary string `json:"portSummary,omitempty"`

	// The conditions are a list of status objects that describe the state of the Device.
	//+listType=map
	//+listMapKey=type
	//+patchStrategy=merge
	//+patchMergeKey=type
	//+optional
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ProvisioningInfo struct {
	StartTime metav1.Time       `json:"startTime"`
	Token     string            `json:"token"`
	Phase     ProvisioningPhase `json:"phase"`
	//+optional
	EndTime metav1.Time `json:"endTime,omitzero"`
	//+optional
	RebootTime metav1.Time `json:"reboot,omitzero"`
	//+optional
	Error string `json:"error,omitempty"`
}

// ProvisioningPhase represents the reason for the current provisioning status.
type ProvisioningPhase string

const (
	ProvisioningDataRetrieved                  ProvisioningPhase = "DataRetrieved"
	ProvisioningScriptExecutionStarted         ProvisioningPhase = "ScriptExecutionStarted"
	ProvisioningScriptExecutionFailed          ProvisioningPhase = "ScriptExecutionFailed"
	ProvisioningInstallingCertificates         ProvisioningPhase = "InstallingCertificates"
	ProvisioningDownloadingImage               ProvisioningPhase = "DownloadingImage"
	ProvisioningImageDownloadFailed            ProvisioningPhase = "ImageDownloadFailed"
	ProvisioningUpgradeStarting                ProvisioningPhase = "UpgradeStarting"
	ProvisioningUpgradeFailed                  ProvisioningPhase = "UpgradeFailed"
	ProvisioningRebootingDevice                ProvisioningPhase = "RebootingDevice"
	ProvisioningExecutionFinishedWithoutReboot ProvisioningPhase = "ExecutionFinishedWithoutReboot"
)

func (d *Device) GetActiveProvisioning() *ProvisioningInfo {
	for i := range d.Status.Provisioning {
		if d.Status.Provisioning[i].EndTime.IsZero() {
			return &d.Status.Provisioning[i]
		}
	}
	return nil
}

func (d *Device) CreateProvisioningEntry() (*ProvisioningInfo, error) {
	if d.Status.Phase != DevicePhaseProvisioning {
		return nil, fmt.Errorf("device is in phase %s, expected %s", d.Status.Phase, DevicePhaseProvisioning)
	}
	active := d.GetActiveProvisioning()
	if active != nil {
		return nil, fmt.Errorf("device has an active provisioning with StartTime %s", active.StartTime.String())
	}
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return nil, err
	}
	entry := ProvisioningInfo{
		StartTime: metav1.Now(),
		Token:     hex.EncodeToString(token),
	}
	d.Status.Provisioning = append(d.Status.Provisioning, entry)
	return &entry, nil
}

type DevicePort struct {
	// Name is the name of the port.
	// +required
	Name string `json:"name"`

	// Type is the type of the port, e.g. "10g".
	// +optional
	Type string `json:"type,omitempty"`

	// SupportedSpeedsGbps is the list of supported speeds in Gbps for this port.
	// +optional
	SupportedSpeedsGbps []int32 `json:"supportedSpeedsGbps,omitempty"`

	// Transceiver is the type of transceiver plugged into the port, if any.
	// +optional
	Transceiver string `json:"transceiver,omitempty"`

	// InterfaceRef is the reference to the corresponding Interface resource
	// configuring this port, if any.
	// +optional
	InterfaceRef *LocalObjectReference `json:"interfaceName,omitzero"`
}

// DevicePhase represents the current phase of the Device as it's being provisioned and managed by the operator.
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Failed;Provisioned
type DevicePhase string

const (
	// DevicePhasePending indicates that the device is pending and has not yet been provisioned.
	DevicePhasePending DevicePhase = "Pending"
	// DevicePhaseProvisioning indicates that the device is being provisioned.
	DevicePhaseProvisioning DevicePhase = "Provisioning"
	// DevicePhaseProvisioned indicates that the device provisioning has completed and the operator is performing post-provisioning tasks.
	DevicePhaseProvisioned DevicePhase = "Provisioned"
	// DevicePhaseRunning indicates that the device has been successfully provisioned and is now ready for use.
	DevicePhaseRunning DevicePhase = "Running"
	// DevicePhaseFailed indicates that the device provisioning has failed.
	DevicePhaseFailed DevicePhase = "Failed"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=devices
// +kubebuilder:resource:singular=device
// +kubebuilder:resource:shortName=dev
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint.address`
// +kubebuilder:printcolumn:name="Manufacturer",type=string,JSONPath=".status.manufacturer",priority=1
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=".status.model",priority=1
// +kubebuilder:printcolumn:name="SerialNumber",type=string,JSONPath=".status.serialNumber",priority=1
// +kubebuilder:printcolumn:name="FirmwareVersion",type=string,JSONPath=".status.firmwareVersion",priority=1
// +kubebuilder:printcolumn:name="Ports",type=string,JSONPath=".status.portSummary",priority=1
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Device is the Schema for the devices API.
type Device struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec DeviceSpec `json:"spec,omitempty"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status DeviceStatus `json:"status,omitempty"`
}

// GetConditions implements conditions.Getter.
func (d *Device) GetConditions() []metav1.Condition {
	return d.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (d *Device) SetConditions(conditions []metav1.Condition) {
	d.Status.Conditions = conditions
}

// GetSecretRefs returns the list of secrets referenced in the [Device] resource.
func (d *Device) GetSecretRefs() []SecretReference {
	refs := []SecretReference{}
	if d.Spec.Endpoint.SecretRef != nil {
		refs = append(refs, *d.Spec.Endpoint.SecretRef)
	}
	if d.Spec.Endpoint.TLS != nil {
		refs = append(refs, d.Spec.Endpoint.TLS.CA.SecretReference)
		if d.Spec.Endpoint.TLS.Certificate != nil {
			refs = append(refs, d.Spec.Endpoint.TLS.Certificate.SecretRef)
		}
	}
	for i := range refs {
		if refs[i].Namespace == "" {
			refs[i].Namespace = d.Namespace
		}
	}
	return refs
}

// GetConfigMapRefs returns the list of configmaps referenced in the [Device] resource.
func (d *Device) GetConfigMapRefs() []ConfigMapReference {
	refs := []ConfigMapReference{}
	for i := range refs {
		if refs[i].Namespace == "" {
			refs[i].Namespace = d.Namespace
		}
	}
	return refs
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device.
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
