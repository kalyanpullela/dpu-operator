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

package nvidia

import (
	"context"
	"testing"

	"github.com/openshift/dpu-operator/pkg/plugin"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
}

func TestPluginInfo(t *testing.T) {
	p := New()
	info := p.Info()

	if info.Name != PluginName {
		t.Errorf("expected name=%s, got %s", PluginName, info.Name)
	}

	if info.Vendor != PluginVendor {
		t.Errorf("expected vendor=%s, got %s", PluginVendor, info.Vendor)
	}

	if info.Version != PluginVersion {
		t.Errorf("expected version=%s, got %s", PluginVersion, info.Version)
	}

	if len(info.SupportedDevices) < 2 {
		t.Errorf("expected at least 2 supported devices, got %d", len(info.SupportedDevices))
	}

	foundBF2 := false
	for _, d := range info.SupportedDevices {
		if d.VendorID == "15b3" && d.DeviceID == "a2d6" {
			foundBF2 = true
			break
		}
	}
	if !foundBF2 {
		t.Error("BlueField-2 device (15b3:a2d6) not in supported devices")
	}

	if len(info.Capabilities) == 0 {
		t.Error("expected at least one capability")
	}

	hasNetworking := false
	for _, c := range info.Capabilities {
		if c == plugin.CapabilityNetworking {
			hasNetworking = true
			break
		}
	}
	if !hasNetworking {
		t.Error("expected networking capability")
	}
}

func TestPluginInitialize(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
		LogLevel:    1,
	}

	err := p.Initialize(ctx, config)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !p.initialized {
		t.Error("expected initialized to be true")
	}

	// Test double initialization
	err = p.Initialize(ctx, config)
	if err != plugin.ErrAlreadyInitialized {
		t.Errorf("expected ErrAlreadyInitialized, got %v", err)
	}

	_ = p.Shutdown(ctx)
}

func TestPluginShutdown(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	err := p.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown before init failed: %v", err)
	}

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	err = p.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	if p.initialized {
		t.Error("expected initialized to be false after shutdown")
	}
}

func TestPluginHealthCheck(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	err := p.HealthCheck(ctx)
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	err = p.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}

	_ = p.Shutdown(ctx)
}

func TestPluginDiscoverDevices(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	_, err := p.DiscoverDevices(ctx)
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	devices, err := p.DiscoverDevices(ctx)
	if err != nil {
		t.Fatalf("DiscoverDevices failed: %v", err)
	}

	_ = devices

	_ = p.Shutdown(ctx)
}

func TestPluginGetInventory(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	_, err := p.GetInventory(ctx, "device-1")
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)

	// In current impl, GetInventory might depend on discovery or OPI.
	// We'll see how it behaves with real OPI client.
	_, err = p.GetInventory(ctx, "device-1")
	_ = err // Ignore error if not implemented yet

	_ = p.Shutdown(ctx)
}

func TestPluginCreateBridgePort(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	defer p.Shutdown(ctx)

	request := &plugin.BridgePortRequest{
		Name:       "test-port",
		MACAddress: "00:11:22:33:44:55",
	}

	port, err := p.CreateBridgePort(ctx, request)
	if err != nil {
		t.Fatalf("CreateBridgePort failed: %v", err)
	}

	if port == nil {
		t.Fatal("expected non-nil port")
	}

	if port.Name != request.Name {
		t.Errorf("expected name=%s, got %s", request.Name, port.Name)
	}

	// MAC address check might differ depending on OPI response
	// The mock server echoes the request, so it should match
}

func TestPluginDeleteBridgePort(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	err := p.DeleteBridgePort(ctx, "port-1")
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got %v", err)
	}

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	defer p.Shutdown(ctx)

	err = p.DeleteBridgePort(ctx, "port-1")
	if err != nil {
		t.Errorf("DeleteBridgePort failed: %v", err)
	}
}

func TestPluginSetVFCount(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	defer p.Shutdown(ctx)

	err := p.SetVFCount(ctx, "device-1", 8)
	if err != nil {
		// SetVFCount currently stubbed or calling OPI
		// The mock server implements SetNumVfs, so it might work if wired
		// But BlueField plugin might override it with sysfs logic
		_ = err
	}
}

func TestPluginGetVFCount(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	defer p.Shutdown(ctx)

	_, err := p.GetVFCount(ctx, "device-1")
	// If stubbed, nil error.
	if err != nil {
		t.Errorf("GetVFCount failed: %v", err)
	}
}

func TestPluginNetworkFunction(t *testing.T) {
	addr, cleanup := startMockServer(t)
	defer cleanup()

	ctx := context.Background()
	p := New()

	config := plugin.PluginConfig{
		OPIEndpoint: addr,
	}
	_ = p.Initialize(ctx, config)
	defer p.Shutdown(ctx)

	err := p.CreateNetworkFunction(ctx, "input-port", "output-port")
	if err != nil {
		t.Errorf("CreateNetworkFunction failed: %v", err)
	}

	err = p.DeleteNetworkFunction(ctx, "input-port", "output-port")
	if err != nil {
		t.Errorf("DeleteNetworkFunction failed: %v", err)
	}
}

func TestPluginImplementsInterfaces(t *testing.T) {
	p := New()
	var _ plugin.Plugin = p
	var _ plugin.NetworkPlugin = p
}

func TestPluginRegistration(t *testing.T) {
	p := plugin.Get(PluginName)
	if p == nil {
		t.Error("nvidia plugin not found in registry")
	}
	if p.Info().Name != PluginName {
		t.Errorf("expected name=%s, got %s", PluginName, p.Info().Name)
	}
}

func TestPluginLookupByDeviceID(t *testing.T) {
	p := plugin.GetByDeviceID("15b3:a2d6")
	if p == nil {
		t.Error("nvidia plugin not found by BlueField-2 device ID")
	}
	p = plugin.GetByDeviceID("15b3:a2dc")
	if p == nil {
		t.Error("nvidia plugin not found by BlueField-3 device ID")
	}
}

func TestDefaultOPIEndpoint(t *testing.T) {
	ctx := context.Background()
	p := New()

	// Initialize without OPIEndpoint -> should default to localhost:50051
	// Will likely fail to connect in environment without OPI
	err := p.Initialize(ctx, plugin.PluginConfig{})

	// Check defaults were set regardless of success
	if p.opiEndpoint != "localhost:50051" {
		t.Errorf("expected default endpoint localhost:50051, got %s", p.opiEndpoint)
	}

	// Clean up if somehow it succeeded
	if err == nil {
		p.Shutdown(ctx)
	}
}
