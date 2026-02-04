package mangoboost

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openshift/dpu-operator/pkg/plugin"
)

func newTestPlugin(t *testing.T) *MangoBoostPlugin {
	t.Helper()
	return &MangoBoostPlugin{
		log: logr.Discard(),
	}
}

func TestMangoBoostPlugin_Info(t *testing.T) {
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
}

func TestMangoBoostPlugin_InitializeAndShutdown(t *testing.T) {
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

func TestMangoBoostPlugin_ShutdownWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	// Shutdown on an uninitialized plugin should be a no-op (idempotent)
	err := p.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected nil (idempotent shutdown), got: %v", err)
	}
}

func TestMangoBoostPlugin_HealthCheckWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	err := p.HealthCheck(context.Background())
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got: %v", err)
	}
}

func TestMangoBoostPlugin_DiscoverDevicesWithoutInit(t *testing.T) {
	p := newTestPlugin(t)
	_, err := p.DiscoverDevices(context.Background())
	if err != plugin.ErrNotInitialized {
		t.Errorf("expected ErrNotInitialized, got: %v", err)
	}
}
