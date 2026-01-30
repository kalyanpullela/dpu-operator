# Migration Guide: v1 to v2 CRDs

This guide helps users migrate from the v1 DPU Operator CRDs to the new v2 API.

## Overview

The v2 API introduces several improvements:

1. **Clean separation** between vendor-neutral and vendor-specific configuration
2. **Extension points** for vendor-specific parameters
3. **Richer status reporting** with plugin information and health data
4. **Capability declarations** on discovered DPUs

## CRD Changes

### DpuOperatorConfig

#### v1 Schema

```yaml
apiVersion: dpu.openshift.io/v1
kind: DpuOperatorConfig
metadata:
  name: default
spec:
  logLevel: 0
```

#### v2 Schema

```yaml
apiVersion: dpu.openshift.io/v2
kind: DpuOperatorConfig
metadata:
  name: default
spec:
  mode: host  # or "dpu"
  logLevel: 0
  
  generic:
    networkMode: switchdev  # switchdev, legacy, or offload
    storageOffloadEnabled: false
    securityPolicyEnabled: false
    vfCount: 16
    opiEndpoint: "localhost:50051"
    resourcePrefix: "openshift.io"
  
  vendorConfigs:
    - vendor: nvidia
      enabled: true
      inline:
        docaPath: /opt/mellanox/doca
        snapEnabled: true
    - vendor: intel
      enabled: true
      configMapRef:
        name: intel-ipu-config
  
  pluginSelector:
    names: []
    vendors: []
    capabilities: []
```

### Key Differences

| v1 Field | v2 Equivalent | Notes |
|----------|---------------|-------|
| `spec.logLevel` | `spec.logLevel` | No change |
| (implicit host mode) | `spec.mode` | Now explicit |
| (none) | `spec.generic.networkMode` | New field |
| (none) | `spec.generic.storageOffloadEnabled` | New field |
| (none) | `spec.generic.securityPolicyEnabled` | New field |
| (none) | `spec.vendorConfigs[]` | New extension point |

### DataProcessingUnit (DPU)

#### v1 Schema

```yaml
apiVersion: dpu.openshift.io/v1
kind: DataProcessingUnit
metadata:
  name: dpu-001
spec:
  dpuProductName: "BlueField-2"
  isDpuSide: false
  nodeName: worker-1
```

#### v2 Schema

```yaml
apiVersion: dpu.openshift.io/v2
kind: DataProcessingUnit
metadata:
  name: dpu-001
spec:
  vendor: nvidia
  model: BlueField-2
  pciAddress: "0000:03:00.0"
  pciDeviceId: "15b3:a2d6"
  nodeName: worker-1
  isDpuSide: false
  capabilities:
    - networking
    - storage
  desiredVfCount: 16
  configuration:
    networkMode: switchdev
status:
  phase: Ready
  currentVfCount: 16
  pluginName: nvidia
  inventory:
    serialNumber: "MT12345"
    firmwareVersion: "24.35.1000"
    biosVersion: "1.2.3"
    cpuModel: "ARM Cortex-A78"
    cpuCores: 16
    memoryTotalBytes: 17179869184
    networkPorts:
      - name: p0
        macAddress: "00:11:22:33:44:55"
        speedMbps: 100000
        linkUp: true
      - name: p1
        macAddress: "00:11:22:33:44:56"
        speedMbps: 100000
        linkUp: true
  health:
    healthy: true
    temperatureCelsius: 45
    powerWatts: 75.5
```

### Key Differences

| v1 Field | v2 Equivalent | Notes |
|----------|---------------|-------|
| `spec.dpuProductName` | `spec.model` | Renamed |
| (none) | `spec.vendor` | Now explicit |
| (none) | `spec.pciAddress` | New field |
| (none) | `spec.pciDeviceId` | New field |
| `spec.isDpuSide` | `spec.isDpuSide` | No change |
| `spec.nodeName` | `spec.nodeName` | No change |
| (none) | `spec.capabilities` | New field |
| `status.conditions` | `status.conditions` + `status.phase` | Added phase |
| (none) | `status.inventory` | Rich inventory |
| (none) | `status.health` | Health monitoring |

## Conversion Webhooks

The operator supports both v1 and v2 CRDs simultaneously via conversion webhooks.

### How Conversion Works

1. When a v1 CR is created, the conversion webhook translates it to v2 internally
2. The controller always works with v2 types
3. When reading back a v1 CR, it's converted from v2

### Conversion Rules

#### DpuOperatorConfig v1 → v2

| v1 Field | v2 Field | Conversion |
|----------|----------|------------|
| `spec.logLevel` | `spec.logLevel` | Direct copy |
| (none) | `spec.mode` | Default: `host` |
| (none) | `spec.generic.networkMode` | Default: `switchdev` |
| (none) | `spec.vendorConfigs` | Empty list |

#### DpuOperatorConfig v2 → v1

| v2 Field | v1 Field | Conversion |
|----------|----------|------------|
| `spec.logLevel` | `spec.logLevel` | Direct copy |
| `spec.mode` | (lost) | Not representable |
| `spec.generic.*` | (lost) | Not representable |
| `spec.vendorConfigs` | (lost) | Not representable |

> **Warning**: Converting from v2 to v1 loses information. Avoid mixing API versions.

#### DataProcessingUnit v1 → v2

| v1 Field | v2 Field | Conversion |
|----------|----------|------------|
| `spec.dpuProductName` | `spec.model` | Direct copy |
| `spec.isDpuSide` | `spec.isDpuSide` | Direct copy |
| `spec.nodeName` | `spec.nodeName` | Direct copy |
| (none) | `spec.vendor` | Inferred from model name |

## Migration Steps

### Step 1: Verify Operator Version

Ensure you're running an operator version that supports v2:

```bash
kubectl get deployment dpu-operator-controller-manager -n openshift-dpu-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
```

### Step 2: Backup Existing Resources

```bash
kubectl get dpuoperatorconfig -o yaml > dpuoperatorconfig-backup.yaml
kubectl get dpu -o yaml > dpu-backup.yaml
```

### Step 3: Convert DpuOperatorConfig

Create a new v2 DpuOperatorConfig:

```yaml
apiVersion: dpu.openshift.io/v2
kind: DpuOperatorConfig
metadata:
  name: default
spec:
  mode: host
  logLevel: 0  # Carry over from v1
  generic:
    networkMode: switchdev
    storageOffloadEnabled: false
    securityPolicyEnabled: false
  # Add vendor-specific config if needed
  vendorConfigs:
    - vendor: nvidia
      enabled: true
```

Apply:

```bash
kubectl apply -f dpuoperatorconfig-v2.yaml
```

### Step 4: DataProcessingUnit (Automatic)

DPU CRs are automatically recreated by the operator during device discovery.
No manual migration is needed. The new CRs will have richer status information.

### Step 5: Verify

Check that DPUs are discovered with new schema:

```bash
kubectl get dpu -o wide
```

## FAQ

### Can I use v1 and v2 CRDs together?

Yes, conversion webhooks allow both versions to work. However, we recommend
migrating to v2 for full functionality.

### What happens to my v1 CRs after migration?

v1 CRs continue to work but are converted to v2 internally. Status updates
reflected back to v1 CRs may have less detail than v2.

### How do I access v2-only features?

Create resources using `apiVersion: dpu.openshift.io/v2`.

### When will v1 be deprecated?

v1 will be deprecated 2 releases after v2 GA. A deprecation warning will be
added to the operator logs.

## Troubleshooting

### Webhook Connection Errors

If you see webhook connection errors:

```bash
# Check webhook is running
kubectl get pods -n openshift-dpu-operator -l app=webhook

# Check webhook certificate
kubectl get secret -n openshift-dpu-operator webhook-server-cert
```

### Conversion Failures

If conversion fails:

```bash
# Check operator logs
kubectl logs -n openshift-dpu-operator deployment/dpu-operator-controller-manager

# Look for conversion errors
kubectl logs -n openshift-dpu-operator deployment/dpu-operator-controller-manager | grep -i conversion
```

### Schema Validation Errors

If you get validation errors with v2:

```bash
# Validate your YAML against the CRD
kubectl apply --dry-run=server -f your-config.yaml
```
