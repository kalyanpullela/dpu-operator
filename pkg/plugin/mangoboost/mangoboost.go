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

// Package mangoboost provides the MangoBoost DPU plugin implementation.
// This plugin supports MangoBoost DPU devices.
package mangoboost

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// PluginName is the identifier for this plugin.
	PluginName = "mangoboost"

	// PluginVendor is the vendor name.
	PluginVendor = "MangoBoost"

	// PluginVersion is the current version of this plugin.
	PluginVersion = "1.0.0"
)

// Supported PCI device IDs for MangoBoost DPU devices.
// TODO: Update with actual MangoBoost PCI device IDs when hardware/SDK is available
var supportedDevices = []plugin.PCIDeviceID{
	{VendorID: "1f67", DeviceID: "0001", Description: "MangoBoost DPU"},
	{VendorID: "1f67", DeviceID: "0002", Description: "MangoBoost DPU Gen2"},
}

// MangoBoostPlugin implements the plugin.Plugin and plugin.NetworkPlugin interfaces
// for MangoBoost DPU devices.
type MangoBoostPlugin struct {
	mu          sync.RWMutex
	log         logr.Logger
	config      plugin.PluginConfig
	initialized bool

	// OPI endpoint for OPI bridge communication
	opiEndpoint string

	// Cache of discovered devices
	devices []plugin.Device
}

// New creates a new MangoBoost plugin instance.
func New() *MangoBoostPlugin {
	return &MangoBoostPlugin{
		log: ctrl.Log.WithName("plugin").WithName("mangoboost"),
	}
}

// Info returns metadata about this plugin.
func (p *MangoBoostPlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:             PluginName,
		Vendor:           PluginVendor,
		Version:          PluginVersion,
		Description:      "MangoBoost DPU plugin for MangoBoost hardware",
		SupportedDevices: supportedDevices,
		Capabilities:     []plugin.Capability{},
	}
}

// Initialize sets up the plugin with the provided configuration.
func (p *MangoBoostPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return plugin.ErrAlreadyInitialized
	}

	p.config = config
	p.opiEndpoint = config.OPIEndpoint
	if p.opiEndpoint == "" {
		p.opiEndpoint = "localhost:50051"
	}

	p.log.Info("Initializing MangoBoost plugin",
		"opiEndpoint", p.opiEndpoint,
		"logLevel", config.LogLevel)

	// TODO: Initialize connection to MangoBoost SDK/OPI bridge when available

	p.initialized = true
	p.log.Info("MangoBoost plugin initialized successfully")
	return nil
}

// Shutdown gracefully stops the plugin and releases resources.
func (p *MangoBoostPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	p.log.Info("Shutting down MangoBoost plugin")

	p.initialized = false
	p.devices = nil
	p.log.Info("MangoBoost plugin shutdown complete")
	return nil
}

// HealthCheck verifies the plugin is operational.
func (p *MangoBoostPlugin) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	return nil
}

// DiscoverDevices scans for MangoBoost hardware.
func (p *MangoBoostPlugin) DiscoverDevices(ctx context.Context) ([]plugin.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Discovering MangoBoost devices")

	var devices []plugin.Device

	discoveredDevices, err := p.scanPCIBus(ctx)
	if err != nil {
		p.log.Error(err, "Failed to scan PCI bus")
		return nil, fmt.Errorf("PCI scan failed: %w", err)
	}

	devices = append(devices, discoveredDevices...)
	p.devices = devices

	p.log.Info("MangoBoost device discovery complete", "deviceCount", len(devices))
	return devices, nil
}

// scanPCIBus scans the PCI bus for supported MangoBoost devices.
func (p *MangoBoostPlugin) scanPCIBus(ctx context.Context) ([]plugin.Device, error) {
	var devices []plugin.Device
	// TODO: Implement actual PCI bus scanning when MangoBoost SDK is available
	return devices, nil
}

// GetInventory retrieves detailed inventory information for a specific device.
func (p *MangoBoostPlugin) GetInventory(ctx context.Context, deviceID string) (*plugin.InventoryResponse, error) {
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
			Manufacturer: "MangoBoost",
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		},
	}

	return inventory, nil
}

// --- NetworkPlugin interface implementation ---

// CreateBridgePort creates a new bridge port for a network function.
func (p *MangoBoostPlugin) CreateBridgePort(ctx context.Context, request *plugin.BridgePortRequest) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("CreateBridgePort not implemented for MangoBoost plugin", "name", request.Name)
	return nil, plugin.ErrNotImplemented
}

// DeleteBridgePort removes a bridge port.
func (p *MangoBoostPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("DeleteBridgePort not implemented for MangoBoost plugin", "portID", portID)
	return plugin.ErrNotImplemented
}

// GetBridgePort retrieves information about a bridge port.
func (p *MangoBoostPlugin) GetBridgePort(ctx context.Context, portID string) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// ListBridgePorts lists all bridge ports managed by this plugin.
func (p *MangoBoostPlugin) ListBridgePorts(ctx context.Context) ([]*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	return nil, plugin.ErrNotImplemented
}

// SetVFCount configures the number of virtual functions.
func (p *MangoBoostPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("SetVFCount not implemented for MangoBoost plugin", "deviceID", deviceID, "count", count)
	return plugin.ErrNotImplemented
}

// GetVFCount returns the current number of virtual functions.
func (p *MangoBoostPlugin) GetVFCount(ctx context.Context, deviceID string) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return 0, plugin.ErrNotInitialized
	}

	return 0, plugin.ErrNotImplemented
}

// CreateNetworkFunction sets up a network function between input and output ports.
func (p *MangoBoostPlugin) CreateNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("CreateNetworkFunction not implemented for MangoBoost plugin", "input", input, "output", output)
	return plugin.ErrNotImplemented
}

// DeleteNetworkFunction removes a network function.
func (p *MangoBoostPlugin) DeleteNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("DeleteNetworkFunction not implemented for MangoBoost plugin", "input", input, "output", output)
	return plugin.ErrNotImplemented
}

// Ensure MangoBoostPlugin implements the required interfaces.
var (
	_ plugin.Plugin        = (*MangoBoostPlugin)(nil)
	_ plugin.NetworkPlugin = (*MangoBoostPlugin)(nil)
)

// init registers the plugin with the global registry.
func init() {
	plugin.MustRegister(New())
}
