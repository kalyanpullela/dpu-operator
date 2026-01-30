/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DpuOperatorConfigSpec defines the desired state of DpuOperatorConfig.
// The v2 spec cleanly separates generic (vendor-neutral) settings from
// vendor-specific configuration through an extension point mechanism.
type DpuOperatorConfigSpec struct {
	// Mode specifies whether this configuration is for host nodes or DPU nodes.
	// +kubebuilder:validation:Enum=host;dpu
	// +kubebuilder:default=host
	Mode OperatorMode `json:"mode,omitempty"`

	// LogLevel sets the logging verbosity for the operator and daemons.
	// 0 = minimal logging, higher values increase verbosity.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=0
	LogLevel int `json:"logLevel,omitempty"`

	// Generic contains vendor-neutral configuration settings that apply
	// to all DPU types.
	Generic GenericConfig `json:"generic,omitempty"`

	// VendorConfigs contains configuration for specific vendors.
	// Each entry corresponds to a vendor plugin and provides
	// vendor-specific parameters.
	// +optional
	VendorConfigs []VendorConfigEntry `json:"vendorConfigs,omitempty"`

	// PluginSelector allows filtering which plugins to enable.
	// If empty, all registered plugins are enabled.
	// +optional
	PluginSelector *PluginSelector `json:"pluginSelector,omitempty"`
}

// OperatorMode specifies whether the operator is running on host nodes or DPU nodes.
// +kubebuilder:validation:Enum=host;dpu
type OperatorMode string

const (
	// OperatorModeHost indicates the operator is running on host nodes with DPUs.
	OperatorModeHost OperatorMode = "host"
	// OperatorModeDpu indicates the operator is running on DPU nodes themselves.
	OperatorModeDpu OperatorMode = "dpu"
)

// GenericConfig contains vendor-neutral DPU configuration.
type GenericConfig struct {
	// NetworkMode specifies the overall networking mode for DPUs.
	// +kubebuilder:validation:Enum=switchdev;legacy;offload
	// +kubebuilder:default=switchdev
	NetworkMode NetworkMode `json:"networkMode,omitempty"`

	// StorageOffloadEnabled enables storage offload capabilities.
	// When enabled, the operator will configure NVMe-oF offload if supported.
	// +kubebuilder:default=false
	StorageOffloadEnabled bool `json:"storageOffloadEnabled,omitempty"`

	// SecurityPolicyEnabled enables security offload capabilities.
	// When enabled, the operator will configure IPsec offload if supported.
	// +kubebuilder:default=false
	SecurityPolicyEnabled bool `json:"securityPolicyEnabled,omitempty"`

	// VFCount specifies the default number of virtual functions to create.
	// Can be overridden per-DPU or per-vendor.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=256
	// +optional
	VFCount *int `json:"vfCount,omitempty"`

	// OPIEndpoint specifies the default gRPC endpoint for OPI bridge communication.
	// Individual plugins may override this.
	// +optional
	OPIEndpoint string `json:"opiEndpoint,omitempty"`

	// ResourcePrefix is the prefix used for extended resources.
	// +kubebuilder:default="openshift.io"
	ResourcePrefix string `json:"resourcePrefix,omitempty"`
}

// NetworkMode specifies the DPU networking mode.
// +kubebuilder:validation:Enum=switchdev;legacy;offload
type NetworkMode string

const (
	// NetworkModeSwitchdev uses switchdev mode for eSwitch representors.
	NetworkModeSwitchdev NetworkMode = "switchdev"
	// NetworkModeLegacy uses legacy SR-IOV mode.
	NetworkModeLegacy NetworkMode = "legacy"
	// NetworkModeOffload uses full hardware offload mode.
	NetworkModeOffload NetworkMode = "offload"
)

// VendorConfigEntry provides vendor-specific configuration.
type VendorConfigEntry struct {
	// Vendor identifies which vendor plugin this configuration applies to.
	// Must match a registered plugin name (e.g., "nvidia", "intel", "marvell").
	// +kubebuilder:validation:Required
	Vendor string `json:"vendor"`

	// Enabled indicates whether this vendor should be enabled.
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// ConfigMapRef references a ConfigMap containing vendor-specific configuration.
	// Mutually exclusive with Inline.
	// +optional
	ConfigMapRef *corev1.LocalObjectReference `json:"configMapRef,omitempty"`

	// Inline contains vendor-specific configuration inline.
	// The structure depends on the vendor plugin.
	// Mutually exclusive with ConfigMapRef.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Inline *runtime.RawExtension `json:"inline,omitempty"`
}

// PluginSelector specifies which plugins to enable.
type PluginSelector struct {
	// Names is a list of plugin names to enable.
	// If specified, only these plugins will be active.
	// +optional
	Names []string `json:"names,omitempty"`

	// Vendors is a list of vendor names to enable.
	// If specified, only plugins from these vendors will be active.
	// +optional
	Vendors []string `json:"vendors,omitempty"`

	// Capabilities is a list of required capabilities.
	// Only plugins supporting all listed capabilities will be active.
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

// DpuOperatorConfigStatus defines the observed state of DpuOperatorConfig.
type DpuOperatorConfigStatus struct {
	// Conditions represent the current state of the operator configuration.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ActivePlugins lists the plugins currently active and managing DPUs.
	// +optional
	ActivePlugins []ActivePlugin `json:"activePlugins,omitempty"`

	// DiscoveredDpuCount is the total number of DPUs discovered across all nodes.
	// +optional
	DiscoveredDpuCount int `json:"discoveredDpuCount,omitempty"`

	// ReadyDpuCount is the number of DPUs in ready state.
	// +optional
	ReadyDpuCount int `json:"readyDpuCount,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ActivePlugin describes an active plugin in the system.
type ActivePlugin struct {
	// Name is the plugin name.
	Name string `json:"name"`

	// Vendor is the hardware vendor.
	Vendor string `json:"vendor"`

	// Version is the plugin version.
	Version string `json:"version"`

	// Capabilities lists the plugin's capabilities.
	Capabilities []string `json:"capabilities,omitempty"`

	// ManagedDeviceCount is the number of devices this plugin is managing.
	ManagedDeviceCount int `json:"managedDeviceCount,omitempty"`

	// Healthy indicates if the plugin is healthy.
	Healthy bool `json:"healthy"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Mode",type="string",JSONPath=".spec.mode"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
//+kubebuilder:printcolumn:name="DPUs",type="integer",JSONPath=".status.discoveredDpuCount"
//+kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyDpuCount"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DpuOperatorConfig is the Schema for the dpuoperatorconfigs API v2.
// It provides a unified configuration interface for managing DPUs from
// multiple vendors through a plugin-based architecture.
type DpuOperatorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DpuOperatorConfigSpec   `json:"spec,omitempty"`
	Status DpuOperatorConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DpuOperatorConfigList contains a list of DpuOperatorConfig.
type DpuOperatorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DpuOperatorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DpuOperatorConfig{}, &DpuOperatorConfigList{})
}
