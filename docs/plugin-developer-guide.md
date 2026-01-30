# Plugin Developer Guide

This guide explains how to add a new vendor plugin to the unified DPU operator.

## Overview

The unified DPU operator uses a plugin architecture that allows any vendor to
implement a standard interface and register with the operator. Each plugin is
responsible for:

1. **Device Discovery** - Scanning for DPU hardware on the system
2. **Inventory** - Reporting hardware details (serial number, firmware, etc.)
3. **Networking** - Managing bridge ports, VFs, and network functions
4. **Storage** (optional) - Managing NVMe subsystems, controllers, namespaces
5. **Security** (optional) - Managing IPsec tunnels

## Getting Started

### Step 1: Create the Plugin Package

Create a new package under `pkg/plugin/<vendor>/`:

```bash
mkdir -p pkg/plugin/<vendor>
```

### Step 2: Define PCI Device IDs

Identify the PCI vendor and device IDs for your hardware. You can find these using:

```bash
lspci -nn | grep -i <vendor>
```

### Step 3: Implement the Plugin Interface

Create a file `pkg/plugin/<vendor>/<device>.go` implementing the `Plugin` interface:

```go
package vendorname

import (
    "context"
    "sync"

    "github.com/go-logr/logr"
    "github.com/openshift/dpu-operator/pkg/plugin"
    ctrl "sigs.k8s.io/controller-runtime"
)

const (
    PluginName    = "vendorname"
    PluginVendor  = "VendorName"
    PluginVersion = "1.0.0"
)

var supportedDevices = []plugin.PCIDeviceID{
    {VendorID: "1234", DeviceID: "5678", Description: "Device Model"},
}

type VendorPlugin struct {
    mu          sync.RWMutex
    log         logr.Logger
    config      plugin.PluginConfig
    initialized bool
    devices     []plugin.Device
}

func New() *VendorPlugin {
    return &VendorPlugin{
        log: ctrl.Log.WithName("plugin").WithName("vendorname"),
    }
}

// Info returns plugin metadata
func (p *VendorPlugin) Info() plugin.PluginInfo {
    return plugin.PluginInfo{
        Name:             PluginName,
        Vendor:           PluginVendor,
        Version:          PluginVersion,
        Description:      "Description of your plugin",
        SupportedDevices: supportedDevices,
        Capabilities: []plugin.Capability{
            plugin.CapabilityNetworking,
            // Add other capabilities as supported
        },
    }
}

// Initialize sets up the plugin
func (p *VendorPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.initialized {
        return plugin.ErrAlreadyInitialized
    }

    p.config = config
    // Initialize vendor SDK or OPI bridge connection here
    
    p.initialized = true
    return nil
}

// ... implement remaining interface methods
```

### Step 4: Register the Plugin

Add an `init()` function that registers the plugin with the global registry:

```go
func init() {
    plugin.MustRegister(New())
}
```

### Step 5: Implement Capability Interfaces

If your plugin supports networking, storage, or security offload, implement
the corresponding interfaces:

#### NetworkPlugin Interface

```go
// CreateBridgePort creates a bridge port
func (p *VendorPlugin) CreateBridgePort(ctx context.Context, req *plugin.BridgePortRequest) (*plugin.BridgePort, error) {
    // Implement using vendor SDK or OPI bridge
}

// DeleteBridgePort removes a bridge port
func (p *VendorPlugin) DeleteBridgePort(ctx context.Context, portID string) error {
    // Implement
}

// SetVFCount configures virtual functions
func (p *VendorPlugin) SetVFCount(ctx context.Context, deviceID string, count int) error {
    // Implement
}

// ... other NetworkPlugin methods
```

#### StoragePlugin Interface (Optional)

```go
// CreateNVMeSubsystem creates an NVMe subsystem
func (p *VendorPlugin) CreateNVMeSubsystem(ctx context.Context, req *plugin.NVMeSubsystemRequest) (*plugin.NVMeSubsystem, error) {
    // Implement using SPDK or vendor storage SDK
}

// ... other StoragePlugin methods
```

#### SecurityPlugin Interface (Optional)

```go
// CreateIPsecTunnel creates an IPsec tunnel
func (p *VendorPlugin) CreateIPsecTunnel(ctx context.Context, req *plugin.IPsecTunnelRequest) (*plugin.IPsecTunnel, error) {
    // Implement using strongSwan or vendor security SDK
}

// ... other SecurityPlugin methods
```

### Step 6: Add Unit Tests

Create `pkg/plugin/<vendor>/<device>_test.go` with comprehensive tests:

```go
package vendorname

import (
    "context"
    "testing"
    
    "github.com/openshift/dpu-operator/pkg/plugin"
)

func TestPluginInfo(t *testing.T) {
    p := New()
    info := p.Info()
    
    if info.Name != PluginName {
        t.Errorf("unexpected name: %s", info.Name)
    }
    // ... more assertions
}

func TestPluginInitialize(t *testing.T) {
    ctx := context.Background()
    p := New()
    
    if err := p.Initialize(ctx, plugin.PluginConfig{}); err != nil {
        t.Fatalf("Initialize failed: %v", err)
    }
    
    // Test double initialization
    if err := p.Initialize(ctx, plugin.PluginConfig{}); err != plugin.ErrAlreadyInitialized {
        t.Errorf("expected ErrAlreadyInitialized, got: %v", err)
    }
}

// ... more tests
```

### Step 7: Add Emulation Tests (Optional)

If an OPI bridge exists for your vendor (e.g., `opi-nvidia-bridge`), add
emulation tests that connect to the mock server:

```go
//go:build emulation

package vendorname

import (
    "context"
    "testing"
)

func TestPluginWithOPIBridge(t *testing.T) {
    // Connect to mock OPI server
    // Run tests against it
}
```

## Plugin Interface Reference

### Core Plugin Interface

```go
type Plugin interface {
    Info() PluginInfo
    Initialize(ctx context.Context, config PluginConfig) error
    Shutdown(ctx context.Context) error
    HealthCheck(ctx context.Context) error
    DiscoverDevices(ctx context.Context) ([]Device, error)
    GetInventory(ctx context.Context, deviceID string) (*InventoryResponse, error)
}
```

### PluginInfo Structure

| Field | Type | Description |
|-------|------|-------------|
| Name | string | Unique identifier (e.g., "nvidia", "intel") |
| Vendor | string | Human-readable vendor name |
| Version | string | Semantic version |
| Description | string | Plugin description |
| SupportedDevices | []PCIDeviceID | List of supported PCI devices |
| Capabilities | []Capability | Supported offload capabilities |

### Error Types

Use these standardized errors:

| Error | When to Use |
|-------|-------------|
| `ErrNotImplemented` | Optional operation not implemented |
| `ErrNotInitialized` | Plugin used before Initialize() |
| `ErrAlreadyInitialized` | Initialize() called twice |
| `ErrDeviceNotFound` | Device ID not found |
| `ErrResourceNotFound` | Port/tunnel/subsystem not found |

## OPI Integration

### Preferred: Use OPI Bridge

When an OPI bridge exists for your vendor, use it via gRPC:

```go
import (
    opi "github.com/opiproject/opi-api/network/evpn-gw/v1alpha1/gen/go"
    "google.golang.org/grpc"
)

func (p *VendorPlugin) Initialize(ctx context.Context, config plugin.PluginConfig) error {
    conn, err := grpc.DialContext(ctx, config.OPIEndpoint,
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return err
    }
    p.opiClient = opi.NewBridgePortServiceClient(conn)
    // ...
}
```

### Alternative: Direct SDK Integration

If no OPI bridge exists, you may use CGO to call the vendor SDK:

```go
// #cgo LDFLAGS: -lvendorsdk
// #include <vendor_sdk.h>
import "C"

func (p *VendorPlugin) DiscoverDevices(ctx context.Context) ([]plugin.Device, error) {
    devices := C.vendor_list_devices()
    // Convert C types to Go types
}
```

## Testing Guidelines

1. **Unit Tests**: Mock all external dependencies
2. **Integration Tests**: Test plugin registration and registry queries
3. **Emulation Tests**: Use OPI mock servers when available
4. **Hardware Tests**: Coordinate with vendor for lab access

## Checklist

Before submitting a new plugin:

- [ ] Implement all `Plugin` interface methods
- [ ] Implement relevant capability interfaces
- [ ] Register in `init()` function
- [ ] Add comprehensive unit tests
- [ ] Add to supported hardware matrix in README
- [ ] Document vendor-specific configuration options
- [ ] Add troubleshooting section
- [ ] Test on actual hardware (if available)

## Examples

See the existing plugin implementations for reference:

- `pkg/plugin/nvidia/`: NVIDIA BlueField (full networking + storage)
- `pkg/plugin/intel/`: Intel IPU (networking)
- `pkg/plugin/marvell/`: Marvell Octeon (networking)
- `pkg/plugin/xsight/`: xSight (networking)
