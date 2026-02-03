# Project Overview

This repository contains the unified, vendor-agnostic Kubernetes DPU (Data Processing Unit) operator. The project's goal is to provide a single Kubernetes operator that manages DPU/IPU devices from multiple vendors—Intel, Marvell, NVIDIA, xSight, and others—through a standardized plugin architecture and OPI-based APIs.

The current OpenShift DPU Operator (`github.com/openshift/dpu-operator`) supports Intel IPU E2100, Intel NetSec Accelerator (Senao SX904), and Marvell Octeon 10 as of OpenShift 4.20. However, the existing implementation has vendor-specific logic somewhat hard-coded, lacks a formal plugin registry, and does not yet support NVIDIA BlueField, xSight, or Mangoboost hardware. The OPI Storage and Security APIs are also not integrated.

This project extends the existing operator with: (1) a formal plugin registry and a hybrid runtime that bridges in-process plugins with the existing VSP gRPC path, (2) new vendor plugins for NVIDIA BlueField, xSight, and optionally Mangoboost, and (3) integration of OPI Storage and Security APIs where vendor bridges are available. The v2 CRD redesign is deferred; v1 remains the active API surface for now.

The Open Programmable Infrastructure (OPI) project, a Linux Foundation initiative, defines the gRPC/protobuf APIs (`github.com/opiproject/opi-api`) and reference bridge implementations (`opi-spdk-bridge`, `opi-nvidia-bridge`, `opi-intel-bridge`, `opi-marvell-bridge`) that this operator leverages for vendor-neutral communication with DPU hardware.

The strategic value proposition is eliminating vendor lock-in: users deploy one operator, configure one set of CRDs, and can run workloads on any supported DPU without learning vendor-specific tools or APIs. The operator abstracts hardware complexity behind Kubernetes-native primitives.

---

# Tech Stack and Scope

## Languages and Tooling

- **Primary Language:** Go 1.21+
- **Kubernetes Tooling:** controller-runtime, operator-sdk, kubebuilder patterns
- **API Definitions:** Kubernetes CRDs (Go types generating YAML), gRPC/protobuf for OPI APIs
- **Build System:** Taskfile (taskfile.yaml), Make, Go modules
- **Container Runtime:** Podman/Docker, multi-stage Dockerfiles for RHEL-based images
- **CI/CD:** OpenShift CI (Prow), GitHub Actions for community testing
- **Testing:** Go testing, Ginkgo/Gomega for BDD-style tests, Kind for integration tests

## Supported and Target Platforms

| Platform | Status | Notes |
|----------|--------|-------|
| OpenShift 4.19+ | Primary target | Full integration with OLM, OperatorHub |
| Vanilla Kubernetes 1.28+ | Secondary target | Abstract OpenShift-specific dependencies where feasible |

| DPU Hardware | Status | Vendor |
|--------------|--------|--------|
| Intel IPU E2100 | ✅ Supported | Intel |
| Intel NetSec Accelerator (Senao SX904) | ✅ Supported | Intel |
| Marvell Octeon 10 | ✅ Supported | Marvell |
| NVIDIA BlueField-2/3 | 🔨 To Build | NVIDIA |
| xSight | 🔨 To Build | xSight |
| Mangoboost | 🔶 Optional | Mangoboost |

## In Scope

- Plugin architecture design and implementation
- Hybrid runtime integration (registry plugins + VSP gRPC path)
- NVIDIA BlueField plugin (discovery, inventory, networking, optional storage)
- xSight plugin (discovery, inventory, networking)
- OPI Storage API integration (NVMe-oF frontend/backend)
- OPI Security API integration (IPsec offload)
- Unit tests, integration tests (Kind), emulation tests (OPI mock servers)
- Documentation for users and plugin developers
- Backward compatibility with existing v1 CRDs

## Out of Scope

- Upstream kernel driver development for DPU devices
- Vendor SDK development (we consume existing SDKs)
- Full production deployment automation (Ansible, Terraform)
- Performance optimization beyond baseline functionality
- Support for DPU vendors not listed above unless explicitly requested

---

# Current State of the World

## Existing Components in `openshift/dpu-operator`

| Component | Responsibility | Status |
|-----------|---------------|--------|
| `dpu-operator-controller-manager` | Main controller deployment; watches DpuOperatorConfig and Dpu CRs; reconciles desired state | Complete |
| `dpu-daemon` | DaemonSet running on host nodes with DPUs; interfaces with VSP and DPU via gRPC | Complete |
| `vsp` (Vendor-Specific Plugin) | Per-vendor binaries translating OPI API calls to vendor SDK calls | Complete for Intel/Marvell |
| `dpu-cni` | CNI plugin enabling pod networking through DPU | Complete |
| `network-resources-injector` | Mutating webhook injecting network resource requests into pods | Complete |
| `DpuOperatorConfig` CRD | User-facing configuration; specifies mode (host/dpu) and settings | Complete (v1) |
| `Dpu` CRD | Represents discovered DPU hardware; populated by operator | Complete (v1) |
| `ServiceFunctionChain` CRD | Defines network functions to deploy on DPU | Basic |

## Current CRD Limitations

The v1 CRDs mix generic and vendor-specific fields in a single flat structure. Adding a new vendor requires modifying the core CRD types, which violates separation of concerns. The `DpuOperatorConfig` spec does not have a clean extension mechanism for vendor-specific configuration. A v2 redesign was planned, but is deferred until the runtime integration is fully stabilized.

## OPI API Integration Status

| OPI API Category | Protobuf Package | Integration Status |
|------------------|------------------|-------------------|
| Inventory | `opi_api.inventory.v1` | ✅ Implemented |
| Networking | `opi_api.network.v1` | 🔶 Partial (basic port management) |
| Storage | `opi_api.storage.v1` | ❌ Not integrated |
| Security (IPsec) | `opi_api.security.v1` | ❌ Not integrated |
| Lifecycle | `opi_api.lifecycle.v1` | 🔶 Partial |
| AI/ML | `opi_api.aiml.v1` | ❌ Not integrated |

## Current VSP Design Limitations

The VSPs are implemented as separate binaries and remain required for hardware
bring-up and for establishing the host↔DPU communication channel. The operator
now has a formal Go plugin interface and registry, and the daemon can use
registry plugins for discovery/VF configuration with safe fallback to VSP.

Adding a new vendor still requires:
- Creating or adopting a VSP (or OPI bridge) that can initialize the hardware
- Providing a plugin implementation for metadata and discovery
- Supplying container images and manifests for deployment

The hybrid model reduces hard-coded vendor logic but does not yet provide
dynamic plugin loading.

## Existing OPI Bridge Implementations

| Repository | Purpose | Maturity |
|------------|---------|----------|
| `opiproject/opi-spdk-bridge` | Storage via SPDK; reference implementation | Production-ready for testing |
| `opiproject/opi-intel-bridge` | Intel SDK integration | Complete |
| `opiproject/opi-nvidia-bridge` | NVIDIA DOCA/SNAP integration | Complete but needs integration |
| `opiproject/opi-marvell-bridge` | Marvell SDK integration | Complete |
| `opiproject/opi-evpn-bridge` | EVPN via FRRouting | Complete |
| `opiproject/opi-strongswan-bridge` | IPsec via strongSwan | Complete |

---

# Target Architecture and Design Principles

## Plugin Architecture

The target architecture introduces a formal plugin system with three core components:

**Plugin Interface:** A Go interface (`pkg/plugin/interface.go`) that all vendor plugins must implement. The interface defines:
- `Info() PluginInfo` — returns plugin metadata (name, vendor, version, supported PCI device IDs, capabilities)
- `Initialize(ctx, config) error` — initializes the plugin with vendor-specific configuration
- `Shutdown(ctx) error` — graceful shutdown
- `HealthCheck(ctx) error` — liveness/readiness probing
- `DiscoverDevices(ctx) ([]Device, error)` — scans for DPU hardware
- `GetInventory(ctx, deviceID) (*InventoryResponse, error)` — returns OPI-format inventory

**Capability Interfaces:** Optional interfaces for specific offload types:
- `NetworkPlugin` — port management, OVS configuration, flow offload
- `StoragePlugin` — NVMe subsystem/controller/namespace management
- `SecurityPlugin` — IPsec tunnel management

Plugins declare which capability interfaces they implement via the `Capabilities` field in `PluginInfo`.

**Plugin Registry:** A singleton registry (`pkg/plugin/registry.go`) that:
- Holds all registered plugins (populated via `init()` functions)
- Provides lookup by plugin name or PCI vendor:device ID
- Supports capability queries ("give me all plugins supporting storage")

**Hybrid Runtime Integration:** The daemon now supports a hybrid runtime model:
- The existing VSP gRPC path remains authoritative for hardware bring-up and
  for establishing the host↔DPU communication channel (VSP `Init` returns the IP/port).
- Registry plugins are initialized opportunistically in the daemon and used for
  discovery and VF configuration when available, with safe fallback to the VSP
  gRPC implementation if a registry plugin is absent or unimplemented.

This keeps current behavior intact while allowing in-process plugins to take on
more responsibility over time without breaking VSP-dependent workflows.

## CRD Strategy (v1 for now)

The operator continues to use the v1 CRDs in the `config.openshift.io` API group
as the active, supported surface. A v2 redesign was scoped but deferred until
the hybrid runtime integration is proven in production. The immediate focus is
stability and vendor integration rather than schema churn.

If/when v2 resumes, it should be driven by concrete vendor config requirements
and migration tooling, not by speculative schema changes.

## OPI APIs as Canonical Control Plane

Where OPI bridges are available, registry plugins communicate via OPI-defined
gRPC APIs. The operator core never calls vendor SDKs directly. The runtime flow
is now hybrid:
1. Controller creates/updates Dpu CRs based on detection
2. DPU Daemon initializes the VSP to bring up the comm channel
3. The daemon uses registry plugins for discovery/VF config when available
4. Registry plugins translate to OPI gRPC calls
5. OPI bridge (running on DPU or host) translates to vendor SDK

This layering keeps the core vendor-agnostic while preserving the existing VSP
bring-up path.

## Deployment Models

**Single-Cluster (1c):** Host nodes and DPU nodes are in the same OpenShift/Kubernetes cluster. DPUs run MicroShift or a lightweight Kubernetes. Simpler to manage.

**Dual-Cluster (2c):** Host cluster (OpenShift on x86) and DPU cluster (MicroShift on ARM DPUs) are separate. Better isolation, more complex management. Operator deploys to both clusters.

The plugin architecture supports both models; deployment topology is a configuration choice.

## Design Principles

1. **Vendor-agnostic core:** The operator controller and daemon code must not import vendor-specific packages or contain vendor-specific conditionals outside of plugin implementations.

2. **All vendor logic behind plugin interfaces:** Any code that touches a vendor SDK, calls vendor-specific APIs, or handles vendor-specific device quirks belongs in a plugin package, never in core.

3. **Backward compatibility:** Existing users on v1 CRDs must be able to upgrade without manual migration. Avoid schema churn until a concrete v2 migration plan exists.

4. **Schema stability first:** Keep the v1 CRDs stable and document vendor-specific configuration via env/configmaps or operator configuration until a v2 design is justified by real needs.

5. **Prefer OPI bridge integration over direct SDK calls:** When an OPI bridge exists (e.g., `opi-nvidia-bridge`), use it via gRPC rather than linking the vendor SDK directly. This reduces binary size, simplifies builds, and leverages community-maintained bridges.

6. **Test at every tier:** Every new feature must have unit tests (mocked), integration tests (Kind), and where applicable, emulation tests (OPI mock). Hardware tests are required before GA.

7. **Document alongside code:** Every PR that adds functionality must include or update relevant documentation. No undocumented features.

---

# Logical Build Order and Milestones

## Phase 1: Foundation (Weeks 1-3)

**Goals:** Establish the plugin architecture scaffold without breaking existing functionality.

**Tasks:**
- Define the `Plugin` interface in `pkg/plugin/interface.go`
- Define capability interfaces (`NetworkPlugin`, `StoragePlugin`, `SecurityPlugin`)
- Implement the plugin registry in `pkg/plugin/registry.go`
- Add unit tests for registry behavior (registration, lookup, capability queries)

**Dependencies:** None (greenfield code).

**Entry Criteria:** Existing operator tests pass.

**Exit Criteria:** Plugin interface and registry implemented with >90% unit test coverage; no changes to existing VSP behavior yet.

---

## Phase 2: Hybrid Runtime Integration (Weeks 3-5)

**Goals:** Wire registry plugins into the daemon while preserving the existing VSP
bring-up path and avoiding regressions.

**Tasks:**
- Attach registry plugins to the VSP-backed gRPC plugin
- Initialize registry plugins from env-based config (OPI endpoints, log level)
- Use registry plugins for discovery and VF configuration with safe fallback to VSP
- Update documentation to reflect the hybrid runtime model

**Dependencies:** Phase 1 complete.

**Entry Criteria:** Plugin interface and registry exist.

**Exit Criteria:** Hybrid runtime is enabled; VSP path remains authoritative for
bring-up; registry plugins improve discovery/VF handling without breaking existing behavior.

---

## Phase 3: Plugin Maturity and Parity (Weeks 5-6)

**Goals:** Bring registry plugin behavior closer to VSP parity for supported vendors.

**Tasks:**
- Implement missing operations in Intel/Marvell/xSight/Mangoboost plugins
- Improve unit and emulation coverage for plugin operations
- Optionally route additional operations (bridge ports, network functions) through
  registry plugins when mappings are stable

**Dependencies:** Phase 2 complete.

**Entry Criteria:** Hybrid runtime stabilized.

**Exit Criteria:** Registry plugins provide reliable device discovery and VF control;
operation parity gaps are documented and shrinking.

---

## Phase 4: NVIDIA BlueField Plugin (Weeks 6-9)

**Goals:** Implement full NVIDIA BlueField support.

**Tasks:**
- Obtain DOCA SDK access; study documentation
- Decide integration approach: `opi-nvidia-bridge` (preferred) vs CGO wrappers
- Create `pkg/plugin/nvidia/` package
- Implement `DiscoverDevices` (PCI ID detection: 15b3:a2d6, 15b3:a2dc)
- Implement `GetInventory` via `opi-nvidia-bridge` or direct DOCA calls
- Implement `NetworkPlugin` methods (OVS-DOCA configuration, representor ports, flow offload)
- Implement `StoragePlugin` methods (SNAP NVMe emulation) — optional but differentiating
- Add unit tests with mocked DOCA client
- Add emulation tests against `opi-nvidia-bridge` mock
- Coordinate with NVIDIA LaunchPad for hardware testing

**Dependencies:** Phase 3 complete; NVIDIA developer program access.

**Entry Criteria:** Intel/Marvell plugins migrated; registry working.

**Exit Criteria:** NVIDIA plugin discovers BlueField hardware, returns inventory, configures networking; passes emulation tests; hardware tests in vendor lab.

---

## Phase 5: xSight Plugin (Weeks 9-10)

**Goals:** Implement xSight support.

**Tasks:**
- Contact xSight for SDK access and documentation
- Create `pkg/plugin/xsight/` package
- Implement standard plugin interface (discovery, inventory, networking)
- Add unit and emulation tests
- Coordinate with xSight for hardware testing

**Dependencies:** Phase 3 complete; xSight SDK access.

**Entry Criteria:** Plugin architecture stable.

**Exit Criteria:** xSight plugin functional; passes tests.

---

## Phase 6: OPI Storage and Security Integration (Weeks 10-11)

**Goals:** Integrate OPI Storage and Security APIs across all plugins.

**Tasks:**
- Define `StoragePlugin` interface methods mapping to OPI Storage API
- Define `SecurityPlugin` interface methods mapping to OPI IPsec API
- Implement storage support in NVIDIA plugin (SNAP + SPDK)
- Implement IPsec support using `opi-strongswan-bridge`
- Add emulation tests using `opi-spdk-bridge`

**Dependencies:** Phase 4 complete.

**Entry Criteria:** At least one vendor plugin fully functional.

**Exit Criteria:** Storage and IPsec offload working for at least NVIDIA; emulation tests pass.

---

## Phase 7: Testing and Hardening (Weeks 11-12)

**Goals:** Achieve production-quality test coverage and stability.

**Tasks:**
- Achieve >80% unit test coverage across all packages
- Integration tests in Kind covering multi-vendor scenarios
- E2E tests on real hardware (Intel, Marvell, NVIDIA via vendor labs)
- Performance benchmarking and profiling
- Fix bugs discovered during testing

**Dependencies:** Phases 4-6 complete.

**Entry Criteria:** All plugins implemented.

**Exit Criteria:** All tests pass; no critical bugs; performance meets baseline.

---

## Phase 8: Documentation and Release (Week 12)

**Goals:** Production-ready documentation and release artifacts.

**Tasks:**
- Write user documentation (installation, configuration, troubleshooting)
- Write plugin developer guide (how to add a new vendor)
- Document hybrid runtime behavior and plugin configuration
- Update README
- Tag release
- Build and publish container images
- Submit to OperatorHub (if targeting OpenShift)

**Dependencies:** Phase 7 complete.

**Entry Criteria:** All tests pass; code frozen.

**Exit Criteria:** Documentation complete; release published; announced to community.

---

# Testing Strategy and CI/CD Expectations

## Testing Tiers

### Tier 1: Unit Tests

**What to test:** Go code logic—plugin registration, config parsing, CRD validation, controller reconciliation with mocked clients.

**Environment:** None; pure Go tests.

**Frameworks:** Go `testing` package; testify for assertions; controller-runtime's fake client for K8s mocks; grpc-go `bufconn` for in-memory gRPC.

**Coverage target:** >80% line coverage for all packages.

**How Claude Code should generate tests:** For every new function or method, generate a corresponding `_test.go` file with table-driven tests covering happy path, error cases, and edge cases. Mock external dependencies (K8s client, gRPC connections, vendor SDKs).

### Tier 2: Integration Tests (Kind Cluster)

**What to test:** Operator deploys correctly; CRDs install and validate; controller watches and reconciles; DaemonSets created on labeled nodes.

**Environment:** Kind cluster with fake "DPU nodes" (labeled regular nodes).

**Frameworks:** Ginkgo/Gomega for BDD-style tests; envtest for controller testing.

**How to run:** `task test-integration` or `go test -tags=integration ./...`

**What cannot be tested:** Actual hardware discovery, real network/storage offload, vendor SDK interactions.

### Tier 3: OPI Emulation Tests

**What to test:** gRPC client code correctness; OPI API compliance; response parsing.

**Environment:** Docker containers running OPI mock servers (`opi-spdk-bridge`, mock inventory server).

**How to run:** Start mock server via Docker Compose; point plugin at mock endpoint; run tests.

**What this validates:** Plugin correctly calls OPI APIs and handles responses without needing real hardware.

### Tier 4: Hardware-in-Loop Tests

**What to test:** Real vendor SDK calls; actual hardware discovery; real offload functionality.

**Environment:** Physical servers with DPU hardware; access via vendor partner labs (NVIDIA LaunchPad, Intel Developer Cloud) or owned hardware.

**How to run:** Deploy operator to real cluster with DPU nodes; run e2e test suite.

### Tier 5: Full E2E Production Tests

**What to test:** Complete multi-vendor cluster; workload deployment; network connectivity through DPU; storage access through DPU; performance benchmarks.

**Environment:** Full lab setup with multiple DPU types.

**How to run:** OpenShift CI with hardware pools; or self-hosted GitHub Actions runners.

## CI/CD Pipeline Structure

**PR Checks:**
- Linting (`golangci-lint`)
- Unit tests (`go test ./...`)
- Integration tests in Kind
- CRD validation

**Nightly/Weekly:**
- Emulation tests against OPI mock servers
- Hardware tests (if self-hosted runners available)

**Release:**
- Full E2E suite on real hardware
- Build and push container images
- Generate release notes

## Test Matrix

| Component | Unit | Integration | Emulation | Hardware |
|-----------|------|-------------|-----------|----------|
| Plugin Interface | ✅ | ✅ | - | - |
| Plugin Registry | ✅ | ✅ | - | - |
| CRD Validation | ✅ | ✅ | - | - |
| Controller Logic | ✅ | ✅ | - | - |
| Intel Plugin | ✅ | - | ✅ | ✅ |
| Marvell Plugin | ✅ | - | ✅ | ✅ |
| NVIDIA Plugin | ✅ | - | ✅ | ✅ |
| xSight Plugin | ✅ | - | ✅ | ✅ |
| OPI gRPC Calls | - | - | ✅ | ✅ |
| Network Offload | - | - | ❌ | ✅ |
| Storage Offload | - | - | ✅ (SPDK) | ✅ |

---

# Coding Conventions and Repository Expectations

## Go Version and Modules

- Go 1.24 or later
- Module path: `github.com/openshift/dpu-operator` (or fork path)
- Vendor dependencies committed (`go mod vendor`)

## Package Layout

```
dpu-operator/
├── api/
│   ├── v1/                 # Existing v1 CRD types (keep for compatibility)
├── cmd/
│   ├── manager/            # Operator entrypoint
│   ├── daemon/             # DPU daemon entrypoint
│   └── cni/                # CNI plugin entrypoint
├── config/
│   ├── crd/                # Generated CRD YAML
│   ├── rbac/               # RBAC manifests
│   └── manager/            # Deployment manifests
├── internal/
│   ├── controller/         # Reconciliation logic (not importable externally)
│   └── daemon/             # Daemon implementation
├── pkg/
│   ├── plugin/             # Plugin interface, registry, capability interfaces
│   │   ├── interface.go
│   │   ├── registry.go
│   │   ├── intel/          # Intel plugin implementation
│   │   ├── marvell/        # Marvell plugin implementation
│   │   ├── nvidia/         # NVIDIA plugin implementation
│   │   └── xsight/         # xSight plugin implementation
│   └── opi/                # OPI API client wrappers
├── test/
│   ├── e2e/                # E2E test suites
│   └── integration/        # Integration test helpers
└── docs/                   # User and developer documentation
```

## Naming Conventions

- Package names: lowercase, single word (`plugin`, `nvidia`, `registry`)
- Interface names: verb or noun describing behavior (`Plugin`, `NetworkPlugin`)
- Struct names: PascalCase (`BlueFieldPlugin`, `PluginRegistry`)
- Function names: PascalCase for exported, camelCase for unexported
- Constants: PascalCase for exported, camelCase for unexported
- File names: lowercase with underscores (`bluefield_plugin.go`, `registry_test.go`)

## Plugin Implementation Pattern

Each vendor plugin package must:
1. Define a struct implementing the `Plugin` interface
2. Implement all required methods
3. Implement optional capability interfaces as appropriate
4. Register with the registry in an `init()` function
5. Include a `*_test.go` file with unit tests using mocked dependencies

## Linting and Formatting

- Run `gofmt -s` on all files
- Run `golangci-lint run` with project config
- No lint errors in CI

## Modifying Existing Code

When modifying existing code:
1. Make small, reviewable changes (one logical change per PR)
2. Preserve backward compatibility unless explicitly breaking (document in PR)
3. Use feature flags for experimental features
4. Add deprecation warnings before removing functionality
5. Update tests to cover modified behavior

---

# How to Work with This Project as Claude Code

## Before Major Changes

Always read or re-open `CLAUDE.md` before starting significant work. If details are needed, consult `opi-dpu-operator-analysis.md` and `unified-dpu-operator-roadmap.md` for deeper context.

## When Asked for Implementation

1. **Identify the phase:** State which phase and component the requested work belongs to (e.g., "This is Phase 4, NVIDIA Plugin - Network Offload").

2. **Check dependencies:** If the request targets a later-phase feature with missing prerequisites, either:
   - Stub dependencies cleanly with TODO comments and interfaces returning not-implemented errors; or
   - Ask the user if they want to jump ahead and accept technical debt.

3. **Align to build order:** Prefer implementing features in the logical build order. If the user requests out-of-order work, note the deviation.

## When Adding a New Vendor Plugin

1. Create a new package under `pkg/plugin/<vendor>/`
2. Implement the `Plugin` interface
3. Implement relevant capability interfaces (`NetworkPlugin`, `StoragePlugin`, `SecurityPlugin`)
4. Register the plugin in an `init()` function calling `plugin.Register()`
5. Add unit tests with mocked SDK/gRPC clients
6. Add emulation tests if an OPI mock server exists for the vendor
7. Update documentation:
   - Add vendor to supported hardware matrix
   - Document vendor-specific configuration options
   - Add troubleshooting section

## When Touching CRDs

1. Preserve v1 types in `api/v1/`
2. Avoid schema churn unless required; document any behavior changes
3. Regenerate CRD YAML (`make manifests`)
4. Update documentation to match the v1 schema

## When Integrating OPI APIs or Bridge Repos

1. Import OPI API Go stubs from `github.com/opiproject/opi-api`
2. Create thin wrappers in `pkg/opi/` if needed for convenience
3. Never expose vendor-specific types from OPI bridges in core controller logic
4. Use gRPC clients configured with appropriate endpoints (configurable via DpuOperatorConfig)
   or environment variables (`DPU_PLUGIN_OPI_ENDPOINT` / `DPU_PLUGIN_OPI_ENDPOINT_<VENDOR>` in hybrid mode)
5. Handle gRPC errors gracefully with retries and logging

## Generating Tests and Documentation

Every significant code change must include:
- Unit tests for new functions/methods
- Updates to integration tests if controller behavior changes
- Documentation updates if user-facing behavior changes

Do not merge code without corresponding tests.

---

# Known Risks, Dependencies, and Decision Log

## Technical Dependencies

| Dependency | Risk Level | Mitigation |
|------------|------------|------------|
| DOCA SDK Go bindings | HIGH | Use `opi-nvidia-bridge` via gRPC; fall back to CGO if needed |
| `opi-nvidia-bridge` stability | MEDIUM | Pin to specific version; fork and stabilize if needed |
| Hardware availability for testing | HIGH | Apply for vendor partner labs early; use emulators for initial development |
| OpenShift version compatibility | LOW | Test on 4.19, 4.20, 4.21 |
| OPI API stability | LOW | Pin to specific `opi-api` version; update carefully |

## External Coordination Required

- **Red Hat:** Align with OpenShift roadmap for upstreaming
- **NVIDIA:** DOCA SDK access, LaunchPad lab access, technical support
- **xSight:** SDK access, device documentation, lab access
- **OPI Project:** API stability discussions, feature requests

## Default Decisions (Assume Unless User Overrides)

1. **DOCA integration approach:** Use `opi-nvidia-bridge` via gRPC first; only use CGO wrappers for features not exposed by the bridge.

2. **Development strategy:** Fork `openshift/dpu-operator`, build features, then contribute upstream. This allows faster iteration without blocking on upstream review cycles.

3. **Hybrid runtime:** Keep the VSP gRPC path for hardware bring-up and comm-channel setup, and use registry plugins for discovery/VF config when available, with safe fallback to VSP. This minimizes risk while enabling incremental migration to in-process plugins.

4. **Kubernetes compatibility:** Abstract OpenShift-specific dependencies where easy (e.g., use standard K8s APIs when possible); keep OpenShift-specific features (e.g., MachineConfig, OLM) where required for full functionality.

5. **Vendor priority:** NVIDIA BlueField first (highest demand), then xSight (per CFP requirements), Mangoboost optional.

6. **Storage/Security integration:** Implement as optional capabilities; not required for MVP but differentiating.

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| DOCA SDK has no Go bindings | High | Medium | Use `opi-nvidia-bridge` or write CGO wrappers |
| Vendor lab access delayed | Medium | High | Apply early; have emulator-based development path |
| OPI APIs change during development | Low | Medium | Pin to specific version |
| Red Hat merges similar work first | Medium | High | Move fast; differentiate on features (storage, multi-vendor) |
| Hardware incompatibilities | Medium | Medium | Test early with emulators; validate assumptions with vendor docs |
| Performance not meeting expectations | Low | High | Profile early; optimize hot paths; set realistic baselines |

---

# How to Ask for Clarification

Ask the user for clarification when:

1. **Vendor SDK details are missing:** "I need access to the xSight SDK documentation to implement discovery. Can you provide SDK docs or access credentials?"

2. **Target K8s/OpenShift version is ambiguous:** "Should this feature support OpenShift 4.19, 4.20, or both? The API differs between versions."

3. **Conflicting requirements:** "You've asked for generic Kubernetes support, but this feature requires OpenShift's MachineConfig API. Should I implement an OpenShift-specific path and stub the generic path, or skip generic support for now?"

4. **Hardware testing logistics:** "The NVIDIA plugin is ready for hardware testing. Do you have access to NVIDIA LaunchPad, or should I document the manual testing steps for you to run?"

5. **Scope boundaries:** "Implementing full storage offload for all vendors would take an additional 3 weeks. Should I implement it for NVIDIA only, or defer storage entirely to a future phase?"

Make reasonable assumptions for:
- Code style (follow existing patterns in repo)
- Test structure (mirror source file structure)
- Documentation format (Markdown, follow existing docs)
- Error handling (return wrapped errors, log with context)

Do not ask for clarification on routine implementation details; make a reasonable choice and note it in the PR description.
