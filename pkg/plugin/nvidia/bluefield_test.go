package nvidia

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin"
)

func newTestPlugin(t *testing.T) *BlueFieldPlugin {
	t.Helper()
	return &BlueFieldPlugin{
		log: logr.Discard(),
	}
}

func TestBlueFieldPlugin_Info(t *testing.T) {
	p := newTestPlugin(t)
	info := p.Info()

	if info.Name != PluginName {
		t.Errorf("expected Name %q, got %q", PluginName, info.Name)
	}
	if info.Vendor != PluginVendor {
		t.Errorf("expected Vendor %q, got %q", PluginVendor, info.Vendor)
	}

	hasNetworking := false
	for _, c := range info.Capabilities {
		if c == plugin.CapabilityNetworking {
			hasNetworking = true
		}
	}
	if !hasNetworking {
		t.Error("expected CapabilityNetworking in Capabilities")
	}
	if len(info.SupportedDevices) == 0 {
		t.Error("expected at least one supported PCI device")
	}
}

func TestBlueFieldPlugin_InitializeAndShutdown(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()

	cfg := plugin.PluginConfig{
		OPIEndpoint: "localhost:50051",
	}

	if err := p.Initialize(ctx, cfg); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !p.initialized {
		t.Error("expected initialized to be true after Initialize")
	}

	if err := p.Initialize(ctx, cfg); err != plugin.ErrAlreadyInitialized {
		t.Errorf("expected ErrAlreadyInitialized on second Init, got: %v", err)
	}

	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if p.initialized {
		t.Error("expected initialized to be false after Shutdown")
	}
}

func TestBlueFieldPlugin_ShutdownWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	// Shutdown on an uninitialized plugin should be a no-op (idempotent)
	err := p.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected nil (idempotent shutdown), got: %v", err)
	}
}

func TestBlueFieldPlugin_HealthCheckWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	err := p.HealthCheck(context.Background())
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got: %v", err)
	}
}

func TestBlueFieldPlugin_HealthCheckNilOpiClient(t *testing.T) {
	p := newTestPlugin(t)
	p.initialized = true
	p.opiClient = nil
	// Should not panic; nil guard must protect the call
	err := p.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck with nil opiClient should succeed, got: %v", err)
	}
}

func TestBlueFieldPlugin_GetVFCountReturnsNotImplemented(t *testing.T) {
	p := newTestPlugin(t)
	p.initialized = true
	_, err := p.GetVFCount(context.Background(), "any-device")
	if err != plugin.ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented from GetVFCount, got: %v", err)
	}
}

func TestBlueFieldPlugin_CreateNetworkFunctionReturnsNotImplemented(t *testing.T) {
	p := newTestPlugin(t)
	p.initialized = true
	err := p.CreateNetworkFunction(context.Background(), "in", "out")
	if err != plugin.ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented from CreateNetworkFunction, got: %v", err)
	}
}

func TestBlueFieldPlugin_DeleteNetworkFunctionReturnsNotImplemented(t *testing.T) {
	p := newTestPlugin(t)
	p.initialized = true
	err := p.DeleteNetworkFunction(context.Background(), "in", "out")
	if err != plugin.ErrNotImplemented {
		t.Errorf("expected ErrNotImplemented from DeleteNetworkFunction, got: %v", err)
	}
}
