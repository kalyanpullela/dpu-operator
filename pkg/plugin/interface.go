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

package plugin

import (
	"context"
)

// Plugin is the core interface that all vendor plugins must implement.
// This interface defines the minimal contract for a DPU plugin to be
// registered and used by the operator.
type Plugin interface {
	// Info returns metadata about this plugin including name, vendor,
	// supported devices, and capabilities.
	Info() PluginInfo

	// Initialize sets up the plugin with the provided configuration.
	// This is called once when the plugin is first loaded.
	Initialize(ctx context.Context, config PluginConfig) error

	// Shutdown gracefully stops the plugin and releases any resources.
	Shutdown(ctx context.Context) error

	// HealthCheck verifies the plugin is operational.
	// Returns nil if healthy, error otherwise.
	HealthCheck(ctx context.Context) error

	// DiscoverDevices scans for DPU hardware that this plugin can manage.
	// Returns a list of discovered devices.
	DiscoverDevices(ctx context.Context) ([]Device, error)

	// GetInventory retrieves detailed inventory information for a specific device.
	// The deviceID should match a device ID returned by DiscoverDevices.
	GetInventory(ctx context.Context, deviceID string) (*InventoryResponse, error)
}

// NetworkPlugin extends Plugin with network offload capabilities.
// Plugins that support networking should implement this interface.
type NetworkPlugin interface {
	Plugin

	// CreateBridgePort creates a new bridge port for a network function.
	CreateBridgePort(ctx context.Context, request *BridgePortRequest) (*BridgePort, error)

	// DeleteBridgePort removes a bridge port.
	DeleteBridgePort(ctx context.Context, portID string) error

	// GetBridgePort retrieves information about a bridge port.
	GetBridgePort(ctx context.Context, portID string) (*BridgePort, error)

	// ListBridgePorts lists all bridge ports managed by this plugin.
	ListBridgePorts(ctx context.Context) ([]*BridgePort, error)

	// SetVFCount configures the number of virtual functions for the device.
	SetVFCount(ctx context.Context, deviceID string, count int) error

	// GetVFCount returns the current number of virtual functions.
	GetVFCount(ctx context.Context, deviceID string) (int, error)

	// CreateNetworkFunction sets up a network function between input and output ports.
	CreateNetworkFunction(ctx context.Context, input, output string) error

	// DeleteNetworkFunction removes a network function.
	DeleteNetworkFunction(ctx context.Context, input, output string) error
}

// BridgePortRequest contains parameters for creating a bridge port.
type BridgePortRequest struct {
	// Name is the desired name for the bridge port.
	Name string

	// MACAddress is the MAC address for the port.
	MACAddress string

	// VLANID is the optional VLAN ID for the port.
	VLANID *int

	// Type specifies the port type (e.g., "trunk", "access").
	Type string

	// Metadata contains additional vendor-specific parameters.
	Metadata map[string]string
}

// BridgePort represents a created bridge port.
type BridgePort struct {
	// ID is the unique identifier for this port.
	ID string

	// Name is the port name.
	Name string

	// MACAddress is the port's MAC address.
	MACAddress string

	// VLANID is the configured VLAN ID, if any.
	VLANID *int

	// Status is the current port status.
	Status string

	// Metadata contains additional port information.
	Metadata map[string]string
}

// StoragePlugin extends Plugin with storage offload capabilities.
// Plugins that support NVMe-oF and storage operations should implement this.
type StoragePlugin interface {
	Plugin

	// CreateNVMeSubsystem creates an NVMe subsystem.
	CreateNVMeSubsystem(ctx context.Context, request *NVMeSubsystemRequest) (*NVMeSubsystem, error)

	// DeleteNVMeSubsystem removes an NVMe subsystem.
	DeleteNVMeSubsystem(ctx context.Context, subsystemID string) error

	// GetNVMeSubsystem retrieves information about an NVMe subsystem.
	GetNVMeSubsystem(ctx context.Context, subsystemID string) (*NVMeSubsystem, error)

	// ListNVMeSubsystems lists all NVMe subsystems.
	ListNVMeSubsystems(ctx context.Context) ([]*NVMeSubsystem, error)

	// CreateNVMeController creates an NVMe controller within a subsystem.
	CreateNVMeController(ctx context.Context, request *NVMeControllerRequest) (*NVMeController, error)

	// DeleteNVMeController removes an NVMe controller.
	DeleteNVMeController(ctx context.Context, controllerID string) error

	// CreateNVMeNamespace creates an NVMe namespace.
	CreateNVMeNamespace(ctx context.Context, request *NVMeNamespaceRequest) (*NVMeNamespace, error)

	// DeleteNVMeNamespace removes an NVMe namespace.
	DeleteNVMeNamespace(ctx context.Context, namespaceID string) error
}

// NVMeSubsystemRequest contains parameters for creating an NVMe subsystem.
type NVMeSubsystemRequest struct {
	// NQN is the NVMe Qualified Name for the subsystem.
	NQN string

	// SerialNumber is the subsystem serial number.
	SerialNumber string

	// MaxNamespaces is the maximum number of namespaces.
	MaxNamespaces int
}

// NVMeSubsystem represents an NVMe subsystem.
type NVMeSubsystem struct {
	// ID is the unique identifier.
	ID string

	// NQN is the NVMe Qualified Name.
	NQN string

	// SerialNumber is the serial number.
	SerialNumber string

	// Status is the current status.
	Status string
}

// NVMeControllerRequest contains parameters for creating an NVMe controller.
type NVMeControllerRequest struct {
	// SubsystemID is the parent subsystem.
	SubsystemID string

	// Name is the controller name.
	Name string

	// PCIeAddress is the PCIe function address.
	PCIeAddress string
}

// NVMeController represents an NVMe controller.
type NVMeController struct {
	// ID is the unique identifier.
	ID string

	// Name is the controller name.
	Name string

	// SubsystemID is the parent subsystem.
	SubsystemID string

	// Status is the current status.
	Status string
}

// NVMeNamespaceRequest contains parameters for creating an NVMe namespace.
type NVMeNamespaceRequest struct {
	// SubsystemID is the parent subsystem.
	SubsystemID string

	// NSID is the namespace ID.
	NSID int

	// Size is the namespace size in bytes.
	Size uint64

	// BlockSize is the logical block size.
	BlockSize int
}

// NVMeNamespace represents an NVMe namespace.
type NVMeNamespace struct {
	// ID is the unique identifier.
	ID string

	// NSID is the namespace ID.
	NSID int

	// Size is the size in bytes.
	Size uint64

	// Status is the current status.
	Status string
}

// SecurityPlugin extends Plugin with security offload capabilities.
// Plugins that support IPsec and encryption should implement this.
type SecurityPlugin interface {
	Plugin

	// CreateIPsecTunnel creates an IPsec tunnel.
	CreateIPsecTunnel(ctx context.Context, request *IPsecTunnelRequest) (*IPsecTunnel, error)

	// DeleteIPsecTunnel removes an IPsec tunnel.
	DeleteIPsecTunnel(ctx context.Context, tunnelID string) error

	// GetIPsecTunnel retrieves information about an IPsec tunnel.
	GetIPsecTunnel(ctx context.Context, tunnelID string) (*IPsecTunnel, error)

	// ListIPsecTunnels lists all IPsec tunnels.
	ListIPsecTunnels(ctx context.Context) ([]*IPsecTunnel, error)

	// UpdateIPsecKeys updates the keys for an IPsec tunnel (for rekeying).
	UpdateIPsecKeys(ctx context.Context, tunnelID string, keys *IPsecKeys) error

	// GetIPsecStats returns statistics for an IPsec tunnel.
	GetIPsecStats(ctx context.Context, tunnelID string) (*IPsecStats, error)
}

// IPsecTunnelRequest contains parameters for creating an IPsec tunnel.
type IPsecTunnelRequest struct {
	// Name is the tunnel name.
	Name string

	// LocalAddress is the local tunnel endpoint IP.
	LocalAddress string

	// RemoteAddress is the remote tunnel endpoint IP.
	RemoteAddress string

	// LocalSubnet is the local protected subnet (CIDR).
	LocalSubnet string

	// RemoteSubnet is the remote protected subnet (CIDR).
	RemoteSubnet string

	// Protocol specifies ESP or AH.
	Protocol string

	// EncryptionAlgorithm specifies the encryption algorithm (e.g., "aes-gcm-256").
	EncryptionAlgorithm string

	// IntegrityAlgorithm specifies the integrity algorithm (e.g., "sha256").
	IntegrityAlgorithm string

	// Keys contains the cryptographic keys.
	Keys *IPsecKeys
}

// IPsecKeys contains the cryptographic material for an IPsec tunnel.
type IPsecKeys struct {
	// EncryptionKey is the encryption key (hex encoded).
	EncryptionKey string

	// AuthenticationKey is the authentication key (hex encoded).
	AuthenticationKey string

	// SPI is the Security Parameter Index.
	SPI uint32
}

// IPsecTunnel represents an IPsec tunnel.
type IPsecTunnel struct {
	// ID is the unique identifier.
	ID string

	// Name is the tunnel name.
	Name string

	// LocalAddress is the local endpoint.
	LocalAddress string

	// RemoteAddress is the remote endpoint.
	RemoteAddress string

	// Status is the current status (up, down, establishing).
	Status string
}

// IPsecStats contains statistics for an IPsec tunnel.
type IPsecStats struct {
	// BytesEncrypted is the total bytes encrypted.
	BytesEncrypted uint64

	// BytesDecrypted is the total bytes decrypted.
	BytesDecrypted uint64

	// PacketsEncrypted is the total packets encrypted.
	PacketsEncrypted uint64

	// PacketsDecrypted is the total packets decrypted.
	PacketsDecrypted uint64

	// Errors is the total error count.
	Errors uint64
}

// PluginChecker provides type assertions for capability interfaces.
// This helps determine which optional interfaces a plugin implements.
type PluginChecker struct {
	plugin Plugin
}

// NewPluginChecker creates a new PluginChecker for the given plugin.
func NewPluginChecker(p Plugin) *PluginChecker {
	return &PluginChecker{plugin: p}
}

// IsNetworkPlugin returns true if the plugin implements NetworkPlugin.
func (c *PluginChecker) IsNetworkPlugin() bool {
	_, ok := c.plugin.(NetworkPlugin)
	return ok
}

// AsNetworkPlugin returns the plugin as NetworkPlugin, or nil if not supported.
func (c *PluginChecker) AsNetworkPlugin() NetworkPlugin {
	if np, ok := c.plugin.(NetworkPlugin); ok {
		return np
	}
	return nil
}

// IsStoragePlugin returns true if the plugin implements StoragePlugin.
func (c *PluginChecker) IsStoragePlugin() bool {
	_, ok := c.plugin.(StoragePlugin)
	return ok
}

// AsStoragePlugin returns the plugin as StoragePlugin, or nil if not supported.
func (c *PluginChecker) AsStoragePlugin() StoragePlugin {
	if sp, ok := c.plugin.(StoragePlugin); ok {
		return sp
	}
	return nil
}

// IsSecurityPlugin returns true if the plugin implements SecurityPlugin.
func (c *PluginChecker) IsSecurityPlugin() bool {
	_, ok := c.plugin.(SecurityPlugin)
	return ok
}

// AsSecurityPlugin returns the plugin as SecurityPlugin, or nil if not supported.
func (c *PluginChecker) AsSecurityPlugin() SecurityPlugin {
	if sp, ok := c.plugin.(SecurityPlugin); ok {
		return sp
	}
	return nil
}

// SupportsCapability checks if the plugin supports a given capability.
func (c *PluginChecker) SupportsCapability(cap Capability) bool {
	info := c.plugin.Info()
	for _, supported := range info.Capabilities {
		if supported == cap {
			return true
		}
	}
	return false
}
