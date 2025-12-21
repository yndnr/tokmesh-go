# TokMesh Deployment Guide

This directory contains deployment configurations and scripts for TokMesh Server.

## Quick Start

### Docker (Recommended for Development)

```bash
# Build Docker image
make docker-build

# Run with Docker Compose
cd deployments/docker
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f tokmesh-server

# Stop
docker-compose down
```

### Linux (systemd)

```bash
# Build binaries
make build-linux

# Install
cd bin
sudo bash ../scripts/install-linux.sh

# Start service
sudo systemctl start tokmesh-server

# Check status
sudo systemctl status tokmesh-server
sudo journalctl -u tokmesh-server -f

# Reload configuration (hot reload)
sudo systemctl reload tokmesh-server

# Uninstall
sudo bash ../scripts/uninstall-linux.sh
```

### Kubernetes

```bash
# Create namespace (optional)
kubectl create namespace tokmesh

# Deploy
kubectl apply -f deployments/kubernetes/

# Check status
kubectl get statefulset tokmesh
kubectl get pods -l app=tokmesh
kubectl get svc tokmesh-external

# View logs
kubectl logs -l app=tokmesh -f

# Scale
kubectl scale statefulset tokmesh --replicas=5

# Delete
kubectl delete -f deployments/kubernetes/
```

## Directory Structure

```
deployments/
├── linux/
│   └── tokmesh-server.service    # systemd service unit
├── docker/
│   └── docker-compose.yaml       # Docker Compose configuration
└── kubernetes/
    ├── configmap.yaml            # Configuration
    ├── statefulset.yaml          # StatefulSet definition
    ├── service.yaml              # Service definitions
    └── serviceaccount.yaml       # RBAC configuration
```

## Build Targets

```bash
make build              # Build for current platform
make build-linux        # Build for Linux (amd64, arm64)
make build-windows      # Build for Windows (amd64)
make build-darwin       # Build for macOS (amd64, arm64)
make build-all          # Build for all platforms

make docker-build       # Build Docker image
make docker-push        # Build and push Docker image

make test               # Run tests
make test-coverage      # Run tests with coverage

make clean              # Clean build artifacts
make help               # Show all targets
```

## Configuration

### Environment Variables

TokMesh Server supports configuration via environment variables with the `TOKMESH_` prefix:

```bash
# Server
TOKMESH_SERVER_HTTP_ADDR=0.0.0.0:5080
TOKMESH_SERVER_HTTPS_ADDR=0.0.0.0:5443

# Storage
TOKMESH_STORAGE_DATA_DIR=/var/lib/tokmesh-server

# Cluster
TOKMESH_CLUSTER_ENABLED=true
TOKMESH_CLUSTER_NODE_ID=node-1
TOKMESH_CLUSTER_ADDR=0.0.0.0:5343

# Logging
TOKMESH_LOG_LEVEL=info
TOKMESH_LOG_FORMAT=json
```

### Configuration File

Default locations:
- Linux: `/etc/tokmesh-server/config.yaml`
- Container: `/etc/tokmesh-server/config.yaml`
- Custom: `--config /path/to/config.yaml`

See `configs/server/` for example configurations.

## Security Hardening

### Linux (systemd)

The systemd service includes security features:
- `NoNewPrivileges=true` - Prevents privilege escalation
- `ProtectSystem=strict` - Read-only system directories
- `ProtectHome=true` - Hides home directories
- `PrivateTmp=true` - Private /tmp directory
- File permissions: 0600/0640 for configs

### Docker

The Docker image:
- Runs as non-root user (`tokmesh`)
- Uses multi-stage build for minimal size
- Read-only root filesystem
- No new privileges
- Drops all capabilities

### Kubernetes

The StatefulSet:
- Runs as non-root user (UID 1000)
- Read-only root filesystem
- No privilege escalation
- Drops all capabilities
- RBAC configured (ServiceAccount, Role, RoleBinding)

## Health Checks

### HTTP Endpoints

- `GET /health` - Liveness probe (returns 200 if running)
- `GET /ready` - Readiness probe (returns 200 if ready to serve)
- `GET /metrics` - Prometheus metrics

### Docker

```bash
docker exec tokmesh-server wget -q -O- http://localhost:5080/health
```

### Kubernetes

```bash
kubectl exec -it tokmesh-0 -- wget -q -O- http://localhost:5080/health
```

## Troubleshooting

### Linux

```bash
# Check service status
systemctl status tokmesh-server

# View logs
journalctl -u tokmesh-server -f
journalctl -u tokmesh-server --since "1 hour ago"

# Test configuration
tokmesh-server --config /etc/tokmesh-server/config.yaml --test

# Check file permissions
ls -la /etc/tokmesh-server/
ls -la /var/lib/tokmesh-server/
```

### Docker

```bash
# View logs
docker-compose logs -f tokmesh-server

# Check health
docker inspect tokmesh-server | grep Health -A 10

# Shell into container
docker exec -it tokmesh-server sh
```

### Kubernetes

```bash
# Pod status
kubectl describe pod tokmesh-0

# Logs
kubectl logs tokmesh-0
kubectl logs tokmesh-0 --previous  # Previous crashed container

# Events
kubectl get events --sort-by='.lastTimestamp'

# PVC status
kubectl get pvc
kubectl describe pvc data-tokmesh-0
```

## Performance Tuning

### Resource Limits

Recommended settings:

**Development:**
- CPU: 100m - 500m
- Memory: 128Mi - 512Mi

**Production:**
- CPU: 500m - 2000m
- Memory: 512Mi - 2Gi
- Storage: 10Gi - 100Gi

### File Descriptors

Increase if handling many concurrent connections:

```bash
# Linux
ulimit -n 65535

# systemd (already configured)
LimitNOFILE=65535
```

## Backup and Restore

### Manual Backup

```bash
# Create snapshot
tokmesh-cli backup snapshot --description "Manual backup"

# Download snapshot
tokmesh-cli backup download snap-20251221-120000 -o backup.bak

# Restore
tokmesh-cli backup restore --file backup.bak --force
```

### Kubernetes Backup

```bash
# Backup PVC data
kubectl exec tokmesh-0 -- tar czf - /var/lib/tokmesh-server > backup.tar.gz

# Restore
kubectl exec tokmesh-0 -- tar xzf - -C /var/lib/tokmesh-server < backup.tar.gz
```

## Monitoring

### Prometheus

Metrics endpoint: `http://localhost:9090/metrics`

Key metrics:
- `tokmesh_sessions_active` - Active sessions
- `tokmesh_tokens_validated_total` - Token validations
- `tokmesh_wal_write_bytes_total` - WAL write bytes
- `tokmesh_http_requests_total` - HTTP requests

### Grafana Dashboard

See `configs/telemetry/prometheus/` for example dashboards.

## Support

- Documentation: https://github.com/yndnr/tokmesh-go
- Issues: https://github.com/yndnr/tokmesh-go/issues
