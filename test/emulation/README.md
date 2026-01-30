# OPI Bridge Emulation Tests

This directory contains emulation tests for DPU plugins that run against real OPI bridge implementations in Docker containers. These tests validate plugin behavior without requiring physical DPU hardware.

## Overview

The emulation test suite:
- Runs DPU plugins against OPI bridge mock servers
- Tests plugin initialization, health checks, and operations
- Validates gRPC communication with OPI APIs
- Simulates multi-vendor scenarios

## Prerequisites

1. **Docker** installed and running
2. **Go 1.23+** installed
3. **DPU Operator** built (`go build ./pkg/...`)

## Quick Start

### 1. Start OPI Bridges

```bash
cd ~/unified-k8s/dpu-operator/test/emulation
docker-compose up -d
```

This starts 5 OPI bridge containers:
- `opi-nvidia` on port 50051 (NVIDIA BlueField)
- `opi-intel` on port 50052 (Intel IPU)
- `opi-spdk` on port 50053 (Storage reference)
- `opi-marvell` on port 50054 (Marvell Octeon)
- `opi-strongswan` on port 50055 (IPsec/Security)

### 2. Wait for Services

```bash
# Check if bridges are healthy
docker-compose ps

# Wait ~10 seconds for all services to be ready
sleep 10
```

### 3. Run Emulation Tests

```bash
cd ~/unified-k8s/dpu-operator
export PATH=/home/kalyanp/go-local/go/bin:$PATH
export GOPATH=/home/kalyanp/go

go test -tags=emulation ./test/emulation/... -v
```

### 4. Stop OPI Bridges

```bash
cd ~/unified-k8s/dpu-operator/test/emulation
docker-compose down
```

## Test Coverage

### Test Matrix

| Plugin | OPI Bridge | Port | Health | Discovery | Network | Storage | Security |
|--------|-----------|------|--------|-----------|---------|---------|----------|
| **NVIDIA** | opi-nvidia-bridge | 50051 | ✅ | ✅ | ✅ | 🔶 | ⏳ |
| **Intel** | opi-intel-bridge | 50052 | ✅ | ✅ | ✅ | 🔶 | ⏳ |
| **Marvell** | opi-marvell-bridge | 50054 | ✅ | ✅ | ✅ | ❌ | ❌ |
| **xSight** | (mock only) | - | ✅ | ✅ | 🔶 | ❌ | ❌ |

Legend:
- ✅ Fully tested
- 🔶 Partially tested
- ⏳ Planned
- ❌ Not supported

### Test Cases

#### `TestNVIDIAPlugin_WithOPIBridge`
- Initializes NVIDIA plugin with opi-nvidia-bridge
- Tests health checks
- Tests device discovery
- Tests bridge port operations (create, list, delete)

#### `TestIntelPlugin_WithOPIBridge`
- Initializes Intel plugin with opi-intel-bridge
- Tests health checks
- Tests device discovery
- Tests bridge port operations

#### `TestPluginConnectivity`
- Verifies all plugins can connect to their bridges
- Tests basic connectivity and health

#### `TestMultiVendorEmulation`
- Simulates a multi-vendor cluster
- Initializes multiple plugins simultaneously
- Validates they can coexist

#### `TestOPIBridgeAvailability`
- Prerequisite check
- Verifies all OPI bridges are running
- Identifies which bridges are available

## Docker Images

The docker-compose uses official OPI project images from GitHub Container Registry:
- `ghcr.io/opiproject/opi-nvidia-bridge:main`
- `ghcr.io/opiproject/opi-intel-bridge:main`
- `ghcr.io/opiproject/opi-spdk-bridge:main`
- `ghcr.io/opiproject/opi-marvell-bridge:main`
- `ghcr.io/opiproject/opi-strongswan-bridge:main`

### Building Images Locally

If images are not available on ghcr.io, build them locally:

```bash
# NVIDIA bridge
cd ~/unified-k8s/opi-nvidia-bridge
docker build -t ghcr.io/opiproject/opi-nvidia-bridge:main .

# Intel bridge
cd ~/unified-k8s/opi-intel-bridge
docker build -t ghcr.io/opiproject/opi-intel-bridge:main .

# SPDK bridge
cd ~/unified-k8s/opi-spdk-bridge
docker build -t ghcr.io/opiproject/opi-spdk-bridge:main .

# Marvell bridge
cd ~/unified-k8s/opi-marvell-bridge
docker build -t ghcr.io/opiproject/opi-marvell-bridge:main .

# StrongSwan bridge
cd ~/unified-k8s/opi-strongswan-bridge
docker build -t ghcr.io/opiproject/opi-strongswan-bridge:main .
```

## Troubleshooting

### Bridges Not Starting

Check Docker logs:
```bash
docker-compose logs opi-nvidia
docker-compose logs opi-intel
```

### Connection Refused Errors

1. Verify bridges are running:
   ```bash
   docker-compose ps
   ```

2. Check bridge health:
   ```bash
   curl http://localhost:8082/v1/inventory/1/inventory/2  # NVIDIA
   curl http://localhost:8083/v1/inventory/1/inventory/2  # Intel
   ```

3. Restart bridges:
   ```bash
   docker-compose restart
   ```

### Tests Failing

1. **Check prerequisites**:
   - Docker running: `docker ps`
   - Bridges up: `docker-compose ps`
   - Go available: `go version`

2. **Run with verbose output**:
   ```bash
   go test -tags=emulation ./test/emulation/... -v -timeout 60s
   ```

3. **Test individual bridges**:
   ```bash
   # Test NVIDIA only
   go test -tags=emulation ./test/emulation/... -v -run TestNVIDIA

   # Test Intel only
   go test -tags=emulation ./test/emulation/... -v -run TestIntel
   ```

### Port Conflicts

If ports 50051-50055 or 8082-8086 are already in use, modify `docker-compose.yml` to use different ports.

## Advanced Usage

### Running Individual Bridges

Start only specific bridges:

```bash
# NVIDIA only
docker-compose up -d opi-nvidia redis-nvidia

# Intel only
docker-compose up -d opi-intel redis-intel

# Multiple
docker-compose up -d opi-nvidia redis-nvidia opi-intel redis-intel
```

### Accessing Bridge HTTP Gateways

Each bridge exposes an HTTP gateway for REST API access:

```bash
# NVIDIA bridge
curl http://localhost:8082/v1/inventory/1/inventory/2

# Intel bridge
curl http://localhost:8083/v1/inventory/1/inventory/2

# SPDK bridge
curl http://localhost:8084/v1/inventory/1/inventory/2
```

### Using grpc-cli

Test bridges with grpc-cli:

```bash
# List services
docker run --rm --network host namely/grpc-cli ls localhost:50051

# Call Ping method
docker run --rm --network host namely/grpc-cli call localhost:50051 opi_api.lifecycle.v1.LifecycleService.Ping ""
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Emulation Tests

on: [push, pull_request]

jobs:
  emulation:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Start OPI Bridges
        run: |
          cd test/emulation
          docker-compose up -d
          sleep 15

      - name: Run Emulation Tests
        run: go test -tags=emulation ./test/emulation/... -v

      - name: Stop OPI Bridges
        if: always()
        run: |
          cd test/emulation
          docker-compose down
```

## Further Reading

- [OPI Project Documentation](https://github.com/opiproject)
- [OPI API Specifications](https://github.com/opiproject/opi-api)
- [NVIDIA Bridge](https://github.com/opiproject/opi-nvidia-bridge)
- [Intel Bridge](https://github.com/opiproject/opi-intel-bridge)
- [SPDK Bridge](https://github.com/opiproject/opi-spdk-bridge)

## Contributing

When adding new emulation tests:

1. Use the `//go:build emulation` build tag
2. Follow existing test patterns
3. Test against all relevant bridges
4. Update this README with new test coverage
5. Ensure tests clean up resources (defer Shutdown())
6. Use reasonable timeouts (30-60 seconds)
