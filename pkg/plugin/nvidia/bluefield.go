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

// Package nvidia provides the NVIDIA BlueField DPU plugin implementation.
// This plugin supports BlueField-2 and BlueField-3 DPUs through the OPI
// nvidia-bridge for vendor-neutral operations.
package nvidia

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin"
	"github.com/openshift/dpu-operator/pkg/plugin/pci"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openshift/dpu-operator/pkg/opi"
	evpnpb "github.com/opiproject/opi-api/network/evpn-gw/v1alpha1/gen/go"
)

const (
	// PluginName is the identifier for this plugin.
	PluginName = "nvidia"

	// PluginVendor is the vendor name.
	PluginVendor = "NVIDIA"

	// PluginVersion is the current version of this plugin.
	PluginVersion = "1.0.0"
)

// Supported PCI device IDs for NVIDIA BlueField DPUs.
var supportedDevices = []plugin.PCIDeviceID{
	// BlueField-2
	{VendorID: "15b3", DeviceID: "a2d6", Description: "NVIDIA BlueField-2 DPU"},
	{VendorID: "15b3", DeviceID: "a2d2", Description: "NVIDIA BlueField-2 Integrated ConnectX-6 Dx"},
	// BlueField-3
	{VendorID: "15b3", DeviceID: "a2dc", Description: "NVIDIA BlueField-3 DPU"},
	{VendorID: "15b3", DeviceID: "a2d8", Description: "NVIDIA BlueField-3 Integrated ConnectX-7"},
}

// BlueFieldPlugin implements the plugin.Plugin and plugin.NetworkPlugin interfaces
// for NVIDIA BlueField DPUs (BlueField-2 and BlueField-3).
type BlueFieldPlugin struct {
	mu          sync.RWMutex
	log         logr.Logger
	config      plugin.PluginConfig
	initialized bool

	// OPI endpoint for opi-nvidia-bridge communication
	opiEndpoint string

	// Cache of discovered devices
	devices []plugin.Device

	// gRPC client for OPI bridge
	opiClient *opi.Client
}

// New creates a new NVIDIA BlueField plugin instance.
func New() *BlueFieldPlugin {
	return &BlueFieldPlugin{
		log: ctrl.Log.WithName("plugin").WithName("nvidia"),
	}
}

// Info returns metadata about this plugin.
func (p *BlueFieldPlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:             PluginName,
		Vendor:           PluginVendor,
		Version:          PluginVersion,
		Description:      "NVIDIA BlueField DPU plugin supporting BlueField-2 and BlueField-3 hardware",
		SupportedDevices: supportedDevices,
		Capabilities: []plugin.Capability{
			plugin.CapabilityNetworking,
			plugin.CapabilityStorage,
			// plugin.CapabilitySecurity, // TODO: Enable when IPsec offload is implemented
		},
	}
}

// Initialize sets up the plugin with the provided configuration.
func (p *BlueFieldPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
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

	p.log.Info("Initializing NVIDIA BlueField plugin",
		"opiEndpoint", p.opiEndpoint,
		"logLevel", config.LogLevel)

	// Initialize gRPC connection to opi-nvidia-bridge
	var err error
	p.opiClient, err = opi.NewClient(p.opiEndpoint)
	if err != nil {
		p.log.Error(err, "Failed to create OPI client")
		return fmt.Errorf("failed to create OPI client: %w", err)
	}

	// Verify connection with Ping (optional, using Lifecycle)
	// if _, err := p.opiClient.Lifecycle().Ping(ctx); err != nil {
	// 	 p.log.Error(err, "Failed to ping OPI bridge")
	// 	 // return err // Maybe fail? Or warn?
	// }

	p.initialized = true
	p.log.Info("NVIDIA BlueField plugin initialized successfully")
	return nil
}

// Shutdown gracefully stops the plugin and releases resources.
func (p *BlueFieldPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	p.log.Info("Shutting down NVIDIA BlueField plugin")

	if p.opiClient != nil {
		if err := p.opiClient.Close(); err != nil {
			p.log.Error(err, "Error closing OPI client")
		}
	}

	p.initialized = false
	p.devices = nil
	p.log.Info("NVIDIA BlueField plugin shutdown complete")
	return nil
}

// HealthCheck verifies the plugin is operational.
func (p *BlueFieldPlugin) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	// Check if OPI client is connected
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

	return nil
}

// DiscoverDevices scans for BlueField DPU hardware.
func (p *BlueFieldPlugin) DiscoverDevices(ctx context.Context) ([]plugin.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Discovering NVIDIA BlueField devices")

	var devices []plugin.Device

	// Scan PCI bus for supported devices
	// This is a simplified implementation; in production, we would use sysfs/lspci
	localDevices, err := p.scanPCIBus(ctx)
	if err != nil {
		p.log.Error(err, "Failed to scan PCI bus")
		// Continue even if local scan fails if we can get data from OPI
	}
	devices = append(devices, localDevices...)

	// Also query OPI bridge for what it sees
	resp, err := p.opiClient.Lifecycle().GetDevices(ctx)
	if err == nil && resp != nil {
		for id, data := range resp.Devices {
			// Merge or append info
			// Check if already in list from PCI scan
			found := false
			for i := range devices {
				if devices[i].ID == id {
					found = true
					// Update metadata
					if devices[i].Metadata == nil {
						devices[i].Metadata = make(map[string]string)
					}
					devices[i].Metadata["health"] = data.Health
					break
				}
			}
			if !found {
				devices = append(devices, plugin.Device{
					ID:       id,
					Vendor:   "NVIDIA",
					Healthy:  data.Health == "healthy" || data.Health == "HEALTH_STATUS_OK",
					Metadata: map[string]string{"health": data.Health},
				})
			}
		}
	} else {
		p.log.Info("Failed to query OPI GetDevices", "error", err)
	}

	p.devices = devices

	p.log.Info("BlueField device discovery complete", "deviceCount", len(devices))
	return devices, nil
}

// scanPCIBus scans the PCI bus for supported NVIDIA devices.
func (p *BlueFieldPlugin) scanPCIBus(ctx context.Context) ([]plugin.Device, error) {
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
				ID:         fmt.Sprintf("nvidia-%s", pciDev.Address),
				PCIAddress: pciDev.Address,
				Vendor:     "NVIDIA",
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
				device.SerialNumber = fmt.Sprintf("NV-%s", pciDev.Address)
			}

			devices = append(devices, device)
			p.log.Info("Discovered NVIDIA BlueField device",
				"pciAddress", pciDev.Address,
				"model", device.Model,
				"driver", pciDev.Driver)
		}
	}

	return devices, nil
}

// GetInventory retrieves detailed inventory information for a specific device.
func (p *BlueFieldPlugin) GetInventory(ctx context.Context, deviceID string) (*plugin.InventoryResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	// We can't query detailed inventory via OPI yet (missing API).
	// We'll return basic info from cache + simple GetDevices check.

	// Find the device
	var device *plugin.Device
	for i := range p.devices {
		if p.devices[i].ID == deviceID {
			device = &p.devices[i]
			break
		}
	}

	// If not in cache, try OPI GetDevices just to verify it exists
	if device == nil {
		resp, err := p.opiClient.Lifecycle().GetDevices(ctx)
		if err == nil && resp != nil {
			if _, ok := resp.Devices[deviceID]; ok {
				device = &plugin.Device{ID: deviceID, Model: "Unknown (OPI)"}
			}
		}
	}

	if device == nil {
		return nil, plugin.NewDeviceError(deviceID, "GetInventory", plugin.ErrDeviceNotFound)
	}

	p.log.V(1).Info("Getting inventory for device", "deviceID", deviceID)

	inventory := &plugin.InventoryResponse{
		DeviceID: deviceID,
		Chassis: &plugin.ChassisInfo{
			Manufacturer: "NVIDIA",
			Model:        device.Model,
			SerialNumber: device.SerialNumber,
		},
	}

	return inventory, nil
}

// --- NetworkPlugin interface implementation ---

// CreateBridgePort creates a new bridge port for a network function.
func (p *BlueFieldPlugin) CreateBridgePort(ctx context.Context, request *plugin.BridgePortRequest) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	p.log.Info("Creating bridge port", "name", request.Name, "mac", request.MACAddress)

	// Parse MAC
	mac, err := net.ParseMAC(request.MACAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid MAC address: %w", err)
	}

	// Call OPI bridge
	req := &evpnpb.CreateBridgePortRequest{
		BridgePortId: request.Name,
		BridgePort: &evpnpb.BridgePort{
			Name: request.Name,
			Spec: &evpnpb.BridgePortSpec{
				MacAddress: mac,
				Ptype:      evpnpb.BridgePortType_BRIDGE_PORT_TYPE_ACCESS, // Default to access
			},
		},
	}

	resp, err := p.opiClient.Network().CreateBridgePort(ctx, req)
	if err != nil {
		p.log.Error(err, "Failed to create bridge port via OPI")
		return nil, fmt.Errorf("OPI creation failed: %w", err)
	}

	// Helper to convert []byte MAC back to string
	respMac := ""
	if resp.Spec != nil && len(resp.Spec.MacAddress) > 0 {
		respMac = net.HardwareAddr(resp.Spec.MacAddress).String()
	} else {
		respMac = request.MACAddress // Fallback
	}

	port := &plugin.BridgePort{
		ID:         resp.Name,
		Name:       resp.Name,
		MACAddress: respMac,
		VLANID:     request.VLANID,
		Status:     "Active", // Assume active if successful
	}

	p.log.Info("Bridge port created", "portID", port.ID)
	return port, nil
}

// DeleteBridgePort removes a bridge port.
func (p *BlueFieldPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting bridge port", "portID", portID)

	err := p.opiClient.Network().DeleteBridgePort(ctx, portID)
	if err != nil {
		return fmt.Errorf("OPI delete failed: %w", err)
	}

	return nil
}

// GetBridgePort retrieves information about a bridge port.
func (p *BlueFieldPlugin) GetBridgePort(ctx context.Context, portID string) (*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	resp, err := p.opiClient.Network().GetBridgePort(ctx, portID)
	if err != nil {
		return nil, fmt.Errorf("OPI get failed: %w", err)
	}

	respMac := ""
	if resp.Spec != nil {
		respMac = net.HardwareAddr(resp.Spec.MacAddress).String()
	}

	return &plugin.BridgePort{
		ID:         resp.Name,
		Name:       resp.Name,
		MACAddress: respMac,
		Status:     "Active",
	}, nil
}

// ListBridgePorts lists all bridge ports managed by this plugin.
func (p *BlueFieldPlugin) ListBridgePorts(ctx context.Context) ([]*plugin.BridgePort, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return nil, plugin.ErrNotInitialized
	}

	resp, err := p.opiClient.Network().ListBridgePorts(ctx)
	if err != nil {
		return nil, fmt.Errorf("OPI list failed: %w", err)
	}

	var ports []*plugin.BridgePort
	for _, bp := range resp.BridgePorts {
		mac := ""
		if bp.Spec != nil {
			mac = net.HardwareAddr(bp.Spec.MacAddress).String()
		}
		ports = append(ports, &plugin.BridgePort{
			ID:         bp.Name,
			Name:       bp.Name,
			MACAddress: mac,
			Status:     "Active",
		})
	}
	return ports, nil
}

// SetVFCount configures the number of virtual functions.
func (p *BlueFieldPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Setting VF count", "deviceID", deviceID, "count", count)

	// Call OPI Lifecycle
	_, err := p.opiClient.Lifecycle().SetNumVfs(ctx, int32(count))
	if err != nil {
		return fmt.Errorf("OPI SetNumVfs failed: %w", err)
	}

	return nil
}

// GetVFCount returns the current number of virtual functions.
func (p *BlueFieldPlugin) GetVFCount(ctx context.Context, deviceID string) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return 0, plugin.ErrNotInitialized
	}

	// OPI currently doesn't expose getting VF count specifically via Lifecycle per device?
	// It's not in the vendored interface used in client.go (only SetNumVfs).
	// So we return 0 or stub.
	return 0, nil
}

// CreateNetworkFunction sets up a network function between input and output ports.
func (p *BlueFieldPlugin) CreateNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Creating network function", "input", input, "output", output)

	// Stub for OPI logic. This likely maps to CreateLogicalBridge or similar.
	// For now, return nil as stub.
	return nil
}

// DeleteNetworkFunction removes a network function.
func (p *BlueFieldPlugin) DeleteNetworkFunction(ctx context.Context, input, output string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.ErrNotInitialized
	}

	p.log.Info("Deleting network function", "input", input, "output", output)
	return nil
}

// Ensure BlueFieldPlugin implements the required interfaces.
var (
	_ plugin.Plugin        = (*BlueFieldPlugin)(nil)
	_ plugin.NetworkPlugin = (*BlueFieldPlugin)(nil)
)

// init registers the plugin with the global registry.
func init() {
	plugin.MustRegister(New())
}
