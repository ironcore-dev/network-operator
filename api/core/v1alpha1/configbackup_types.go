// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"path"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ConfigBackupSpec defines the desired state of ConfigBackup.
// +kubebuilder:validation:XValidation:rule="self.type != 'Startup' || (!has(self.path) || size(self.path) == 0)",message="path must be omitted for Startup backups"
// +kubebuilder:validation:XValidation:rule="self.type != 'Local' || (has(self.path) && size(self.path) > 0)",message="path must be set for Local backups"
// +kubebuilder:validation:XValidation:rule="self.type == 'Local' || !has(self.retention)",message="retention must only be specified for Local backups"
// +kubebuilder:validation:XValidation:rule="self.type == 'Local' || !has(self.storageThreshold)",message="storageThreshold must only be specified for Local backups"
type ConfigBackupSpec struct {
	// DeviceRef is a reference to the Device this object belongs to. The Device object must exist in the same namespace.
	// Immutable.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="DeviceRef is immutable"
	DeviceRef LocalObjectReference `json:"deviceRef"`

	// ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
	// This reference is used to link the ConfigBackup to its provider-specific configuration.
	// +optional
	ProviderConfigRef *TypedLocalObjectReference `json:"providerConfigRef,omitempty"`

	// Schedule is an optional cron expression.
	// If omitted, the controller performs a one-shot backup.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// Type determines whether the backup is saved as a local file or as startup-config.
	// +required
	Type ConfigBackupType `json:"type"`

	// Path is the device-local destination path for Local backups.
	// Different providers may accept different path formats, such as "bootflash:///backups/".
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Path string `json:"path,omitempty"`

	// Retention configures automatic cleanup of older backups for Local backups.
	// +optional
	Retention *ConfigBackupRetention `json:"retention,omitempty"`

	// StorageThreshold defines the minimum free space that must remain before creating a new Local backup.
	// +optional
	StorageThreshold *ConfigBackupStorageThreshold `json:"storageThreshold,omitempty"`
}

// ConfigBackupType defines how the device should persist a configuration backup.
// +kubebuilder:validation:Enum=Local;Startup
type ConfigBackupType string

const (
	// ConfigBackupTypeLocal stores the running configuration in a device-local file path.
	ConfigBackupTypeLocal ConfigBackupType = "Local"
	// ConfigBackupTypeStartup stores the running configuration as the device startup configuration.
	ConfigBackupTypeStartup ConfigBackupType = "Startup"
)

// ConfigBackupRetention defines how many historical backups are kept on the device.
type ConfigBackupRetention struct {
	// KeepLast is the number of most recent backups to keep for Local backups.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	KeepLast int32 `json:"keepLast,omitempty"`
}

// ConfigBackupStorageThreshold defines when the controller must stop writing additional backups.
// +kubebuilder:validation:XValidation:rule="has(self.minFreeBytes) || has(self.minFreePercent)",message="at least one threshold must be specified"
type ConfigBackupStorageThreshold struct {
	// MinFreeBytes is the minimum number of free bytes required before a new backup can be written.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinFreeBytes *int64 `json:"minFreeBytes,omitempty"`

	// MinFreePercent is the minimum percentage of free storage required before a new backup can be written.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MinFreePercent *int32 `json:"minFreePercent,omitempty"`
}

// ConfigBackupStatus defines the observed state of ConfigBackup.
type ConfigBackupStatus struct {
	// Conditions represent the current state of the ConfigBackup resource.
	// +listType=map
	// +listMapKey=type
	// +patchStrategy=merge
	// +patchMergeKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastBackup contains details about the most recent successful backup operation.
	// This is updated only when a backup completes successfully, and may be nil if no successful backups have occurred.
	// +optional
	LastBackup *ConfigBackupRunStatus `json:"lastBackup,omitempty"`

	// LastAttemptTime is the timestamp of the most recent backup attempt, regardless of outcome.
	// This is updated whenever the controller attempts to perform a backup, even if it fails.
	// +optional
	LastAttemptTime metav1.Time `json:"lastAttemptTime,omitempty"`

	// OldestBackupTimestamp is the timestamp of the oldest discovered backup on the device.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// +optional
	OldestBackupTimestamp metav1.Time `json:"oldestBackupTimestamp,omitempty"`

	// NextScheduledBackup is the next time at which the controller intends to trigger a backup.
	// This only applies to scheduled backups, and may be unknown if the controller cannot determine the next schedule.
	// +optional
	NextScheduledBackup metav1.Time `json:"nextScheduledBackup,omitempty"`

	// TotalBackups is the number of backups currently discovered on the device.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// For Startup backups, this is always 1, since the device only maintains a single startup configuration.
	// +optional
	TotalBackups *int32 `json:"totalBackups,omitempty"`

	// TotalSizeBytes is the total size in bytes of the discovered backups on the device.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// +optional
	TotalSizeBytes *int64 `json:"totalSizeBytes,omitempty"`

	// Storage contains device-local storage statistics for the configured backup target.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// +optional
	Storage *ConfigBackupStorageStatus `json:"storage,omitempty"`
}

// ConfigBackupRunStatus contains the result of a single successful backup run.
type ConfigBackupRunStatus struct {
	// Timestamp is the time at which the backup was created on the device.
	// +required
	Timestamp metav1.Time `json:"timestamp"`

	// Duration is the duration of the backup operation.
	// +required
	Duration metav1.Duration `json:"duration"`

	// ObservedGeneration represents the .metadata.generation that produced this backup.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// SizeBytes is the size in bytes of the backup artifact.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// +optional
	// +kubebuilder:validation:Minimum=0
	SizeBytes *int64 `json:"sizeBytes,omitempty"`

	// Filepath is the device-local path of the backup artifact.
	// This only applies to Local backups, and may be unknown if the controller cannot query the device.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Filepath string `json:"filepath,omitempty"`
}

// ConfigBackupStorageStatus contains storage utilization for the configured backup target.
type ConfigBackupStorageStatus struct {
	// TotalBytes is the total storage capacity in bytes, if known.
	// +optional
	TotalBytes *int64 `json:"totalBytes,omitempty"`

	// UsedBytes is the used storage in bytes, if known.
	// +optional
	UsedBytes *int64 `json:"usedBytes,omitempty"`

	// FreeBytes is the free storage in bytes, if known.
	// +optional
	FreeBytes *int64 `json:"freeBytes,omitempty"`

	// FreePercent is the free storage percentage, if known.
	// +optional
	FreePercent *int32 `json:"freePercent,omitempty"`

	// ThresholdBreached indicates whether the configured threshold currently blocks new backups.
	// +optional
	ThresholdBreached *bool `json:"thresholdBreached,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=configbackups
// +kubebuilder:resource:singular=configbackup
// +kubebuilder:printcolumn:name="Device",type=string,JSONPath=`.spec.deviceRef.name`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Backup",type=date,JSONPath=`.status.lastBackup.timestamp`,priority=1
// +kubebuilder:printcolumn:name="Next Backup",type=string,JSONPath=`.status.nextScheduledBackup`,priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ConfigBackup is the Schema for the configbackups API.
type ConfigBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the resource.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +required
	Spec ConfigBackupSpec `json:"spec"`

	// Status of the resource. This is set and updated automatically.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Status ConfigBackupStatus `json:"status,omitzero"`
}

// Filename returns a string that can be used as a prefix for backup filenames,
// incorporating the namespace and name of the ConfigBackup resource.
func (c *ConfigBackup) Filename() string {
	return path.Join(c.Spec.Path, fmt.Sprintf("configbackup-%s-%s-", c.Namespace, c.Name))
}

// GetConditions implements conditions.Getter.
func (c *ConfigBackup) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions implements conditions.Setter.
func (c *ConfigBackup) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// ConfigBackupList contains a list of ConfigBackup.
type ConfigBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ConfigBackup `json:"items"`
}

var (
	ConfigBackupListDependencies   []schema.GroupVersionKind
	configBackupListDependenciesMu sync.Mutex
)

func RegisterConfigBackupListDependency(gvk schema.GroupVersionKind) {
	configBackupListDependenciesMu.Lock()
	defer configBackupListDependenciesMu.Unlock()
	ConfigBackupListDependencies = append(ConfigBackupListDependencies, gvk)
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &ConfigBackup{}, &ConfigBackupList{})
		return nil
	})
}
