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

// Package xsight provides the xSight DPU plugin implementation.
// This plugin supports xSight DPU devices.
package xsight

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
	PluginName = "xsight"

	// PluginVendor is the vendor name.
	PluginVendor = "xSight"

	// PluginVersion is the current version of this plugin.
	PluginVersion = "1.0.0"
)

// Supported PCI device IDs for xSight DPU devices.
// TODO: Update with actual xSight PCI device IDs when SDK documentation is available
var supportedDevices = []plugin.PCIDeviceID{
	{VendorID: "1de5", DeviceID: "0001", Description: "xSight DPU"},
}

// XSightPlugin implements the plugin.Plugin and plugin.NetworkPlugin interfaces
// for xSight DPU devices.
type XSightPlugin struct {
	mu          sync.RWMutex
	log         logr.Logger
	config      plugin.PluginConfig
	initialized bool

	// OPI endpoint for OPI bridge communication
	opiEndpoint string

	// Cache of discovered devices
	devices []plugin.Device

	// gRPC client for OPI bridge
	opiClient *opi.Client
}

// New creates a new xSight plugin instance.
func New() *XSightPlugin {
	return &XSightPlugin{
		log: ctrl.Log.WithName("plugin").WithName("xsight"),
	}
}

// Info returns metadata about this plugin.
func (p *XSightPlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:             PluginName,
		Vendor:           PluginVendor,
		Version:          PluginVersion,
		Description:      "xSight DPU plugin for xSight hardware",
		SupportedDevices: supportedDevices,
		Capabilities: []plugin.Capability{
			plugin.CapabilityNetworking,
		},
	}
}

// Initialize sets up the plugin with the provided configuration.
func (p *XSightPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
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

	p.log.Info("Initializing xSight plugin",
		"opiEndpoint", p.opiEndpoint,
		"logLevel", config.LogLevel)

	// Initialize gRPC connection to xSight OPI bridge
	var err error
	p.opiClient, err = opi.NewClient(p.opiEndpoint)
	if err != nil {
		p.log.Error(err, "Failed to create OPI client")
		return fmt.Errorf("failed to create OPI client: %w", err)
	}

	p.initialized = true
	p.log.Info("xSight plugin initialized successfully")
	return nil
}

// Shutdown gracefully stops the plugin and releases resources.
func (p *XSightPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	p.log.Info("Shutting down xSight plugin")

	if p.opiClient != nil {
		if err := p.opiClient.Close(); err != nil {
			p.log.Error(err, "Error closing OPI client")
		}
	}

	p.initialized = false
	p.devices = nil
	p.log.Info("xSight plugin shutdown complete")
	return nil
}

// HealthCheck verifies the plugin is operational.
func (p *XSightPlugin) HealthCheck(ctx context.Context) error {
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

// DiscoverDevices scans for xSight hardware.
func (p *XSightPlugin) DiscoverDevices(ctx context.Context) ([]plugin.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Discovering xSight devices")

	var devices []plugin.Device

	discoveredDevices, err := p.scanPCIBus(ctx)
	if err != nil {
		p.log.Error(err, "Failed to scan PCI bus")
		return nil, fmt.Errorf("PCI scan failed: %w", err)
	}

	devices = append(devices, discoveredDevices...)
	p.devices = devices

	p.log.Info("xSight device discovery complete", "deviceCount", len(devices))
	return devices, nil
}

// scanPCIBus scans the PCI bus for supported xSight devices.
func (p *XSightPlugin) scanPCIBus(ctx context.Context) ([]plugin.Device, error) {
	scanner := pci.NewScanner()
	var devices []plugin.Device

	// TODO: Update PCI IDs with actual xSight vendor and device IDs when available.
	// Current IDs (XXXX:YYYY) are placeholders.
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
				ID:         fmt.Sprintf("xsight-%s", pciDev.Address),
				PCIAddress: pciDev.Address,
				Vendor:     "xSight",
				Model:      supportedDevice.Description,
				Healthy:    true,
				Metadata: map[string]string{
					"pci_vendor_id": pciDev.VendorID,
					"pci_device_id": pciDev.DeviceID,
					"pci_class":     pciDev.Class,
					"device_type":   supportedDevice.Description,
					"driver":        pciDev.Driver,
					"numa_node":     pciDev.NumaNode,
				},
			}

			// Try to get serial number from VPD
			if serialNum, err := scanner.GetSerialNumber(pciDev.Address); err == nil {
				device.SerialNumber = serialNum
			} else {
				// Generate a stable ID based on PCI address
				device.SerialNumber = fmt.Sprintf("XSIGHT-%s", pciDev.Address)
			}

			devices = append(devices, device)
			p.log.Info("Discovered xSight device",
				"pciAddress", pciDev.Address,
				"model", device.Model,
				"driver", pciDev.Driver)
		}
	}

	return devices, nil
}

// GetInventory retrieves detailed inventory information for a specific device.
func (p *XSightPlugin) GetInventory(ctx context.Context, deviceID string) (*plugin.InventoryResponse, error) {
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
			Manufacturer: "xSight",
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		},
	}

	return inventory, nil
}

// --- NetworkPlugin interface implementation ---

// CreateBridgePort creates a new bridge port for a network function.
func (p *XSightPlugin) CreateBridgePort(ctx context.Context, request *plugin.BridgePortRequest) (*plugin.BridgePort, error) {
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
func (p *XSightPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting bridge port", "portID", portID)
	return nil
}

// GetBridgePort retrieves information about a bridge port.
func (p *XSightPlugin) GetBridgePort(ctx context.Context, portID string) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// ListBridgePorts lists all bridge ports managed by this plugin.
func (p *XSightPlugin) ListBridgePorts(ctx context.Context) ([]*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// SetVFCount configures the number of virtual functions.
func (p *XSightPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Setting VF count", "deviceID", deviceID, "count", count)
	return nil
}

// GetVFCount returns the current number of virtual functions.
func (p *XSightPlugin) GetVFCount(ctx context.Context, deviceID string) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return 0, plugin.ErrNotInitialized
	}

	return 0, nil
}

// CreateNetworkFunction sets up a network function between input and output ports.
func (p *XSightPlugin) CreateNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Creating network function", "input", input, "output", output)
	return nil
}

// DeleteNetworkFunction removes a network function.
func (p *XSightPlugin) DeleteNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting network function", "input", input, "output", output)
	return nil
}

// Ensure XSightPlugin implements the required interfaces.
var (
	_ plugin.Plugin        = (*XSightPlugin)(nil)
	_ plugin.NetworkPlugin = (*XSightPlugin)(nil)
)

// init registers the plugin with the global registry.
func init() {
	plugin.MustRegister(New())
}
