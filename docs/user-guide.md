# Unified DPU Operator User Guide

The Unified DPU Operator is a vendor-agnostic Kubernetes operator that manages
DPU (Data Processing Unit) and IPU (Infrastructure Processing Unit) devices
from multiple vendors through a standardized plugin architecture.

## Supported Hardware

| Vendor | Hardware | Status |
|--------|----------|--------|
| NVIDIA | BlueField-2 | ✅ Supported (networking via OPI bridge; storage/security planned) |
| NVIDIA | BlueField-3 | ✅ Supported (networking via OPI bridge; storage/security planned) |
| Intel | IPU E2100 | ✅ Supported |
| Intel | NetSec Accelerator (Senao SX904) | ✅ Supported |
| Marvell | Octeon 10 | ✅ Supported |
| xSight | DPU | 🔶 Experimental (discovery/inventory only) |
| MangoBoost | DPU | 🔶 Experimental (discovery/inventory only) |

**Note**: NVIDIA BlueField support requires the NVIDIA VSP image (`nvidia_bf`) to be configured
and an `opi-nvidia-bridge` endpoint reachable from the operator/daemon. Storage and security
offloads are planned but not yet integrated.

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

# Install operator (recommended: applies namespace and patches from config/default)
kubectl apply -k config/default
```

**Note**: `config/default` deploys into `openshift-dpu-operator` by default. If you deploy
the operator into a different namespace, ensure the `POD_NAMESPACE` environment variable
is set to that namespace so the operator and daemon render resources consistently. The
`config/manager/` manifests are a lower-level base that deploys into `system` by default.

## Configuration

### DpuOperatorConfig

Create a `DpuOperatorConfig` to configure the operator:

```yaml
apiVersion: config.openshift.io/v1
kind: DpuOperatorConfig
metadata:
  name: dpu-operator-config
spec:
  # Log verbosity (0-10)
  logLevel: 0
```

Apply:

```bash
kubectl apply -f dpuoperatorconfig.yaml
```

**Note**: The name must be `dpu-operator-config` as enforced by the validating webhook.

### Vendor-Specific Configuration

Vendor-specific configuration is managed through environment variables and ConfigMaps.
See the vendor-specific plugin documentation for details on configuration options.

For registry plugins (hybrid runtime), you can configure OPI bridge endpoints via:
- `DPU_PLUGIN_OPI_ENDPOINT` (global default)
- `DPU_PLUGIN_OPI_ENDPOINT_<VENDOR>` (vendor-specific override, e.g. `DPU_PLUGIN_OPI_ENDPOINT_NVIDIA`)
- `DPU_PLUGIN_LOG_LEVEL` (optional integer log level)

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
NAME                DPU PRODUCT       DPU SIDE   NODE NAME   STATUS
dpu-node1-bf2-0     NVIDIA BlueField  false      node1       True
dpu-node1-bf2-1     NVIDIA BlueField  false      node1       True
dpu-node2-e2100-0   Intel IPU E2100   false      node2       True
```

## Working with DPUs

### Viewing DPU Status

The v1 DPU CRD currently exposes readiness via conditions. Inventory and
health fields are not yet populated in the CRD status.

```bash
kubectl get dpu dpu-node1-bf2-0 -o jsonpath='{.status.conditions}' | jq
```

### DPU Configuration

DataProcessingUnit resources are automatically created by the operator when DPUs are discovered.
The spec contains the following fields:

```yaml
apiVersion: config.openshift.io/v1
kind: DataProcessingUnit
metadata:
  name: dpu-node1-bf2-0
spec:
  # Product name from detection
  dpuProductName: "NVIDIA BlueField"
  # Whether this is the DPU-side (true) or host-side (false)
  isDpuSide: false
  # Node where the DPU is located
  nodeName: node1
```

These resources are typically managed by the operator and should not be manually edited.

### DataProcessingUnitConfig (VF Count)

Use `DataProcessingUnitConfig` to apply VF count overrides to matching DPUs:

```yaml
apiVersion: config.openshift.io/v1
kind: DataProcessingUnitConfig
metadata:
  name: dpuconfig-vfcount
spec:
  dpuSelector:
    matchLabels:
      dpu: "enabled"
  vfCount: 8
```

The controller writes a config-specific annotation (`dpu.config.openshift.io/vf-count/<configName>`).
If multiple configs apply different VF counts to the same DPU, the daemon will log a conflict and
skip applying the change until the conflict is resolved.

### ServiceFunctionChain

ServiceFunctionChain deploys network function pods with optional Multus networks and DPU resources:

```yaml
apiVersion: config.openshift.io/v1
kind: ServiceFunctionChain
metadata:
  name: sfc-sample
spec:
  nodeSelector:
    dpu: "true"
  networkFunctions:
    - name: firewall
      image: nginx:latest
      networks:
        - dpunfcni-conf
        - dpunfcni-conf
      dpuResources:
        requests: 2
        limits: 2
```

## DPU Features

The operator manages DPU hardware discovery, health monitoring, and integration with
Kubernetes. Specific features and capabilities depend on the DPU vendor and model:

- **Network Offload**: SR-IOV VF management and hardware flow offload
- **Storage Offload**: Planned (not yet integrated)
- **Security Offload**: Planned (not yet integrated)

Refer to vendor plugin documentation for specific feature availability and configuration.

## Monitoring

### Prometheus Metrics

The operator exposes metrics at `/metrics` on the secure metrics port (default `10443`).
Key metrics include:

- `dpu_operator_plugins_registered_total`
- `dpu_operator_devices_discovered_total`
- `dpu_operator_device_health_status`
- `dpu_operator_reconciliation_duration_seconds`
- `dpu_operator_reconciliation_errors_total`
- `dpu_operator_opi_bridge_latency_seconds`
- `dpu_operator_opi_bridge_errors_total`

## Troubleshooting

### DPU Not Discovered

1. Check the node has DPU hardware:
   ```bash
   lspci -nn | grep -i 'nvidia\|intel\|marvell'
   ```

2. Check the daemon is running:
   ```bash
   kubectl get pods -n <operator-namespace> -l app=dpu-daemon
   ```

3. Check daemon logs:
   ```bash
   kubectl logs -n <operator-namespace> -l app=dpu-daemon
   ```

### DPU in Error State

1. Check DPU conditions:
   ```bash
   kubectl get dpu <name> -o jsonpath='{.status.conditions}'
   ```

2. Check operator status:
   ```bash
   kubectl get dpuoperatorconfig dpu-operator-config -o jsonpath='{.status.conditions}'
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

Storage offload is not yet integrated in the v1 operator. If you are
experimenting with vendor-specific storage paths, consult the vendor
documentation and bridge logs.

## Best Practices

1. **Start with defaults**: Begin with basic configuration and enable features gradually

2. **Test in staging**: Validate configuration in a non-production environment

3. **Monitor health**: Set up alerts on DPU condition transitions and reconciliation error metrics

4. **Document vendor config**: Keep track of vendor-specific settings

5. **Version control**: Store DpuOperatorConfig in Git

## API Reference

For CRD schemas, see the generated manifests in `config/crd/bases/`.

## Plugin Developer Guide

For adding new vendor support, see the [Plugin Developer Guide](./plugin-developer-guide.md).
