package intel

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin"
)

func newTestPlugin(t *testing.T) *IPUPlugin {
	t.Helper()
	return &IPUPlugin{
		log: logr.Discard(),
	}
}

func TestIPUPlugin_Info(t *testing.T) {
	p := newTestPlugin(t)
	info := p.Info()

	if info.Name != PluginName {
		t.Errorf("expected Name %q, got %q", PluginName, info.Name)
	}
	if info.Vendor != PluginVendor {
		t.Errorf("expected Vendor %q, got %q", PluginVendor, info.Vendor)
	}
	if len(info.Capabilities) == 0 {
		t.Error("expected at least one capability")
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

func TestIPUPlugin_InitializeAndShutdown(t *testing.T) {
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

	// Double-init must return ErrAlreadyInitialized
	if err := p.Initialize(ctx, cfg); !plugin.IsNotImplemented(err) && err != plugin.ErrAlreadyInitialized {
		t.Errorf("expected ErrAlreadyInitialized on second Init, got: %v", err)
	}

	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if p.initialized {
		t.Error("expected initialized to be false after Shutdown")
	}
}

func TestIPUPlugin_ShutdownWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	// Shutdown on an uninitialized plugin should be a no-op (idempotent)
	err := p.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected nil (idempotent shutdown), got: %v", err)
	}
}

func TestIPUPlugin_HealthCheckWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	err := p.HealthCheck(context.Background())
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got: %v", err)
	}
}

func TestIPUPlugin_DiscoverDevicesWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	_, err := p.DiscoverDevices(context.Background())
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got: %v", err)
	}
}
