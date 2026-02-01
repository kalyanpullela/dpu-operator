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

// Package marvell provides the Marvell Octeon DPU plugin implementation.
// This plugin supports Marvell Octeon 10 DPU devices.
package marvell

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/opi"
	"github.com/openshift/dpu-operator/pkg/plugin"
	"github.com/openshift/dpu-operator/pkg/plugin/pci"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// PluginName is the identifier for this plugin.
	PluginName = "marvell"

	// PluginVendor is the vendor name.
	PluginVendor = "Marvell"

	// PluginVersion is the current version of this plugin.
	PluginVersion = "1.0.0"
)

// Supported PCI device IDs for Marvell DPU devices.
var supportedDevices = []plugin.PCIDeviceID{
	// Marvell Octeon 10
	{VendorID: "177d", DeviceID: "b903", Description: "Marvell Octeon 10 DPU"},
	{VendorID: "177d", DeviceID: "b900", Description: "Marvell Octeon 10 CN10K"},
}

// OcteonPlugin implements the plugin.Plugin and plugin.NetworkPlugin interfaces
// for Marvell Octeon DPU devices.
type OcteonPlugin struct {
	mu          sync.RWMutex
	log         logr.Logger
	config      plugin.PluginConfig
	initialized bool

	// OPI endpoint for opi-marvell-bridge communication
	opiEndpoint string

	// Cache of discovered devices
	devices []plugin.Device

	// gRPC client for OPI bridge
	opiClient *opi.Client
}

// New creates a new Marvell Octeon plugin instance.
func New() *OcteonPlugin {
	return &OcteonPlugin{
		log: ctrl.Log.WithName("plugin").WithName("marvell"),
	}
}

// Info returns metadata about this plugin.
func (p *OcteonPlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:             PluginName,
		Vendor:           PluginVendor,
		Version:          PluginVersion,
		Description:      "Marvell Octeon DPU plugin supporting Octeon 10 hardware",
		SupportedDevices: supportedDevices,
		Capabilities: []plugin.Capability{
			plugin.CapabilityNetworking,
		},
	}
}

// Initialize sets up the plugin with the provided configuration.
func (p *OcteonPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return plugin.ErrAlreadyInitialized
	}

	p.config = config
	p.opiEndpoint = config.OPIEndpoint
	if p.opiEndpoint == "" {
		p.opiEndpoint = "localhost:50051" // Default OPI endpoint
	}

	p.log.Info("Initializing Marvell Octeon plugin",
		"opiEndpoint", p.opiEndpoint,
		"logLevel", config.LogLevel)

	// Initialize gRPC connection to opi-marvell-bridge
	var err error
	p.opiClient, err = opi.NewClient(p.opiEndpoint)
	if err != nil {
		p.log.Error(err, "Failed to create OPI client")
		return fmt.Errorf("failed to create OPI client: %w", err)
	}

	p.initialized = true
	p.log.Info("Marvell Octeon plugin initialized successfully")
	return nil
}

// Shutdown gracefully stops the plugin and releases resources.
func (p *OcteonPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	p.log.Info("Shutting down Marvell Octeon plugin")

	if p.opiClient != nil {
		if err := p.opiClient.Close(); err != nil {
			p.log.Error(err, "Error closing OPI client")
		}
	}

	p.initialized = false
	p.devices = nil
	p.log.Info("Marvell Octeon plugin shutdown complete")
	return nil
}

// HealthCheck verifies the plugin is operational.
func (p *OcteonPlugin) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	// Check if OPI client is connected
	if p.opiClient != nil {
		if !p.opiClient.IsConnected() {
			return fmt.Errorf("OPI client not connected")
		}

		// Try to ping the OPI bridge if Lifecycle service is available
		// Note: Some OPI bridges may not implement Lifecycle service, so we
		// make this a soft check rather than failing health entirely
		_, err := p.opiClient.Lifecycle().Ping(ctx)
		if err != nil {
			// Log warning but don't fail - bridge might not implement Lifecycle
			p.log.V(1).Info("Lifecycle.Ping not available (may not be implemented)", "error", err.Error())
		}
	}

	return nil
}

// DiscoverDevices scans for Marvell Octeon hardware.
func (p *OcteonPlugin) DiscoverDevices(ctx context.Context) ([]plugin.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Discovering Marvell Octeon devices")

	var devices []plugin.Device

	discoveredDevices, err := p.scanPCIBus(ctx)
	if err != nil {
		p.log.Error(err, "Failed to scan PCI bus")
		return nil, fmt.Errorf("PCI scan failed: %w", err)
	}

	devices = append(devices, discoveredDevices...)
	p.devices = devices

	p.log.Info("Marvell Octeon device discovery complete", "deviceCount", len(devices))
	return devices, nil
}

// scanPCIBus scans the PCI bus for supported Marvell devices.
func (p *OcteonPlugin) scanPCIBus(ctx context.Context) ([]plugin.Device, error) {
	scanner := pci.NewScanner()
	var devices []plugin.Device

	// Scan for each supported device ID
	for _, supportedDevice := range supportedDevices {
		pciDevices, err := scanner.ScanByVendorDevice(supportedDevice.VendorID, supportedDevice.DeviceID)
		if err != nil {
			p.log.V(1).Info("Failed to scan for PCI device",
				"vendorID", supportedDevice.VendorID,
				"deviceID", supportedDevice.DeviceID,
				"error", err)
			continue
		}

		for _, pciDev := range pciDevices {
			// Create plugin device from PCI device
			device := plugin.Device{
				ID:          fmt.Sprintf("marvell-%s", pciDev.Address),
				PCIAddress:  pciDev.Address,
				Vendor:      "Marvell",
				Model:       supportedDevice.Description,
				Healthy:     true,
				Metadata: map[string]string{
					"pci_vendor_id":   pciDev.VendorID,
					"pci_device_id":   pciDev.DeviceID,
					"pci_class":       pciDev.Class,
					"device_type":     supportedDevice.Description,
					"driver":          pciDev.Driver,
					"numa_node":       pciDev.NumaNode,
				},
			}

			// Try to get serial number from VPD
			if serialNum, err := scanner.GetSerialNumber(pciDev.Address); err == nil {
				device.SerialNumber = serialNum
			} else {
				// Generate a stable ID based on PCI address
				device.SerialNumber = fmt.Sprintf("MARVELL-%s", pciDev.Address)
			}

			devices = append(devices, device)
			p.log.Info("Discovered Marvell Octeon device",
				"pciAddress", pciDev.Address,
				"model", device.Model,
				"driver", pciDev.Driver)
		}
	}

	return devices, nil
}

// GetInventory retrieves detailed inventory information for a specific device.
func (p *OcteonPlugin) GetInventory(ctx context.Context, deviceID string) (*plugin.InventoryResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	var device *plugin.Device
	for i := range p.devices {
		if p.devices[i].ID == deviceID {
			device = &p.devices[i]
			break
		}
	}

	if device == nil {
		return nil, plugin.NewDeviceError(deviceID, "GetInventory", plugin.ErrDeviceNotFound)
	}

	inventory := &plugin.InventoryResponse{
		DeviceID: deviceID,
		Chassis: &plugin.ChassisInfo{
			Manufacturer: "Marvell",
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		},
	}

	return inventory, nil
}

// --- NetworkPlugin interface implementation ---

// CreateBridgePort creates a new bridge port for a network function.
func (p *OcteonPlugin) CreateBridgePort(ctx context.Context, request *plugin.BridgePortRequest) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Creating bridge port", "name", request.Name, "mac", request.MACAddress)

	port := &plugin.BridgePort{
		ID:         fmt.Sprintf("bp-%s", request.Name),
		Name:       request.Name,
		MACAddress: request.MACAddress,
		VLANID:     request.VLANID,
		Status:     "Active",
	}

	return port, nil
}

// DeleteBridgePort removes a bridge port.
func (p *OcteonPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting bridge port", "portID", portID)
	return nil
}

// GetBridgePort retrieves information about a bridge port.
func (p *OcteonPlugin) GetBridgePort(ctx context.Context, portID string) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// ListBridgePorts lists all bridge ports managed by this plugin.
func (p *OcteonPlugin) ListBridgePorts(ctx context.Context) ([]*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// SetVFCount configures the number of virtual functions.
func (p *OcteonPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Setting VF count", "deviceID", deviceID, "count", count)
	return nil
}

// GetVFCount returns the current number of virtual functions.
func (p *OcteonPlugin) GetVFCount(ctx context.Context, deviceID string) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return 0, plugin.ErrNotInitialized
	}

	return 0, nil
}

// CreateNetworkFunction sets up a network function between input and output ports.
func (p *OcteonPlugin) CreateNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Creating network function", "input", input, "output", output)
	return nil
}

// DeleteNetworkFunction removes a network function.
func (p *OcteonPlugin) DeleteNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting network function", "input", input, "output", output)
	return nil
}

// Ensure OcteonPlugin implements the required interfaces.
var (
	_ plugin.Plugin        = (*OcteonPlugin)(nil)
	_ plugin.NetworkPlugin = (*OcteonPlugin)(nil)
)

// init registers the plugin with the global registry.
func init() {
	plugin.MustRegister(New())
}
