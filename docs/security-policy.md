# Security Policy

This document describes the security posture and best practices for the DPU Operator.

## Pod Security Standards

The DPU Operator complies with Kubernetes **Pod Security Standards (PSS)** at the **Restricted** level, the most stringent security profile.

### Security Contexts

All operator pods run with:

- **Non-root user**: `runAsUser: 65532` (nobody)
- **No privilege escalation**: `allowPrivilegeEscalation: false`
- **Dropped capabilities**: All Linux capabilities dropped
- **Read-only root filesystem**: `readOnlyRootFilesystem: true`
- **Seccomp profile**: `RuntimeDefault`

### Network Policies

The operator deploys with **deny-all-by-default** network policies:

| Policy | Purpose |
|--------|---------|
| `deny-all-default` | Blocks all ingress and egress by default |
| `operator-egress` | Allows only necessary outbound connections (K8s API, DNS, OPI bridges) |
| `allow-metrics-scraping` | Permits Prometheus to scrape metrics on port 8080 |
| `allow-webhook-traffic` | Enables admission webhooks on port 9443 |

### Resource Limits

Resource quotas prevent resource exhaustion attacks:

- **Namespace-level quota**: Max 8 CPU cores, 16Gi memory
- **Pod-level limits**: Max 2 CPU cores, 4Gi memory per container
- **Default requests**: 50m CPU, 64Mi memory
- **No persistent volumes** or external load balancers allowed

## RBAC (Role-Based Access Control)

The operator uses **least-privilege RBAC**:

### Cluster-Wide Permissions

- **Full control**: DPU CRDs only (`dpus`, `dpuconfigs`, etc.)
- **Read-only**: Nodes, pods, services (for discovery)
- **Limited write**: DaemonSets, ConfigMaps (for managed workloads)

### Namespace Permissions

- **Full control**: Resources in `dpu-operator-system` namespace only
- **No cluster-admin**: Operator never requires cluster-admin role

## Secrets and Credentials

### OPI Bridge Authentication

- OPI bridges use **mTLS** when TLS is enabled
- Certificates managed by **cert-manager**
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

1. **Privilege escalation**: Prevented by PSS Restricted + read-only filesystem
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

1. **Always use TLS** for OPI bridge connections
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

- [ ] Pod Security Standards enforced at Restricted level
- [ ] NetworkPolicies enabled with deny-all default
- [ ] ResourceQuota and LimitRange configured
- [ ] TLS enabled for all OPI connections
- [ ] Image pull from trusted registries only
- [ ] RBAC follows least-privilege principle
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
