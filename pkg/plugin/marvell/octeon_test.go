/*
Copyright 2024.
Licensed under the Apache License, Version 2.0 (the "License");
*/

package marvell

import (
	"context"
	"testing"

	"github.com/openshift/dpu-operator/pkg/plugin"
)

func TestOcteonPlugin_Info(t *testing.T) {
	p := New()
	info := p.Info()

	if info.Name != PluginName {
		t.Errorf("Expected name '%s', got %s", PluginName, info.Name)
	}
	if info.Vendor != PluginVendor {
		t.Errorf("Expected vendor '%s', got %s", PluginVendor, info.Vendor)
	}
}

func TestOcteonPlugin_Initialize(t *testing.T) {
	p := New()
	ctx := context.Background()
	err := p.Initialize(ctx, plugin.PluginConfig{})
	if err != nil {
		t.Errorf("Initialize should succeed: %v", err)
	}
}

func TestOcteonPlugin_DiscoverDevices(t *testing.T) {
	p := New()
	ctx := context.Background()
	_ = p.Initialize(ctx, plugin.PluginConfig{})
	devices, err := p.DiscoverDevices(ctx)
	if err != nil {
		t.Errorf("DiscoverDevices error: %v", err)
	}
	_ = devices
}

func TestOcteonPlugin_Shutdown(t *testing.T) {
	p := New()
	ctx := context.Background()
	_ = p.Initialize(ctx, plugin.PluginConfig{})
	err := p.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestOcteonPlugin_ImplementsPlugin(t *testing.T) {
	var _ plugin.Plugin = (*OcteonPlugin)(nil)
}

func TestOcteonPlugin_ImplementsNetworkPlugin(t *testing.T) {
	var _ plugin.NetworkPlugin = (*OcteonPlugin)(nil)
}
