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
	"fmt"
	"sync"
	"testing"
)

// BenchmarkPluginRegistry_Lookup benchmarks plugin lookup operations by name.
func BenchmarkPluginRegistry_Lookup(b *testing.B) {
	tests := []struct {
		name        string
		pluginCount int
	}{
		{"Small_10plugins", 10},
		{"Medium_50plugins", 50},
		{"Large_100plugins", 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup: Create registry with N plugins
			r := NewRegistry()
			for i := 0; i < tt.pluginCount; i++ {
				plugin := NewMockPlugin(
					fmt.Sprintf("vendor-%d", i),
					fmt.Sprintf("Vendor%d", i),
					[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
					[]Capability{CapabilityNetworking},
				)
				if err := r.Register(plugin); err != nil {
					b.Fatalf("failed to register plugin: %v", err)
				}
			}

			// Benchmark lookup by name
			targetName := fmt.Sprintf("vendor-%d", tt.pluginCount/2) // Middle plugin

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugin := r.Get(targetName)
				if plugin == nil {
					b.Fatal("expected to find plugin")
				}
			}
		})
	}
}

// BenchmarkPluginRegistry_LookupByPCIID benchmarks plugin lookup by PCI device ID.
func BenchmarkPluginRegistry_LookupByPCIID(b *testing.B) {
	tests := []struct {
		name        string
		pluginCount int
		devicesPerPlugin int
	}{
		{"Small_10plugins_1device", 10, 1},
		{"Medium_50plugins_5devices", 50, 5},
		{"Large_100plugins_10devices", 100, 10},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup: Create registry with N plugins, each supporting M devices
			r := NewRegistry()
			for i := 0; i < tt.pluginCount; i++ {
				devices := make([]PCIDeviceID, tt.devicesPerPlugin)
				for j := 0; j < tt.devicesPerPlugin; j++ {
					devices[j] = PCIDeviceID{
						VendorID: fmt.Sprintf("%04x", i),
						DeviceID: fmt.Sprintf("%04x", j),
					}
				}

				plugin := NewMockPlugin(
					fmt.Sprintf("vendor-%d", i),
					fmt.Sprintf("Vendor%d", i),
					devices,
					[]Capability{CapabilityNetworking},
				)
				if err := r.Register(plugin); err != nil {
					b.Fatalf("failed to register plugin: %v", err)
				}
			}

			// Benchmark lookup by PCI ID (middle plugin, middle device)
			targetVendor := fmt.Sprintf("%04x", tt.pluginCount/2)
			targetDevice := fmt.Sprintf("%04x", tt.devicesPerPlugin/2)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugin := r.GetByVendorDevice(targetVendor, targetDevice)
				if plugin == nil {
					b.Fatal("expected to find plugin")
				}
			}
		})
	}
}

// BenchmarkPluginInitialization benchmarks plugin initialization for different vendors.
func BenchmarkPluginInitialization(b *testing.B) {
	vendors := []struct {
		name    string
		devices []PCIDeviceID
		caps    []Capability
	}{
		{
			name: "NVIDIA_BlueField",
			devices: []PCIDeviceID{
				{VendorID: "15b3", DeviceID: "a2d6", Description: "BlueField-2"},
				{VendorID: "15b3", DeviceID: "a2dc", Description: "BlueField-3"},
			},
			caps: []Capability{CapabilityNetworking, CapabilityStorage},
		},
		{
			name: "Intel_IPU",
			devices: []PCIDeviceID{
				{VendorID: "8086", DeviceID: "1452", Description: "IPU E2100"},
			},
			caps: []Capability{CapabilityNetworking, CapabilitySecurity},
		},
		{
			name: "Marvell_Octeon",
			devices: []PCIDeviceID{
				{VendorID: "177d", DeviceID: "a0f8", Description: "Octeon DPU"},
			},
			caps: []Capability{CapabilityNetworking, CapabilityStorage, CapabilitySecurity},
		},
		{
			name: "MangoBoost_XSight",
			devices: []PCIDeviceID{
				{VendorID: "1edb", DeviceID: "0001", Description: "XSight DPU"},
			},
			caps: []Capability{CapabilityNetworking, CapabilityAIML},
		},
	}

	for _, vendor := range vendors {
		b.Run(vendor.name, func(b *testing.B) {
			ctx := context.Background()
			config := PluginConfig{
				OPIEndpoint: "localhost:50051",
				LogLevel:    0,
				VendorConfig: map[string]interface{}{
					"timeout": 30,
				},
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugin := NewMockPlugin(vendor.name, vendor.name, vendor.devices, vendor.caps)

				// Initialize
				if err := plugin.Initialize(ctx, config); err != nil {
					b.Fatalf("failed to initialize plugin: %v", err)
				}

				// Shutdown
				if err := plugin.Shutdown(ctx); err != nil {
					b.Fatalf("failed to shutdown plugin: %v", err)
				}
			}
		})
	}
}

// BenchmarkDeviceDiscovery benchmarks device discovery operations.
func BenchmarkDeviceDiscovery(b *testing.B) {
	tests := []struct {
		name        string
		deviceCount int
	}{
		{"SingleDevice", 1},
		{"SmallCluster_4devices", 4},
		{"MediumCluster_16devices", 16},
		{"LargeCluster_64devices", 64},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ctx := context.Background()

			// Setup: Create plugin with mock devices
			plugin := NewMockPlugin(
				"test-vendor",
				"TestVendor",
				[]PCIDeviceID{{VendorID: "15b3", DeviceID: "a2d6"}},
				[]Capability{CapabilityNetworking},
			)

			// Populate mock devices
			devices := make([]Device, tt.deviceCount)
			for i := 0; i < tt.deviceCount; i++ {
				devices[i] = Device{
					ID:              fmt.Sprintf("device-%d", i),
					PCIAddress:      fmt.Sprintf("0000:%02x:00.0", i),
					PCIID:           PCIDeviceID{VendorID: "15b3", DeviceID: "a2d6"},
					Vendor:          "TestVendor",
					Model:           "TestDPU",
					SerialNumber:    fmt.Sprintf("SN%08d", i),
					FirmwareVersion: "1.0.0",
					Healthy:         true,
					Metadata: map[string]string{
						"numa_node": fmt.Sprintf("%d", i%2),
					},
				}
			}
			plugin.discoverDevices = devices

			// Initialize plugin
			if err := plugin.Initialize(ctx, PluginConfig{}); err != nil {
				b.Fatalf("failed to initialize: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				devs, err := plugin.DiscoverDevices(ctx)
				if err != nil {
					b.Fatalf("discovery failed: %v", err)
				}
				if len(devs) != tt.deviceCount {
					b.Fatalf("expected %d devices, got %d", tt.deviceCount, len(devs))
				}
			}
		})
	}
}

// BenchmarkOPIClientCreation benchmarks OPI client creation overhead.
// This simulates the cost of establishing gRPC connections.
func BenchmarkOPIClientCreation(b *testing.B) {
	tests := []struct {
		name     string
		endpoint string
	}{
		{"LocalEndpoint", "localhost:50051"},
		{"RemoteEndpoint", "192.168.1.100:50051"},
		{"UnixSocket", "unix:///var/run/opi.sock"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugin := NewMockPlugin(
					"opi-test",
					"OPITest",
					[]PCIDeviceID{{VendorID: "15b3", DeviceID: "a2d6"}},
					[]Capability{CapabilityNetworking},
				)

				config := PluginConfig{
					OPIEndpoint: tt.endpoint,
					LogLevel:    0,
				}

				// Simulate client creation in Initialize
				if err := plugin.Initialize(ctx, config); err != nil {
					b.Fatalf("initialize failed: %v", err)
				}

				// Cleanup
				_ = plugin.Shutdown(ctx)
			}
		})
	}
}

// BenchmarkConcurrentPluginAccess benchmarks thread-safety and concurrent access.
func BenchmarkConcurrentPluginAccess(b *testing.B) {
	tests := []struct {
		name        string
		goroutines  int
		pluginCount int
	}{
		{"Serial_1goroutine_10plugins", 1, 10},
		{"Parallel_10goroutines_10plugins", 10, 10},
		{"Parallel_50goroutines_50plugins", 50, 50},
		{"Parallel_100goroutines_100plugins", 100, 100},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup: Create registry with plugins
			r := NewRegistry()
			for i := 0; i < tt.pluginCount; i++ {
				plugin := NewMockPlugin(
					fmt.Sprintf("vendor-%d", i),
					fmt.Sprintf("Vendor%d", i),
					[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
					[]Capability{CapabilityNetworking},
				)
				if err := r.Register(plugin); err != nil {
					b.Fatalf("failed to register plugin: %v", err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					// Mix of operations to simulate real-world access patterns
					pluginIdx := b.N % tt.pluginCount

					// 1. Get by name (40% of operations)
					if b.N%10 < 4 {
						name := fmt.Sprintf("vendor-%d", pluginIdx)
						_ = r.Get(name)
					}

					// 2. Get by PCI ID (30% of operations)
					if b.N%10 >= 4 && b.N%10 < 7 {
						vendorID := fmt.Sprintf("%04x", pluginIdx)
						_ = r.GetByVendorDevice(vendorID, "0001")
					}

					// 3. List all plugins (20% of operations)
					if b.N%10 >= 7 && b.N%10 < 9 {
						_ = r.List()
					}

					// 4. Get by capability (10% of operations)
					if b.N%10 >= 9 {
						_ = r.GetByCapability(CapabilityNetworking)
					}
				}
			})
		})
	}
}

// BenchmarkRegistryList benchmarks listing all registered plugins.
func BenchmarkRegistryList(b *testing.B) {
	tests := []struct {
		name        string
		pluginCount int
	}{
		{"Small_10plugins", 10},
		{"Medium_50plugins", 50},
		{"Large_100plugins", 100},
		{"VeryLarge_500plugins", 500},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			// Setup
			r := NewRegistry()
			for i := 0; i < tt.pluginCount; i++ {
				plugin := NewMockPlugin(
					fmt.Sprintf("vendor-%d", i),
					fmt.Sprintf("Vendor%d", i),
					[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
					[]Capability{CapabilityNetworking},
				)
				if err := r.Register(plugin); err != nil {
					b.Fatalf("failed to register plugin: %v", err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugins := r.List()
				if len(plugins) != tt.pluginCount {
					b.Fatalf("expected %d plugins, got %d", tt.pluginCount, len(plugins))
				}
			}
		})
	}
}

// BenchmarkGetByCapability benchmarks filtering plugins by capability.
func BenchmarkGetByCapability(b *testing.B) {
	capabilities := []Capability{
		CapabilityNetworking,
		CapabilityStorage,
		CapabilitySecurity,
		CapabilityAIML,
	}

	// Setup: Create registry with mixed capabilities
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		// Assign capabilities in a pattern
		caps := []Capability{capabilities[i%4]}
		if i%10 == 0 {
			// Some plugins have multiple capabilities
			caps = append(caps, capabilities[(i+1)%4])
		}

		plugin := NewMockPlugin(
			fmt.Sprintf("vendor-%d", i),
			fmt.Sprintf("Vendor%d", i),
			[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
			caps,
		)
		if err := r.Register(plugin); err != nil {
			b.Fatalf("failed to register plugin: %v", err)
		}
	}

	for _, cap := range capabilities {
		b.Run(string(cap), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				plugins := r.GetByCapability(cap)
				if len(plugins) == 0 {
					b.Fatal("expected to find plugins with capability")
				}
			}
		})
	}
}

// BenchmarkGetNetworkPlugins benchmarks type assertion for NetworkPlugin interface.
func BenchmarkGetNetworkPlugins(b *testing.B) {
	// Setup: Mix of regular plugins and network plugins
	r := NewRegistry()
	for i := 0; i < 50; i++ {
		plugin := NewMockPlugin(
			fmt.Sprintf("vendor-%d", i),
			fmt.Sprintf("Vendor%d", i),
			[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
			[]Capability{CapabilityNetworking},
		)
		if err := r.Register(plugin); err != nil {
			b.Fatalf("failed to register plugin: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// This will attempt type assertions on all plugins
		plugins := r.GetNetworkPlugins()
		_ = plugins
	}
}

// BenchmarkPluginChecker benchmarks the PluginChecker helper.
func BenchmarkPluginChecker(b *testing.B) {
	plugin := NewMockPlugin(
		"test",
		"Test",
		[]PCIDeviceID{{VendorID: "15b3", DeviceID: "a2d6"}},
		[]Capability{CapabilityNetworking, CapabilityStorage, CapabilitySecurity},
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		checker := NewPluginChecker(plugin)

		// Check all capabilities
		_ = checker.SupportsCapability(CapabilityNetworking)
		_ = checker.SupportsCapability(CapabilityStorage)
		_ = checker.SupportsCapability(CapabilitySecurity)
		_ = checker.SupportsCapability(CapabilityAIML)

		// Type checks
		_ = checker.IsNetworkPlugin()
		_ = checker.IsStoragePlugin()
		_ = checker.IsSecurityPlugin()
	}
}

// BenchmarkRegistryRegisterUnregister benchmarks registration churn.
func BenchmarkRegistryRegisterUnregister(b *testing.B) {
	r := NewRegistry()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("vendor-%d", i)
		plugin := NewMockPlugin(
			name,
			"Vendor",
			[]PCIDeviceID{{VendorID: "15b3", DeviceID: "a2d6"}},
			[]Capability{CapabilityNetworking},
		)

		// Register
		if err := r.Register(plugin); err != nil {
			b.Fatalf("failed to register: %v", err)
		}

		// Unregister
		if err := r.Unregister(name); err != nil {
			b.Fatalf("failed to unregister: %v", err)
		}
	}
}

// BenchmarkInventoryRetrieval benchmarks getting device inventory.
func BenchmarkInventoryRetrieval(b *testing.B) {
	ctx := context.Background()
	plugin := NewMockPlugin(
		"test",
		"Test",
		[]PCIDeviceID{{VendorID: "15b3", DeviceID: "a2d6"}},
		[]Capability{CapabilityNetworking},
	)

	// Setup mock inventory
	plugin.inventory = &InventoryResponse{
		DeviceID:    "device-0",
		BIOSVersion: "1.0.0",
		BMCVersion:  "2.0.0",
		Chassis: &ChassisInfo{
			Manufacturer: "TestVendor",
			Model:        "TestDPU",
			SerialNumber: "SN12345678",
		},
		CPU: &CPUInfo{
			Model:        "ARM Cortex-A72",
			CoreCount:    8,
			ThreadCount:  8,
			FrequencyMHz: 2000,
		},
		Memory: &MemoryInfo{
			TotalBytes: 16 * 1024 * 1024 * 1024, // 16GB
			Type:       "DDR4",
		},
		NetworkInterfaces: []NetworkInterface{
			{Name: "eth0", MACAddress: "00:11:22:33:44:55", SpeedMbps: 100000, LinkUp: true},
			{Name: "eth1", MACAddress: "00:11:22:33:44:56", SpeedMbps: 100000, LinkUp: true},
		},
		StorageDevices: []StorageDevice{
			{Name: "nvme0", Model: "Samsung 980 PRO", CapacityBytes: 1024 * 1024 * 1024 * 1024, Type: "NVMe"},
		},
	}

	if err := plugin.Initialize(ctx, PluginConfig{}); err != nil {
		b.Fatalf("failed to initialize: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		inv, err := plugin.GetInventory(ctx, "device-0")
		if err != nil {
			b.Fatalf("failed to get inventory: %v", err)
		}
		if inv == nil {
			b.Fatal("expected inventory")
		}
	}
}

// BenchmarkConcurrentPluginOperations simulates realistic concurrent workload.
func BenchmarkConcurrentPluginOperations(b *testing.B) {
	ctx := context.Background()
	r := NewRegistry()

	// Setup: Register multiple plugins
	pluginCount := 10
	for i := 0; i < pluginCount; i++ {
		plugin := NewMockPlugin(
			fmt.Sprintf("vendor-%d", i),
			fmt.Sprintf("Vendor%d", i),
			[]PCIDeviceID{{VendorID: fmt.Sprintf("%04x", i), DeviceID: "0001"}},
			[]Capability{CapabilityNetworking},
		)

		// Populate devices
		devices := []Device{
			{
				ID:              fmt.Sprintf("device-%d-0", i),
				PCIAddress:      fmt.Sprintf("0000:%02x:00.0", i),
				Vendor:          plugin.info.Vendor,
				Model:           "TestDPU",
				SerialNumber:    fmt.Sprintf("SN%08d", i),
				FirmwareVersion: "1.0.0",
				Healthy:         true,
			},
		}
		plugin.discoverDevices = devices

		if err := r.Register(plugin); err != nil {
			b.Fatalf("failed to register: %v", err)
		}

		if err := plugin.Initialize(ctx, PluginConfig{}); err != nil {
			b.Fatalf("failed to initialize: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	workers := 10

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < b.N/workers; i++ {
				pluginIdx := i % pluginCount

				// Lookup plugin
				plugin := r.Get(fmt.Sprintf("vendor-%d", pluginIdx))
				if plugin == nil {
					b.Errorf("worker %d: plugin not found", workerID)
					return
				}

				// Health check
				_ = plugin.HealthCheck(ctx)

				// Device discovery
				_, _ = plugin.DiscoverDevices(ctx)

				// Inventory retrieval
				_, _ = plugin.GetInventory(ctx, fmt.Sprintf("device-%d-0", pluginIdx))
			}
		}(w)
	}

	wg.Wait()
}
