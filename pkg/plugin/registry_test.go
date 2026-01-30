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
	"testing"
)

// --- Mock Plugin for Testing ---

// MockPlugin is a simple implementation of Plugin for testing.
type MockPlugin struct {
	info            PluginInfo
	initError       error
	shutdownError   error
	healthError     error
	discoverDevices []Device
	discoverError   error
	inventory       *InventoryResponse
	inventoryError  error
	initialized     bool
}

func NewMockPlugin(name, vendor string, devices []PCIDeviceID, caps []Capability) *MockPlugin {
	return &MockPlugin{
		info: PluginInfo{
			Name:             name,
			Vendor:           vendor,
			Version:          "1.0.0",
			Description:      "Mock plugin for testing",
			SupportedDevices: devices,
			Capabilities:     caps,
		},
	}
}

func (m *MockPlugin) Info() PluginInfo {
	return m.info
}

func (m *MockPlugin) Initialize(ctx context.Context, config PluginConfig) error {
	if m.initError != nil {
		return m.initError
	}
	m.initialized = true
	return nil
}

func (m *MockPlugin) Shutdown(ctx context.Context) error {
	m.initialized = false
	return m.shutdownError
}

func (m *MockPlugin) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func (m *MockPlugin) DiscoverDevices(ctx context.Context) ([]Device, error) {
	if m.discoverError != nil {
		return nil, m.discoverError
	}
	return m.discoverDevices, nil
}

func (m *MockPlugin) GetInventory(ctx context.Context, deviceID string) (*InventoryResponse, error) {
	if m.inventoryError != nil {
		return nil, m.inventoryError
	}
	return m.inventory, nil
}

// --- Registry Tests ---

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("expected empty registry, got count=%d", r.Count())
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	plugin := NewMockPlugin("test", "TestVendor", []PCIDeviceID{
		{VendorID: "1234", DeviceID: "5678", Description: "Test Device"},
	}, []Capability{CapabilityNetworking})

	err := r.Register(plugin)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("expected count=1, got %d", r.Count())
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	plugin1 := NewMockPlugin("test", "Vendor1", nil, nil)
	plugin2 := NewMockPlugin("test", "Vendor2", nil, nil)

	if err := r.Register(plugin1); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	err := r.Register(plugin2)
	if err == nil {
		t.Error("expected error when registering duplicate plugin name")
	}
}

func TestRegistryRegisterNil(t *testing.T) {
	r := NewRegistry()
	err := r.Register(nil)
	if err == nil {
		t.Error("expected error when registering nil plugin")
	}
}

func TestRegistryRegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	plugin := &MockPlugin{
		info: PluginInfo{Name: ""}, // Empty name
	}
	err := r.Register(plugin)
	if err == nil {
		t.Error("expected error when registering plugin with empty name")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()

	plugin := NewMockPlugin("nvidia", "NVIDIA", nil, nil)
	_ = r.Register(plugin)

	got := r.Get("nvidia")
	if got == nil {
		t.Error("expected to get plugin, got nil")
	}
	if got.Info().Name != "nvidia" {
		t.Errorf("expected name=nvidia, got %s", got.Info().Name)
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()
	got := r.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent plugin")
	}
}

func TestRegistryGetByDeviceID(t *testing.T) {
	r := NewRegistry()

	plugin := NewMockPlugin("nvidia", "NVIDIA", []PCIDeviceID{
		{VendorID: "15b3", DeviceID: "a2d6", Description: "BlueField-2"},
		{VendorID: "15b3", DeviceID: "a2dc", Description: "BlueField-3"},
	}, nil)
	_ = r.Register(plugin)

	tests := []struct {
		deviceID string
		wantName string
	}{
		{"15b3:a2d6", "nvidia"},
		{"15B3:A2D6", "nvidia"}, // Case insensitive
		{"15b3:a2dc", "nvidia"},
		{"1234:5678", ""},
	}

	for _, tt := range tests {
		t.Run(tt.deviceID, func(t *testing.T) {
			got := r.GetByDeviceID(tt.deviceID)
			if tt.wantName == "" {
				if got != nil {
					t.Errorf("expected nil, got plugin %s", got.Info().Name)
				}
			} else {
				if got == nil {
					t.Errorf("expected plugin %s, got nil", tt.wantName)
				} else if got.Info().Name != tt.wantName {
					t.Errorf("expected %s, got %s", tt.wantName, got.Info().Name)
				}
			}
		})
	}
}

func TestRegistryGetByVendorDevice(t *testing.T) {
	r := NewRegistry()

	plugin := NewMockPlugin("intel", "Intel", []PCIDeviceID{
		{VendorID: "8086", DeviceID: "1234", Description: "IPU E2100"},
	}, nil)
	_ = r.Register(plugin)

	got := r.GetByVendorDevice("8086", "1234")
	if got == nil {
		t.Error("expected plugin, got nil")
	}
	if got.Info().Name != "intel" {
		t.Errorf("expected intel, got %s", got.Info().Name)
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()

	plugin := NewMockPlugin("test", "Test", []PCIDeviceID{
		{VendorID: "1234", DeviceID: "5678"},
	}, nil)
	_ = r.Register(plugin)

	if r.Count() != 1 {
		t.Fatal("expected count=1 after register")
	}

	err := r.Unregister("test")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("expected count=0 after unregister, got %d", r.Count())
	}

	// Verify device index is cleaned up
	got := r.GetByDeviceID("1234:5678")
	if got != nil {
		t.Error("expected nil after unregister, device index not cleaned")
	}
}

func TestRegistryUnregisterNotFound(t *testing.T) {
	r := NewRegistry()
	err := r.Unregister("nonexistent")
	if err == nil {
		t.Error("expected error when unregistering nonexistent plugin")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	plugins := []*MockPlugin{
		NewMockPlugin("nvidia", "NVIDIA", nil, nil),
		NewMockPlugin("intel", "Intel", nil, nil),
		NewMockPlugin("marvell", "Marvell", nil, nil),
	}

	for _, p := range plugins {
		_ = r.Register(p)
	}

	list := r.List()
	if len(list) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(list))
	}
}

func TestRegistryListNames(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(NewMockPlugin("alpha", "", nil, nil))
	_ = r.Register(NewMockPlugin("beta", "", nil, nil))

	names := r.ListNames()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}

	// Check both names are present (order not guaranteed)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Error("expected alpha and beta in names")
	}
}

func TestRegistryGetByCapability(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(NewMockPlugin("net-only", "V1", nil, []Capability{CapabilityNetworking}))
	_ = r.Register(NewMockPlugin("storage-only", "V2", nil, []Capability{CapabilityStorage}))
	_ = r.Register(NewMockPlugin("multi-cap", "V3", nil, []Capability{CapabilityNetworking, CapabilityStorage}))

	netPlugins := r.GetByCapability(CapabilityNetworking)
	if len(netPlugins) != 2 {
		t.Errorf("expected 2 networking plugins, got %d", len(netPlugins))
	}

	storagePlugins := r.GetByCapability(CapabilityStorage)
	if len(storagePlugins) != 2 {
		t.Errorf("expected 2 storage plugins, got %d", len(storagePlugins))
	}

	securityPlugins := r.GetByCapability(CapabilitySecurity)
	if len(securityPlugins) != 0 {
		t.Errorf("expected 0 security plugins, got %d", len(securityPlugins))
	}
}

func TestRegistryClear(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(NewMockPlugin("test1", "", nil, nil))
	_ = r.Register(NewMockPlugin("test2", "", nil, nil))

	if r.Count() != 2 {
		t.Fatalf("expected count=2, got %d", r.Count())
	}

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("expected count=0 after clear, got %d", r.Count())
	}
}

func TestFindPluginForDevice(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(NewMockPlugin("nvidia", "NVIDIA", []PCIDeviceID{
		{VendorID: "15b3", DeviceID: "a2d6"},
	}, nil))

	plugin, found := r.FindPluginForDevice("15b3", "a2d6")
	if !found {
		t.Error("expected to find plugin")
	}
	if plugin.Info().Name != "nvidia" {
		t.Errorf("expected nvidia, got %s", plugin.Info().Name)
	}

	_, found = r.FindPluginForDevice("1234", "5678")
	if found {
		t.Error("expected not to find plugin for unknown device")
	}
}

// --- PCIDeviceID Tests ---

func TestPCIDeviceIDString(t *testing.T) {
	pci := PCIDeviceID{VendorID: "15b3", DeviceID: "a2d6"}
	got := pci.String()
	want := "15b3:a2d6"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

// --- PluginChecker Tests ---

// MockNetworkPlugin implements both Plugin and NetworkPlugin for testing.
type MockNetworkPlugin struct {
	MockPlugin
}

func (m *MockNetworkPlugin) CreateBridgePort(ctx context.Context, request *BridgePortRequest) (*BridgePort, error) {
	return nil, nil
}

func (m *MockNetworkPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
	return nil
}

func (m *MockNetworkPlugin) GetBridgePort(ctx context.Context, portID string) (*BridgePort, error) {
	return nil, nil
}

func (m *MockNetworkPlugin) ListBridgePorts(ctx context.Context) ([]*BridgePort, error) {
	return nil, nil
}

func (m *MockNetworkPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
	return nil
}

func (m *MockNetworkPlugin) GetVFCount(ctx context.Context, deviceID string) (int, error) {
	return 0, nil
}

func (m *MockNetworkPlugin) CreateNetworkFunction(ctx context.Context, input, output string) error {
	return nil
}

func (m *MockNetworkPlugin) DeleteNetworkFunction(ctx context.Context, input, output string) error {
	return nil
}

func TestPluginCheckerNetworkPlugin(t *testing.T) {
	// Test with a plugin that implements NetworkPlugin
	netPlugin := &MockNetworkPlugin{
		MockPlugin: MockPlugin{
			info: PluginInfo{
				Name:         "net-test",
				Capabilities: []Capability{CapabilityNetworking},
			},
		},
	}

	checker := NewPluginChecker(netPlugin)

	if !checker.IsNetworkPlugin() {
		t.Error("expected IsNetworkPlugin to return true")
	}

	if checker.AsNetworkPlugin() == nil {
		t.Error("expected AsNetworkPlugin to return non-nil")
	}

	if checker.IsStoragePlugin() {
		t.Error("expected IsStoragePlugin to return false")
	}

	if checker.AsStoragePlugin() != nil {
		t.Error("expected AsStoragePlugin to return nil")
	}
}

func TestPluginCheckerSupportsCapability(t *testing.T) {
	plugin := NewMockPlugin("test", "Test", nil, []Capability{
		CapabilityNetworking,
		CapabilityStorage,
	})

	checker := NewPluginChecker(plugin)

	if !checker.SupportsCapability(CapabilityNetworking) {
		t.Error("expected networking capability to be supported")
	}

	if !checker.SupportsCapability(CapabilityStorage) {
		t.Error("expected storage capability to be supported")
	}

	if checker.SupportsCapability(CapabilitySecurity) {
		t.Error("expected security capability to not be supported")
	}
}

// --- Concurrent Access Tests ---

func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()

	// Pre-register some plugins
	for i := 0; i < 10; i++ {
		p := NewMockPlugin(
			"plugin-"+string(rune('a'+i)),
			"Vendor",
			[]PCIDeviceID{{VendorID: "1234", DeviceID: "000" + string(rune('0'+i))}},
			nil,
		)
		_ = r.Register(p)
	}

	// Run concurrent reads
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			_ = r.List()
			_ = r.Get("plugin-a")
			_ = r.GetByDeviceID("1234:0001")
			_ = r.Count()
			_ = r.ListNames()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

// --- Default Registry Tests ---

func TestDefaultRegistry(t *testing.T) {
	// Clear default registry first
	defaultRegistry.Clear()

	plugin := NewMockPlugin("default-test", "Test", nil, nil)
	err := Register(plugin)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got := Get("default-test")
	if got == nil {
		t.Error("expected to get plugin from default registry")
	}

	plugins := List()
	if len(plugins) < 1 {
		t.Error("expected at least 1 plugin in default registry")
	}

	// Cleanup
	_ = Unregister("default-test")
}

func TestMustRegisterPanics(t *testing.T) {
	defaultRegistry.Clear()

	// Register first plugin normally
	plugin1 := NewMockPlugin("panic-test", "Test", nil, nil)
	MustRegister(plugin1)

	// Second registration with same name should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustRegister to panic on duplicate registration")
		}
	}()

	plugin2 := NewMockPlugin("panic-test", "Test2", nil, nil)
	MustRegister(plugin2)
}

// --- Plugin Interface Tests ---

func TestMockPluginImplementation(t *testing.T) {
	ctx := context.Background()

	plugin := NewMockPlugin("test", "Vendor", nil, []Capability{CapabilityNetworking})
	plugin.discoverDevices = []Device{
		{ID: "dev1", Model: "Test Device"},
	}
	plugin.inventory = &InventoryResponse{
		DeviceID: "dev1",
		Chassis:  &ChassisInfo{Model: "Test"},
	}

	// Test Info
	info := plugin.Info()
	if info.Name != "test" {
		t.Errorf("expected name=test, got %s", info.Name)
	}

	// Test Initialize
	err := plugin.Initialize(ctx, PluginConfig{})
	if err != nil {
		t.Errorf("Initialize failed: %v", err)
	}
	if !plugin.initialized {
		t.Error("expected plugin to be initialized")
	}

	// Test HealthCheck
	if err := plugin.HealthCheck(ctx); err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}

	// Test DiscoverDevices
	devices, err := plugin.DiscoverDevices(ctx)
	if err != nil {
		t.Errorf("DiscoverDevices failed: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}

	// Test GetInventory
	inv, err := plugin.GetInventory(ctx, "dev1")
	if err != nil {
		t.Errorf("GetInventory failed: %v", err)
	}
	if inv.DeviceID != "dev1" {
		t.Errorf("expected deviceID=dev1, got %s", inv.DeviceID)
	}

	// Test Shutdown
	err = plugin.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if plugin.initialized {
		t.Error("expected plugin to be shutdown")
	}
}
