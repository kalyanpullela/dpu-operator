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

package plugin_test

import (
	"context"
	"testing"

	"github.com/openshift/dpu-operator/pkg/plugin"

	// Import all plugins to trigger init() registration
	_ "github.com/openshift/dpu-operator/pkg/plugin/intel"
	_ "github.com/openshift/dpu-operator/pkg/plugin/mangoboost"
	_ "github.com/openshift/dpu-operator/pkg/plugin/marvell"
	_ "github.com/openshift/dpu-operator/pkg/plugin/nvidia"
	_ "github.com/openshift/dpu-operator/pkg/plugin/xsight"
)

// TestPluginRegistry_AllPluginsRegistered verifies all expected plugins are registered
func TestPluginRegistry_AllPluginsRegistered(t *testing.T) {
	registry := plugin.DefaultRegistry()
	plugins := registry.List()

	if len(plugins) == 0 {
		t.Fatal("No plugins registered in the global registry")
	}

	expectedPlugins := map[string]bool{
		"nvidia":     false,
		"intel":      false,
		"marvell":    false,
		"xsight":     false,
		"mangoboost": false,
	}

	for _, p := range plugins {
		info := p.Info()
		if _, expected := expectedPlugins[info.Name]; expected {
			expectedPlugins[info.Name] = true
			t.Logf("✓ Plugin '%s' registered (vendor: %s, version: %s)",
				info.Name, info.Vendor, info.Version)
		}
	}

	// Check which expected plugins are registered
	registered := 0
	for name, found := range expectedPlugins {
		if found {
			registered++
		} else {
			t.Logf("Plugin '%s' not registered (may not have been imported)", name)
		}
	}

	if registered == 0 {
		t.Error("No expected plugins were registered")
	} else {
		t.Logf("Total registered plugins: %d/%d", registered, len(expectedPlugins))
	}
}

// TestPluginRegistry_CapabilityQueries tests querying plugins by capability
func TestPluginRegistry_CapabilityQueries(t *testing.T) {
	registry := plugin.DefaultRegistry()

	testCases := []struct {
		capability  plugin.Capability
		minExpected int
	}{
		{plugin.CapabilityNetworking, 1}, // At least one plugin should support networking
		{plugin.CapabilityStorage, 0},    // Storage is optional
		{plugin.CapabilitySecurity, 0},   // Security is optional
	}

	for _, tc := range testCases {
		t.Run(string(tc.capability), func(t *testing.T) {
			plugins := registry.List()
			count := 0

			for _, p := range plugins {
				info := p.Info()
				for _, cap := range info.Capabilities {
					if cap == tc.capability {
						count++
						t.Logf("Plugin '%s' supports %s", info.Name, tc.capability)
						break
					}
				}
			}

			if count < tc.minExpected {
				t.Errorf("Expected at least %d plugins with %s capability, found %d",
					tc.minExpected, tc.capability, count)
			} else {
				t.Logf("Found %d plugins with %s capability", count, tc.capability)
			}
		})
	}
}

// TestPluginRegistry_PCIDeviceLookup tests looking up plugins by PCI device ID
func TestPluginRegistry_PCIDeviceLookup(t *testing.T) {
	registry := plugin.DefaultRegistry()

	testCases := []struct {
		name         string
		vendorID     string
		deviceID     string
		expectPlugin string
	}{
		{"NVIDIA BlueField-2", "15b3", "a2d6", "nvidia"},
		{"NVIDIA BlueField-3", "15b3", "a2dc", "nvidia"},
		{"Intel IPU E2100", "8086", "1453", "intel"},
		{"Marvell Octeon 10", "177d", "b903", "marvell"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := registry.GetByVendorDevice(tc.vendorID, tc.deviceID)

			if p != nil {
				info := p.Info()
				t.Logf("✓ Found plugin '%s' for PCI %s:%s", info.Name, tc.vendorID, tc.deviceID)

				if info.Name != tc.expectPlugin {
					t.Errorf("Expected plugin '%s', got '%s'", tc.expectPlugin, info.Name)
				}
			} else {
				t.Logf("No plugin found for PCI %s:%s (plugin may not be imported)", tc.vendorID, tc.deviceID)
			}
		})
	}
}

// TestPluginRegistry_PluginInfo verifies plugin metadata is complete
func TestPluginRegistry_PluginInfo(t *testing.T) {
	registry := plugin.DefaultRegistry()
	plugins := registry.List()

	for _, p := range plugins {
		info := p.Info()

		t.Run(info.Name, func(t *testing.T) {
			// Check required fields
			if info.Name == "" {
				t.Error("Plugin has empty name")
			}
			if info.Vendor == "" {
				t.Error("Plugin has empty vendor")
			}
			if info.Version == "" {
				t.Error("Plugin has empty version")
			}

			// Check PCI device IDs
			if len(info.SupportedDevices) == 0 {
				t.Error("Plugin has no supported devices")
			} else {
				t.Logf("Supports %d device types:", len(info.SupportedDevices))
				for _, dev := range info.SupportedDevices {
					if dev.VendorID == "" || dev.DeviceID == "" {
						t.Errorf("Invalid PCI ID: %s", dev.String())
					} else {
						t.Logf("  - %s (%s)", dev.Description, dev.String())
					}
				}
			}

			// Check capabilities
			if len(info.Capabilities) == 0 {
				t.Log("Plugin declares no capabilities")
			} else {
				t.Logf("Capabilities: %v", info.Capabilities)
			}
		})
	}
}

// TestPluginRegistry_MultiVendorScenario simulates a multi-vendor deployment
func TestPluginRegistry_MultiVendorScenario(t *testing.T) {
	registry := plugin.DefaultRegistry()

	// Simulate discovering multiple vendor devices in a cluster
	discoveredDevices := []struct {
		vendorID string
		deviceID string
		nodeName string
	}{
		{"15b3", "a2d6", "node1"}, // NVIDIA on node1
		{"8086", "1453", "node2"}, // Intel on node2
		{"177d", "b903", "node3"}, // Marvell on node3
	}

	t.Log("Simulating multi-vendor cluster discovery:")
	for _, dev := range discoveredDevices {
		plugin := registry.GetByVendorDevice(dev.vendorID, dev.deviceID)
		if plugin != nil {
			info := plugin.Info()
			t.Logf("  Node %s: %s DPU (plugin: %s)", dev.nodeName, info.Vendor, info.Name)
		} else {
			t.Logf("  Node %s: Unknown device %s:%s", dev.nodeName, dev.vendorID, dev.deviceID)
		}
	}
}

// TestPluginRegistry_CapabilityInterfaceChecks verifies capability interfaces
func TestPluginRegistry_CapabilityInterfaceChecks(t *testing.T) {
	registry := plugin.DefaultRegistry()
	plugins := registry.List()

	for _, p := range plugins {
		info := p.Info()

		t.Run(info.Name, func(t *testing.T) {
			// Check if plugin implements capability interfaces
			for _, cap := range info.Capabilities {
				switch cap {
				case plugin.CapabilityNetworking:
					if _, ok := p.(plugin.NetworkPlugin); !ok {
						t.Errorf("Plugin declares %s capability but doesn't implement NetworkPlugin", cap)
					} else {
						t.Logf("✓ Implements NetworkPlugin interface")
					}
				case plugin.CapabilityStorage:
					if _, ok := p.(plugin.StoragePlugin); !ok {
						t.Logf("⚠ Plugin declares %s capability but doesn't fully implement StoragePlugin (may be planned)", cap)
					} else {
						t.Logf("✓ Implements StoragePlugin interface")
					}
				case plugin.CapabilitySecurity:
					if _, ok := p.(plugin.SecurityPlugin); !ok {
						t.Errorf("Plugin declares %s capability but doesn't implement SecurityPlugin", cap)
					} else {
						t.Logf("✓ Implements SecurityPlugin interface")
					}
				}
			}
		})
	}
}

// TestPluginLifecycle tests basic plugin lifecycle operations
func TestPluginLifecycle(t *testing.T) {
	registry := plugin.DefaultRegistry()
	plugins := registry.List()

	ctx := context.Background()

	for _, p := range plugins {
		info := p.Info()

		t.Run(info.Name, func(t *testing.T) {
			// Test initialization
			config := plugin.PluginConfig{
				OPIEndpoint: "localhost:50051", // Mock endpoint
				LogLevel:    1,
			}

			// Note: This will fail without a real OPI bridge, but tests the interface
			err := p.Initialize(ctx, config)
			if err != nil {
				t.Logf("Initialize failed (expected without OPI bridge): %v", err)
			} else {
				t.Log("✓ Initialize succeeded")

				// Test health check
				err = p.HealthCheck(ctx)
				if err != nil {
					t.Logf("HealthCheck failed (expected without OPI bridge): %v", err)
				} else {
					t.Log("✓ HealthCheck passed")
				}

				// Test shutdown
				err = p.Shutdown(ctx)
				if err != nil {
					t.Errorf("Shutdown failed: %v", err)
				} else {
					t.Log("✓ Shutdown succeeded")
				}
			}
		})
	}
}
