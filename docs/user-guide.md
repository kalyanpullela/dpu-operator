# Unified DPU Operator User Guide

The Unified DPU Operator is a vendor-agnostic Kubernetes operator that manages
DPU (Data Processing Unit) and IPU (Infrastructure Processing Unit) devices
from multiple vendors through a standardized plugin architecture.

## Supported Hardware

| Vendor | Hardware | Status |
|--------|----------|--------|
| NVIDIA | BlueField-2 | ✅ Supported |
| NVIDIA | BlueField-3 | ✅ Supported |
| Intel | IPU E2100 | ✅ Supported |
| Intel | NetSec Accelerator (Senao SX904) | ✅ Supported |
| Marvell | Octeon 10 | ✅ Supported |
| xSight | DPU | 🔨 In Development |

## Prerequisites

- OpenShift 4.19+ or Kubernetes 1.28+
- DPU hardware installed in cluster nodes
- Node labels identifying DPU-enabled nodes

## Installation

### OpenShift (via OLM)

```bash
# Subscribe to the operator
cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: dpu-operator
  namespace: openshift-dpu-operator
spec:
  channel: stable
  name: dpu-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF
```

### Kubernetes (via kubectl)

```bash
# Install CRDs
kubectl apply -f config/crd/bases/

# Install operator
kubectl apply -f config/manager/
```

## Configuration

### DpuOperatorConfig

Create a `DpuOperatorConfig` to configure the operator:

```yaml
apiVersion: dpu.openshift.io/v2
kind: DpuOperatorConfig
metadata:
  name: default
spec:
  # Mode: "host" for running on host nodes, "dpu" for running on DPU nodes
  mode: host
  
  # Log verbosity (0-10)
  logLevel: 0
  
  # Generic (vendor-neutral) settings
  generic:
    # Network mode: switchdev, legacy, or offload
    networkMode: switchdev
    
    # Enable storage offload (NVMe-oF)
    storageOffloadEnabled: false
    
    # Enable security offload (IPsec)
    securityPolicyEnabled: false
    
    # Default VF count for all DPUs
    vfCount: 16
    
    # OPI gRPC endpoint
    opiEndpoint: "localhost:50051"
  
  # Vendor-specific configuration
  vendorConfigs:
    - vendor: nvidia
      enabled: true
      inline:
        docaVersion: "2.2"
```

Apply:

```bash
kubectl apply -f dpuoperatorconfig.yaml
```

### Vendor-Specific Configuration

Each vendor may have specific configuration options.

#### NVIDIA BlueField

```yaml
vendorConfigs:
  - vendor: nvidia
    enabled: true
    inline:
      # DOCA SDK path
      docaPath: /opt/mellanox/doca
      # Enable SNAP for NVMe emulation
      snapEnabled: true
      # Enable OVS-DOCA for hardware offload
      ovsDoca: true
```

#### Intel IPU

```yaml
vendorConfigs:
  - vendor: intel
    enabled: true
    inline:
      # P4 program to load
      p4Program: "default.p4"
      # Enable IMC (Infrastructure Management Complex)
      imcEnabled: true
```

#### Marvell Octeon

```yaml
vendorConfigs:
  - vendor: marvell
    enabled: true
    inline:
      # SDK version
      sdkVersion: "1.0"
```

## Discovering DPUs

Once configured, the operator automatically discovers DPU hardware and creates
`DataProcessingUnit` resources:

```bash
# List discovered DPUs
kubectl get dpu

# Get detailed information
kubectl get dpu -o wide

# Describe a specific DPU
kubectl describe dpu dpu-node1-bf2-0
```

Example output:

```
NAME                VENDOR   MODEL         NODE      PHASE   HEALTHY   AGE
dpu-node1-bf2-0     nvidia   BlueField-2   node1     Ready   true      5m
dpu-node1-bf2-1     nvidia   BlueField-2   node1     Ready   true      5m
dpu-node2-e2100-0   intel    IPU E2100     node2     Ready   true      5m
```

## Working with DPUs

### Viewing DPU Inventory

```bash
kubectl get dpu dpu-node1-bf2-0 -o jsonpath='{.status.inventory}' | jq
```

### Viewing DPU Health

```bash
kubectl get dpu dpu-node1-bf2-0 -o jsonpath='{.status.health}' | jq
```

### Configuring VF Count

To change the VF count for a specific DPU:

```yaml
apiVersion: dpu.openshift.io/v2
kind: DataProcessingUnit
metadata:
  name: dpu-node1-bf2-0
spec:
  vendor: nvidia
  model: BlueField-2
  nodeName: node1
  desiredVfCount: 32  # Request 32 VFs
```

### Per-DPU Configuration

Override cluster-wide settings for a specific DPU:

```yaml
apiVersion: dpu.openshift.io/v2
kind: DataProcessingUnit
metadata:
  name: dpu-node1-bf2-0
spec:
  vendor: nvidia
  model: BlueField-2
  nodeName: node1
  configuration:
    networkMode: offload  # Override to use full offload
    storageOffloadEnabled: true  # Enable storage for this DPU
```

## Network Offload

### Bridge Ports

The operator automatically configures bridge ports for SR-IOV-enabled pods.

### Flow Offload

With `networkMode: offload` or `networkMode: switchdev`, the operator
configures hardware flow offload for improved performance.

## Storage Offload (Optional)

When `storageOffloadEnabled: true`, the operator configures NVMe-oF offload:

1. Creates NVMe subsystems on the DPU
2. Exposes storage backends to host pods
3. Provides transparent storage access with hardware acceleration

## Security Offload (Optional)

When `securityPolicyEnabled: true`, the operator configures IPsec offload:

1. Creates IPsec tunnels with hardware acceleration
2. Offloads encryption/decryption to DPU
3. Provides transparent security for pod traffic

## Monitoring

### Prometheus Metrics

The operator exposes metrics at `/metrics`:

- `dpu_discovered_total`: Total DPUs discovered
- `dpu_ready_total`: DPUs in ready state
- `dpu_health_status`: Health status per DPU
- `dpu_temperature_celsius`: Temperature readings

### Grafana Dashboard

Import the included dashboard:

```bash
kubectl apply -f config/grafana/dpu-operator-dashboard.yaml
```

## Troubleshooting

### DPU Not Discovered

1. Check the node has DPU hardware:
   ```bash
   lspci -nn | grep -i 'nvidia\|intel\|marvell'
   ```

2. Check the daemon is running:
   ```bash
   kubectl get pods -n openshift-dpu-operator -l app=dpu-daemon
   ```

3. Check daemon logs:
   ```bash
   kubectl logs -n openshift-dpu-operator -l app=dpu-daemon
   ```

### DPU in Error State

1. Check DPU conditions:
   ```bash
   kubectl get dpu <name> -o jsonpath='{.status.conditions}'
   ```

2. Check plugin health:
   ```bash
   kubectl get dpuoperatorconfig default -o jsonpath='{.status.activePlugins}'
   ```

### Networking Issues

1. Check VFs are created:
   ```bash
   ls /sys/class/net/*/device/sriov_numvfs
   ```

2. Check representor ports:
   ```bash
   ip link show | grep rep
   ```

### Storage Issues

1. Check NVMe subsystems:
   ```bash
   nvme list
   ```

2. Check SPDK/SNAP status on DPU:
   ```bash
   rpc.py bdev_get_bdevs
   ```

## Best Practices

1. **Start with defaults**: Begin with basic configuration and enable features gradually

2. **Test in staging**: Validate configuration in a non-production environment

3. **Monitor health**: Set up alerts on DPU health metrics

4. **Document vendor config**: Keep track of vendor-specific settings

5. **Version control**: Store DpuOperatorConfig in Git

## API Reference

See the [API documentation](./api-reference.md) for complete CRD reference.

## Plugin Developer Guide

For adding new vendor support, see the [Plugin Developer Guide](./plugin-developer-guide.md).
