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

// Package plugin provides the plugin architecture for the unified DPU operator.
// This package defines the core interfaces and types that all vendor plugins must implement.
package plugin

// Capability represents a specific offload capability that a plugin can provide.
type Capability string

// DefaultOPIEndpoint is the default gRPC endpoint for OPI bridge communication.
const DefaultOPIEndpoint = "localhost:50051"

const (
	// CapabilityNetworking indicates the plugin supports network offload operations.
	CapabilityNetworking Capability = "networking"
	// CapabilityStorage indicates the plugin supports storage offload operations.
	CapabilityStorage Capability = "storage"
	// CapabilitySecurity indicates the plugin supports security offload operations (e.g., IPsec).
	CapabilitySecurity Capability = "security"
	// CapabilityAIML indicates the plugin supports AI/ML inference offload.
	CapabilityAIML Capability = "aiml"
)

// PluginInfo contains metadata about a vendor plugin.
type PluginInfo struct {
	// Name is the unique identifier for this plugin (e.g., "nvidia", "intel", "marvell").
	Name string

	// Vendor is the hardware vendor name (e.g., "NVIDIA", "Intel", "Marvell").
	Vendor string

	// Version is the semantic version of this plugin implementation.
	Version string

	// Description provides a human-readable description of the plugin.
	Description string

	// SupportedDevices is a list of PCI vendor:device IDs this plugin can handle.
	// Format: "vendorID:deviceID" (e.g., "15b3:a2d6" for NVIDIA BlueField-2).
	SupportedDevices []PCIDeviceID

	// Capabilities lists the offload capabilities this plugin provides.
	Capabilities []Capability
}

// PCIDeviceID represents a PCI device identifier.
type PCIDeviceID struct {
	// VendorID is the PCI vendor ID in hexadecimal (e.g., "15b3" for Mellanox/NVIDIA).
	VendorID string

	// DeviceID is the PCI device ID in hexadecimal (e.g., "a2d6" for BlueField-2).
	DeviceID string

	// Description provides a human-readable name for this device.
	Description string
}

// String returns the PCI ID in standard format "vendorID:deviceID".
func (p PCIDeviceID) String() string {
	return p.VendorID + ":" + p.DeviceID
}

// Device represents a discovered DPU device.
type Device struct {
	// ID is a unique identifier for this device instance.
	ID string

	// PCIAddress is the PCI bus address (e.g., "0000:03:00.0").
	PCIAddress string

	// PCIID is the PCI vendor:device ID.
	PCIID PCIDeviceID

	// Vendor is the hardware vendor name.
	Vendor string

	// Model is the device model name.
	Model string

	// SerialNumber is the device serial number, if available.
	SerialNumber string

	// FirmwareVersion is the currently running firmware version.
	FirmwareVersion string

	// Healthy indicates if the device is in a healthy state.
	Healthy bool

	// Metadata contains additional vendor-specific metadata.
	Metadata map[string]string
}

// PluginConfig provides configuration for plugin initialization.
type PluginConfig struct {
	// OPIEndpoint is the gRPC endpoint for OPI bridge communication.
	OPIEndpoint string

	// NetworkEndpoint overrides the gRPC endpoint for EVPN-GW network operations.
	// If empty, the OPIEndpoint is used.
	NetworkEndpoint string

	// LogLevel sets the logging verbosity for the plugin.
	LogLevel int

	// VendorConfig contains vendor-specific configuration as key-value pairs.
	VendorConfig map[string]interface{}
}

// InventoryResponse contains the OPI-format inventory for a device.
type InventoryResponse struct {
	// DeviceID is the identifier of the inventoried device.
	DeviceID string

	// BIOSVersion is the BIOS/UEFI version.
	BIOSVersion string

	// BMCVersion is the BMC firmware version, if applicable.
	BMCVersion string

	// Chassis contains chassis information.
	Chassis *ChassisInfo

	// CPU contains CPU information.
	CPU *CPUInfo

	// Memory contains memory information.
	Memory *MemoryInfo

	// NetworkInterfaces lists available network interfaces.
	NetworkInterfaces []NetworkInterface

	// StorageDevices lists available storage devices.
	StorageDevices []StorageDevice
}

// ChassisInfo contains chassis-level information.
type ChassisInfo struct {
	// Manufacturer is the chassis manufacturer.
	Manufacturer string
	// Model is the chassis model.
	Model string
	// SerialNumber is the chassis serial number.
	SerialNumber string
}

// CPUInfo contains CPU information.
type CPUInfo struct {
	// Model is the CPU model name.
	Model string
	// CoreCount is the number of CPU cores.
	CoreCount int
	// ThreadCount is the number of CPU threads.
	ThreadCount int
	// FrequencyMHz is the CPU frequency in MHz.
	FrequencyMHz int
}

// MemoryInfo contains memory information.
type MemoryInfo struct {
	// TotalBytes is the total memory in bytes.
	TotalBytes uint64
	// Type is the memory type (e.g., "DDR4", "DDR5").
	Type string
}

// NetworkInterface represents a network interface on the DPU.
type NetworkInterface struct {
	// Name is the interface name (e.g., "eth0").
	Name string
	// MACAddress is the hardware MAC address.
	MACAddress string
	// SpeedMbps is the interface speed in Mbps.
	SpeedMbps int
	// LinkUp indicates if the link is up.
	LinkUp bool
}

// StorageDevice represents a storage device on the DPU.
type StorageDevice struct {
	// Name is the device name.
	Name string
	// Model is the device model.
	Model string
	// CapacityBytes is the capacity in bytes.
	CapacityBytes uint64
	// Type is the device type (e.g., "NVMe", "SSD").
	Type string
}
