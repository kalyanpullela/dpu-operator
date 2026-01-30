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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DataProcessingUnitSpec defines the desired state of DataProcessingUnit.
// This v2 spec provides a richer representation of DPU hardware with
// vendor-neutral fields and capability declarations.
type DataProcessingUnitSpec struct {
	// Vendor is the hardware vendor identifier (e.g., "nvidia", "intel", "marvell").
	// +kubebuilder:validation:Required
	Vendor string `json:"vendor"`

	// Model is the specific DPU hardware model (e.g., "BlueField-3", "IPU E2100").
	// +optional
	Model string `json:"model,omitempty"`

	// PCIAddress is the PCI bus address of the DPU (e.g., "0000:03:00.0").
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-9a-fA-F]$`
	// +optional
	PCIAddress string `json:"pciAddress,omitempty"`

	// PCIDeviceID is the PCI vendor:device ID (e.g., "15b3:a2dc").
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{4}:[0-9a-fA-F]{4}$`
	// +optional
	PCIDeviceID string `json:"pciDeviceId,omitempty"`

	// NodeName is the Kubernetes node where this DPU is located.
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// IsDpuSide indicates whether this represents the DPU-side view
	// (running on the DPU's ARM cores) vs host-side view.
	// +kubebuilder:default=false
	IsDpuSide bool `json:"isDpuSide,omitempty"`

	// Capabilities lists the offload capabilities this DPU supports.
	// Populated from plugin discovery.
	// +optional
	Capabilities []DpuCapability `json:"capabilities,omitempty"`

	// DesiredVFCount specifies the desired number of virtual functions.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=256
	// +optional
	DesiredVFCount *int `json:"desiredVfCount,omitempty"`

	// Configuration contains DPU-specific configuration.
	// +optional
	Configuration *DpuConfiguration `json:"configuration,omitempty"`
}

// DpuCapability represents a specific capability of the DPU.
// +kubebuilder:validation:Enum=networking;storage;security;aiml;lifecycle
type DpuCapability string

const (
	// DpuCapabilityNetworking indicates network offload support.
	DpuCapabilityNetworking DpuCapability = "networking"
	// DpuCapabilityStorage indicates storage offload support.
	DpuCapabilityStorage DpuCapability = "storage"
	// DpuCapabilitySecurity indicates security offload support.
	DpuCapabilitySecurity DpuCapability = "security"
	// DpuCapabilityAIML indicates AI/ML inference offload support.
	DpuCapabilityAIML DpuCapability = "aiml"
	// DpuCapabilityLifecycle indicates lifecycle management support.
	DpuCapabilityLifecycle DpuCapability = "lifecycle"
)

// DpuConfiguration contains configuration settings for a DPU.
type DpuConfiguration struct {
	// NetworkMode specifies the networking mode for this DPU.
	// Overrides the cluster-wide setting in DpuOperatorConfig.
	// +kubebuilder:validation:Enum=switchdev;legacy;offload
	// +optional
	NetworkMode *NetworkMode `json:"networkMode,omitempty"`

	// StorageOffloadEnabled enables storage offload for this DPU.
	// +optional
	StorageOffloadEnabled *bool `json:"storageOffloadEnabled,omitempty"`

	// SecurityPolicyEnabled enables security offload for this DPU.
	// +optional
	SecurityPolicyEnabled *bool `json:"securityPolicyEnabled,omitempty"`
}

// DataProcessingUnitStatus defines the observed state of DataProcessingUnit.
type DataProcessingUnitStatus struct {
	// Conditions represent the current state of the DPU.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Phase indicates the current lifecycle phase of the DPU.
	// +kubebuilder:validation:Enum=Discovered;Initializing;Ready;Error;Unknown
	Phase DpuPhase `json:"phase,omitempty"`

	// CurrentVFCount is the current number of virtual functions configured.
	// +optional
	CurrentVFCount *int `json:"currentVfCount,omitempty"`

	// Inventory contains detailed hardware inventory information.
	// +optional
	Inventory *DpuInventory `json:"inventory,omitempty"`

	// Health contains health and monitoring information.
	// +optional
	Health *DpuHealth `json:"health,omitempty"`

	// PluginName is the name of the plugin managing this DPU.
	// +optional
	PluginName string `json:"pluginName,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastUpdated is the timestamp of the last status update.
	// +optional
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// DpuPhase represents the lifecycle phase of a DPU.
// +kubebuilder:validation:Enum=Discovered;Initializing;Ready;Error;Unknown
type DpuPhase string

const (
	// DpuPhaseDiscovered indicates the DPU has been discovered but not initialized.
	DpuPhaseDiscovered DpuPhase = "Discovered"
	// DpuPhaseInitializing indicates the DPU is being initialized.
	DpuPhaseInitializing DpuPhase = "Initializing"
	// DpuPhaseReady indicates the DPU is fully operational.
	DpuPhaseReady DpuPhase = "Ready"
	// DpuPhaseError indicates the DPU is in an error state.
	DpuPhaseError DpuPhase = "Error"
	// DpuPhaseUnknown indicates the DPU state is unknown.
	DpuPhaseUnknown DpuPhase = "Unknown"
)

// DpuInventory contains hardware inventory information.
type DpuInventory struct {
	// SerialNumber is the hardware serial number.
	// +optional
	SerialNumber string `json:"serialNumber,omitempty"`

	// FirmwareVersion is the running firmware version.
	// +optional
	FirmwareVersion string `json:"firmwareVersion,omitempty"`

	// BIOSVersion is the BIOS version.
	// +optional
	BIOSVersion string `json:"biosVersion,omitempty"`

	// CPUModel is the ARM CPU model.
	// +optional
	CPUModel string `json:"cpuModel,omitempty"`

	// CPUCores is the number of CPU cores.
	// +optional
	CPUCores *int `json:"cpuCores,omitempty"`

	// MemoryTotalBytes is the total memory in bytes.
	// +optional
	MemoryTotalBytes *int64 `json:"memoryTotalBytes,omitempty"`

	// NetworkPorts lists available network ports.
	// +optional
	NetworkPorts []NetworkPortInfo `json:"networkPorts,omitempty"`

	// StorageDevices lists attached storage devices.
	// +optional
	StorageDevices []StorageDeviceInfo `json:"storageDevices,omitempty"`
}

// NetworkPortInfo describes a network port on the DPU.
type NetworkPortInfo struct {
	// Name is the port name (e.g., "p0", "p1").
	Name string `json:"name"`

	// MACAddress is the hardware MAC address.
	// +optional
	MACAddress string `json:"macAddress,omitempty"`

	// SpeedMbps is the port speed in Mbps.
	// +optional
	SpeedMbps *int `json:"speedMbps,omitempty"`

	// LinkUp indicates if the link is up.
	// +optional
	LinkUp *bool `json:"linkUp,omitempty"`

	// SupportedModes lists supported modes (e.g., "switchdev", "legacy").
	// +optional
	SupportedModes []string `json:"supportedModes,omitempty"`
}

// StorageDeviceInfo describes a storage device on the DPU.
type StorageDeviceInfo struct {
	// Name is the device name.
	Name string `json:"name"`

	// Type is the device type (e.g., "NVMe", "eMMC").
	Type string `json:"type"`

	// CapacityBytes is the capacity in bytes.
	// +optional
	CapacityBytes *int64 `json:"capacityBytes,omitempty"`

	// Model is the device model.
	// +optional
	Model string `json:"model,omitempty"`
}

// DpuHealth contains health monitoring information.
type DpuHealth struct {
	// Healthy indicates overall health status.
	Healthy bool `json:"healthy"`

	// LastHealthCheck is the timestamp of the last health check.
	// +optional
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// TemperatureCelsius is the current temperature in Celsius.
	// +optional
	TemperatureCelsius *int `json:"temperatureCelsius,omitempty"`

	// PowerWatts is the current power consumption in watts (as a string, e.g., "125.5").
	// +optional
	PowerWatts string `json:"powerWatts,omitempty"`

	// Alerts lists current health alerts.
	// +optional
	Alerts []string `json:"alerts,omitempty"`

	// Message provides additional health information.
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=dpu
//+kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendor"
//+kubebuilder:printcolumn:name="Model",type="string",JSONPath=".spec.model"
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Healthy",type="boolean",JSONPath=".status.health.healthy"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// DataProcessingUnit is the Schema for the dataprocessingunits API v2.
// It represents a discovered DPU device managed by the operator.
type DataProcessingUnit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DataProcessingUnitSpec   `json:"spec,omitempty"`
	Status DataProcessingUnitStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DataProcessingUnitList contains a list of DataProcessingUnit.
type DataProcessingUnitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DataProcessingUnit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DataProcessingUnit{}, &DataProcessingUnitList{})
}
