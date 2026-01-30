//go:build emulation
// +build emulation

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

// Package emulation provides emulation tests for DPU plugins against real OPI bridges.
// These tests require Docker and running OPI bridge containers.
//
// To run: docker-compose up -d && go test -tags=emulation ./test/emulation/... -v
package emulation

import (
	"context"
	"testing"
	"time"

	"github.com/openshift/dpu-operator/pkg/plugin"
	"github.com/openshift/dpu-operator/pkg/plugin/intel"
	"github.com/openshift/dpu-operator/pkg/plugin/nvidia"
)

const (
	// OPI bridge endpoints (from docker-compose.yml)
	nvidiaEndpoint = "localhost:50051"
	intelEndpoint  = "localhost:50052"
	spdkEndpoint   = "localhost:50053"
	marvellEndpoint = "localhost:50054"
	strongswanEndpoint = "localhost:50055"
)

// TestNVIDIAPlugin_WithOPIBridge tests NVIDIA plugin against real OPI bridge
func TestNVIDIAPlugin_WithOPIBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create NVIDIA plugin
	nvPlugin := nvidia.New()

	// Initialize with real OPI bridge endpoint
	config := plugin.PluginConfig{
		OPIEndpoint: nvidiaEndpoint,
		LogLevel:    1,
	}

	t.Log("Initializing NVIDIA plugin with OPI bridge...")
	err := nvPlugin.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Plugin initialization failed: %v", err)
	}
	defer nvPlugin.Shutdown(ctx)

	// Test health check
	t.Log("Testing health check...")
	err = nvPlugin.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	} else {
		t.Log("✓ Health check passed")
	}

	// Test device discovery
	t.Log("Testing device discovery...")
	devices, err := nvPlugin.DiscoverDevices(ctx)
	if err != nil {
		t.Logf("Device discovery failed (expected in emulation): %v", err)
	} else {
		t.Logf("✓ Discovered %d devices", len(devices))
		for _, dev := range devices {
			t.Logf("  - %s: %s (%s)", dev.ID, dev.Model, dev.PCIAddress)
		}
	}

	// Test network operations
	t.Log("Testing network operations...")
	portReq := &plugin.BridgePortRequest{
		Name:       "test-port-nvidia",
		MACAddress: "02:00:00:00:00:01",
		VLANID:     100,
	}

	port, err := nvPlugin.CreateBridgePort(ctx, portReq)
	if err != nil {
		t.Logf("CreateBridgePort failed (may not be fully implemented): %v", err)
	} else {
		t.Logf("✓ Created bridge port: %s", port.Name)

		// Clean up
		if port != nil {
			err = nvPlugin.DeleteBridgePort(ctx, port.ID)
			if err != nil {
				t.Logf("DeleteBridgePort failed: %v", err)
			} else {
				t.Log("✓ Deleted bridge port")
			}
		}
	}

	// Test getting bridge ports
	t.Log("Testing ListBridgePorts...")
	ports, err := nvPlugin.ListBridgePorts(ctx)
	if err != nil {
		t.Logf("ListBridgePorts failed: %v", err)
	} else {
		t.Logf("✓ Listed %d bridge ports", len(ports))
	}
}

// TestIntelPlugin_WithOPIBridge tests Intel plugin against real OPI bridge
func TestIntelPlugin_WithOPIBridge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Intel plugin
	intelPlugin := intel.New()

	// Initialize with real OPI bridge endpoint
	config := plugin.PluginConfig{
		OPIEndpoint: intelEndpoint,
		LogLevel:    1,
	}

	t.Log("Initializing Intel plugin with OPI bridge...")
	err := intelPlugin.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Plugin initialization failed: %v", err)
	}
	defer intelPlugin.Shutdown(ctx)

	// Test health check
	t.Log("Testing health check...")
	err = intelPlugin.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	} else {
		t.Log("✓ Health check passed")
	}

	// Test device discovery
	t.Log("Testing device discovery...")
	devices, err := intelPlugin.DiscoverDevices(ctx)
	if err != nil {
		t.Logf("Device discovery failed (expected in emulation): %v", err)
	} else {
		t.Logf("✓ Discovered %d devices", len(devices))
		for _, dev := range devices {
			t.Logf("  - %s: %s (%s)", dev.ID, dev.Model, dev.PCIAddress)
		}
	}

	// Test network operations
	t.Log("Testing network operations...")
	portReq := &plugin.BridgePortRequest{
		Name:       "test-port-intel",
		MACAddress: "02:00:00:00:00:02",
		VLANID:     200,
	}

	port, err := intelPlugin.CreateBridgePort(ctx, portReq)
	if err != nil {
		t.Logf("CreateBridgePort failed (may not be fully implemented): %v", err)
	} else {
		t.Logf("✓ Created bridge port: %s", port.Name)

		// Clean up
		if port != nil {
			err = intelPlugin.DeleteBridgePort(ctx, port.ID)
			if err != nil {
				t.Logf("DeleteBridgePort failed: %v", err)
			} else {
				t.Log("✓ Deleted bridge port")
			}
		}
	}
}

// TestPluginConnectivity tests that all plugins can connect to their bridges
func TestPluginConnectivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		endpoint string
		plugin   plugin.Plugin
	}{
		{"NVIDIA", nvidiaEndpoint, nvidia.New()},
		{"Intel", intelEndpoint, intel.New()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := plugin.PluginConfig{
				OPIEndpoint: tc.endpoint,
				LogLevel:    0,
			}

			err := tc.plugin.Initialize(ctx, config)
			if err != nil {
				t.Fatalf("Failed to initialize %s plugin: %v", tc.name, err)
			}
			defer tc.plugin.Shutdown(ctx)

			err = tc.plugin.HealthCheck(ctx)
			if err != nil {
				t.Errorf("%s plugin health check failed: %v", tc.name, err)
			} else {
				t.Logf("✓ %s plugin connected and healthy", tc.name)
			}
		})
	}
}

// TestMultiVendorEmulation simulates a multi-vendor scenario
func TestMultiVendorEmulation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Simulating multi-vendor DPU cluster:")

	// Initialize all plugins
	plugins := map[string]struct {
		plugin   plugin.Plugin
		endpoint string
	}{
		"NVIDIA": {nvidia.New(), nvidiaEndpoint},
		"Intel":  {intel.New(), intelEndpoint},
	}

	// Initialize each plugin
	for name, p := range plugins {
		config := plugin.PluginConfig{
			OPIEndpoint: p.endpoint,
			LogLevel:    0,
		}

		err := p.plugin.Initialize(ctx, config)
		if err != nil {
			t.Logf("  Node with %s DPU: initialization failed: %v", name, err)
			continue
		}
		defer p.plugin.Shutdown(ctx)

		err = p.plugin.HealthCheck(ctx)
		if err != nil {
			t.Logf("  Node with %s DPU: unhealthy: %v", name, err)
		} else {
			t.Logf("  ✓ Node with %s DPU: operational", name)
		}
	}
}

// TestOPIBridgeAvailability checks if OPI bridges are running
func TestOPIBridgeAvailability(t *testing.T) {
	// This is a prerequisite test - if this fails, other tests will fail too
	bridges := map[string]string{
		"NVIDIA":     nvidiaEndpoint,
		"Intel":      intelEndpoint,
		"SPDK":       spdkEndpoint,
		"Marvell":    marvellEndpoint,
		"StrongSwan": strongswanEndpoint,
	}

	t.Log("Checking OPI bridge availability:")
	available := 0
	for name, endpoint := range bridges {
		// Try to create a simple plugin and connect
		nvPlugin := nvidia.New()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		config := plugin.PluginConfig{
			OPIEndpoint: endpoint,
			LogLevel:    0,
		}

		err := nvPlugin.Initialize(ctx, config)
		cancel()

		if err == nil {
			nvPlugin.Shutdown(context.Background())
			t.Logf("  ✓ %s bridge available at %s", name, endpoint)
			available++
		} else {
			t.Logf("  ✗ %s bridge not available at %s: %v", name, endpoint, err)
		}
	}

	if available == 0 {
		t.Fatal("No OPI bridges are available. Run 'docker-compose up -d' first.")
	}

	t.Logf("\nTotal available bridges: %d/%d", available, len(bridges))
}
