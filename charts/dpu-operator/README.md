# DPU Operator Helm Chart

This Helm chart deploys the unified, vendor-agnostic DPU (Data Processing Unit) operator for Kubernetes.

## Prerequisites

- Kubernetes 1.28+ or OpenShift 4.19+
- Helm 3.8+
- cert-manager (optional, for webhook certificates). If `webhook.enabled=true` and cert-manager
  is disabled, you must provide a TLS secret named
  `<release-name>-webhook-server-cert` in the release namespace.

## Installing the Chart

### Add the Helm repository (when published)

```bash
helm repo add dpu-operator https://openshift.github.io/dpu-operator
helm repo update
```

### Install from local chart

```bash
helm install dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --create-namespace
```

**Note:** CRDs are packaged in the chart `crds/` directory and will be installed by Helm.
Use `--skip-crds` if you want to manage CRDs separately.

### Install with custom values

```bash
helm install dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --create-namespace \
  --set operator.logLevel=1 \
  --set plugins.nvidia.enabled=true \
  --set metrics.serviceMonitor.enabled=true
```

## Uninstalling the Chart

```bash
helm uninstall dpu-operator --namespace dpu-operator-system
```

## Configuration

The following table lists the configurable parameters and their default values.

### Global Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Operator container image repository | `ghcr.io/openshift/dpu-operator` |
| `image.tag` | Operator container image tag | `""` (uses appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Operator Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.replicas` | Number of operator replicas | `1` |
| `operator.logLevel` | Zap log level ("info", "debug", "error", or integer > 0) | `info` |
| `operator.leaderElection.enabled` | Enable leader election | `true` |

### Resource Limits

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |

### Security

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podSecurityContext.runAsNonRoot` | Run as non-root user | `true` |
| `podSecurityContext.runAsUser` | User ID to run as | `65532` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.readOnlyRootFilesystem` | Use read-only root filesystem | `true` |
| `networkPolicy.enabled` | Enable network policy | `false` |

### Metrics and Monitoring

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.port` | Metrics port | `10443` |
| `metrics.serviceMonitor.enabled` | Create Prometheus ServiceMonitor | `true` |
| `metrics.serviceMonitor.interval` | Scrape interval | `30s` |

### Health Probes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `healthProbe.port` | Health probe port for `/healthz` and `/readyz` | `8081` |

### Plugin Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `plugins.nvidia.enabled` | Enable NVIDIA BlueField plugin | `true` |
| `plugins.nvidia.opiEndpoint` | NVIDIA OPI bridge endpoint | `localhost:50051` |
| `plugins.nvidia.networkEndpoint` | NVIDIA EVPN-GW endpoint for networking | `localhost:50056` |
| `plugins.intel.enabled` | Enable Intel IPU plugin | `true` |
| `plugins.intel.opiEndpoint` | Intel OPI bridge endpoint | `localhost:50052` |
| `plugins.intel.networkEndpoint` | Intel EVPN-GW endpoint for networking | `""` |
| `plugins.marvell.enabled` | Enable Marvell Octeon plugin | `true` |
| `plugins.marvell.opiEndpoint` | Marvell OPI bridge endpoint | `localhost:50053` |
| `plugins.marvell.networkEndpoint` | Marvell EVPN-GW endpoint for networking | `""` |
| `plugins.xsight.enabled` | Enable xSight plugin | `true` |
| `plugins.xsight.opiEndpoint` | xSight OPI bridge endpoint | `localhost:50054` |
| `plugins.xsight.networkEndpoint` | xSight EVPN-GW endpoint for networking | `""` |
| `plugins.mangoboost.enabled` | Enable MangoBoost plugin | `false` |
| `plugins.mangoboost.opiEndpoint` | MangoBoost OPI bridge endpoint | `localhost:50055` |
| `plugins.mangoboost.networkEndpoint` | MangoBoost EVPN-GW endpoint for networking | `""` |

### Component Images

| Parameter | Description | Default |
|-----------|-------------|---------|
| `daemon.image.repository` | DPU daemon image repository | `ghcr.io/openshift/dpu-daemon` |
| `daemon.image.tag` | DPU daemon image tag | `latest` |
| `nri.image.repository` | NRI webhook image repository | `quay.io/openshift/dpu-network-resources-injector` |
| `nri.image.tag` | NRI webhook image tag | `latest` |
| `vspImages.intelIpu` | Intel IPU VSP image | `quay.io/openshift/dpu-intel-ipu-vsp:latest` |
| `vspImages.intelNetsec` | Intel NetSec VSP image | `quay.io/openshift/dpu-intel-netsec-vsp:latest` |
| `vspImages.intelP4` | Intel P4 SDK image | `quay.io/openshift/dpu-intel-ipu-p4sdk:latest` |
| `vspImages.marvellDpu` | Marvell VSP image | `quay.io/openshift/dpu-marvell-vsp:latest` |
| `vspImages.marvellCpAgent` | Marvell CP agent image | `quay.io/openshift/dpu-marvell-cp-agent:latest` |
| `vspImages.nvidiaBf` | NVIDIA BlueField VSP image | `quay.io/openshift/dpu-nvidia-bf-vsp:latest` |
| `vspImages.xsight` | xSight VSP image | `quay.io/openshift/dpu-xsight-vsp:latest` |
| `vspImages.mangoboost` | MangoBoost VSP image | `quay.io/openshift/dpu-mangoboost-vsp:latest` |

### Operator Config

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operatorConfig.create` | Create a default DpuOperatorConfig | `true` |
| `operatorConfig.logLevel` | DpuOperatorConfig spec log level | `0` |
| `operatorConfig.resourceName` | Override DPU resource name used by NADs and device plugin | `""` |

### DPU Daemon

| Parameter | Description | Default |
|-----------|-------------|---------|
| `daemon.enabled` | Deploy DPU daemon DaemonSet | `true` |
| `daemon.nodeSelector` | Node selector for daemon pods | `{"feature.node.kubernetes.io/dpu": "true"}` |

## Examples

### Minimal Installation

```bash
helm install dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --create-namespace
```

### High Availability with Resource Limits

```yaml
# values-ha.yaml
operator:
  replicas: 3
  leaderElection:
    enabled: true

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            app.kubernetes.io/name: dpu-operator
        topologyKey: kubernetes.io/hostname
```

```bash
helm install dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --create-namespace \
  -f values-ha.yaml
```

### Production Setup with Security and Monitoring

```yaml
# values-production.yaml
operator:
  logLevel: 0

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s

networkPolicy:
  enabled: true

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532
  seccompProfile:
    type: RuntimeDefault

priorityClassName: system-cluster-critical

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

```bash
helm install dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --create-namespace \
  -f values-production.yaml
```

## Upgrading

```bash
helm upgrade dpu-operator ./charts/dpu-operator \
  --namespace dpu-operator-system \
  --reuse-values
```

## Troubleshooting

### Check operator logs

```bash
kubectl logs -n dpu-operator-system \
  -l control-plane=controller-manager \
  --tail=100 -f
```

### Verify CRDs are installed

```bash
kubectl get crds | grep dpu
```

### Check plugin registration

```bash
kubectl logs -n dpu-operator-system \
  -l control-plane=controller-manager | \
  grep "Registered plugin"
```

## Contributing

See the main [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](../../LICENSE) for details.
