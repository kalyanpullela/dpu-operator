# Security Policy

This document describes the security posture and best practices for the DPU Operator.

## Pod Security Standards

The DPU Operator uses a **mixed** Pod Security Standards model:

- The controller manager can run at **Restricted** level.
- The daemon and VSP pods require **Privileged** access (hostNetwork/hostPID, device access).

**Note:** The example manifest in `config/security/pod-security-standards.yaml` targets the
`openshift-dpu-operator` namespace. If you deploy via Helm (often `dpu-operator-system`),
adjust the namespace accordingly.

### Security Contexts

The controller manager pods run with:

- **Non-root user**: `runAsUser: 65532` (nobody)
- **No privilege escalation**: `allowPrivilegeEscalation: false`
- **Dropped capabilities**: All Linux capabilities dropped
- **Read-only root filesystem**: `readOnlyRootFilesystem: true`
- **Seccomp profile**: `RuntimeDefault`

The daemon and VSP pods run privileged to access host devices and networking
and therefore do not meet the Restricted profile.

### Network Policies

The operator deploys with **deny-all-by-default** network policies:

| Policy | Purpose |
|--------|---------|
| `deny-all-default` | Blocks all ingress and egress by default |
| `operator-egress` | Allows only necessary outbound connections (K8s API, DNS, OPI bridges) |
| `allow-metrics-scraping` | Permits Prometheus to scrape metrics on port 10443 |
| `allow-webhook-traffic` | Enables admission webhooks on port 9443 |

### Resource Limits

Resource quotas prevent resource exhaustion attacks:

- **Namespace-level quota**: Max 8 CPU cores, 16Gi memory
- **Pod-level limits**: Max 2 CPU cores, 4Gi memory per container
- **Default requests**: 50m CPU, 64Mi memory
- **No persistent volumes** or external load balancers allowed

## RBAC (Role-Based Access Control)

The operator uses **scoped RBAC**, but some core resources require broad verbs
to support privileged daemon/VSP workflows.

### Cluster-Wide Permissions

- **Full control**: DPU CRDs only (`dpus`, `dpuconfigs`, etc.)
- **Broad verbs**: Core resources like pods/services are required for VSP and daemon management
- **Managed workloads**: DaemonSets, Deployments, RBAC objects

### Namespace Permissions

- **Full control**: Resources in the operator namespace only (defaults to `openshift-dpu-operator` for
  `config/default` and OLM installs, and `system` for the lower-level `config/manager/` base)
- **No cluster-admin**: Operator never requires cluster-admin role

## Secrets and Credentials

### OPI Bridge Authentication

- OPI gRPC connections are **plaintext by default** today
- TLS/mTLS support is planned and should be enabled when implemented
- No hardcoded credentials or API keys

### Image Pull Secrets

- Support for private registries via `imagePullSecrets`
- Secrets never logged or exposed in metrics

## Vulnerability Management

### Automated Scanning

CI/CD pipeline includes:

- **Trivy**: Container image vulnerability scanning
- **govulncheck**: Go vulnerability database scanning
- **CodeQL**: Static code analysis for security issues
- **Dependency Review**: GitHub dependency scanning

### Patching Policy

- **Critical CVEs**: Patched within 7 days
- **High CVEs**: Patched within 30 days
- **Medium/Low CVEs**: Addressed in next minor release

## Admission Control

### Webhooks

The operator implements:

- **Validating webhooks**: Enforce CRD schema and business logic
- **Mutating webhooks**: Apply security defaults (if needed)
- **Failure policy**: `Fail` (block invalid resources)

### Audit Logging

OpenShift audit logs capture:

- All CRD create/update/delete operations
- Webhook validation results
- RBAC authorization denials

## Compliance

The operator is designed to support:

- **PCI DSS**: No credit card data processed
- **HIPAA**: No PHI processed by operator
- **SOC 2**: Audit logging, RBAC, encryption in transit

## Threat Model

### In-Scope Threats

1. **Privilege escalation**: Controller manager is restricted; daemon/VSP require privileged access
2. **Resource exhaustion**: Mitigated by ResourceQuota and LimitRange
3. **Network exfiltration**: Prevented by default-deny NetworkPolicies
4. **Supply chain attacks**: Addressed by image signing and SBOM

### Out-of-Scope Threats

- Physical hardware attacks on DPU devices
- Side-channel attacks (Spectre/Meltdown)
- DDoS attacks on managed infrastructure

## Security Reporting

### Reporting a Vulnerability

Email: `dpu-operator-security@lists.openshift.io`

Include:
- Description of vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if available)

### Response SLA

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 7 days
- **Fix timeline**: Based on severity (see Patching Policy above)

## Best Practices for Users

### Deployment

1. **Use TLS** for OPI bridge connections when supported
2. **Enable NetworkPolicies** in production
3. **Set resource limits** on all pods
4. **Use dedicated namespace** for operator
5. **Enable audit logging** for compliance

### Node Security

For nodes with DPU hardware:

1. **Label nodes** appropriately (`feature.node.kubernetes.io/dpu=true`)
2. **Apply taints** to prevent non-DPU workloads
3. **Use SELinux/AppArmor** on host OS
4. **Keep firmware updated** on DPU devices

### Monitoring

1. **Enable Prometheus metrics** for security events
2. **Set alerts** for reconciliation errors
3. **Monitor OPI bridge errors** for anomalies
4. **Track device health** metrics

## Security Hardening Checklist

- [ ] Pod Security Standards enforced (Restricted for controller, Privileged for daemon/VSP)
- [ ] NetworkPolicies enabled with deny-all default
- [ ] ResourceQuota and LimitRange configured
- [ ] TLS/mTLS enabled for OPI connections (when supported)
- [ ] Image pull from trusted registries only
- [ ] RBAC scoped appropriately for privileged workflows
- [ ] Secrets stored in encrypted backend (etcd encryption at rest)
- [ ] Audit logging enabled
- [ ] Security scanning in CI/CD pipeline
- [ ] Vulnerability patching SLA defined

## References

- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [OpenShift Security Context Constraints](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)
- [OPI Security Architecture](https://github.com/opiproject/opi-api/blob/main/doc/security.md)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)

## License

This security policy is licensed under Apache License 2.0.

Last Updated: 2026-02-01
