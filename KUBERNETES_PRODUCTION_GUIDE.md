# Kubernetes Production Hardening Guide
**Version:** 1.0 | **Date:** 2026-06-02

---

## Prerequisites

- Kubernetes 1.28+
- Helm 3.x
- kubectl configured for your cluster
- Container registry with `trading-engine:latest` image

---

## Quick Deploy

```bash
# Create namespace and all resources
kubectl apply -f infrastructure/kubernetes/trading-engine.yaml
kubectl apply -f infrastructure/kubernetes/timescale-ha.yaml
kubectl apply -f infrastructure/kubernetes/redis-ha.yaml

# Verify
kubectl -n trading get pods
kubectl -n trading get pdb
```

---

## Resource Requirements

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|------------|-----------|----------------|--------------|
| Trading Engine × 3 | 500m | 2000m | 512Mi | 2Gi |
| TimescaleDB Primary | 500m | 4000m | 1Gi | 4Gi |
| TimescaleDB Replica | 250m | 2000m | 512Mi | 2Gi |
| Redis Primary | 100m | 500m | 256Mi | 768Mi |
| Redis Replica | 100m | 500m | 256Mi | 768Mi |
| Redis Sentinel × 3 | 50m | 200m | 64Mi | 128Mi |

**Total minimum cluster capacity required:** 4 CPU, 8Gi RAM

---

## Anti-Affinity Rules

All StatefulSets use `requiredDuringSchedulingIgnoredDuringExecution` anti-affinity
on `kubernetes.io/hostname`. This guarantees no two trading engine pods, DB pods,
or Redis pods land on the same physical node.

For production, use a dedicated node pool:
```yaml
tolerations:
  - key: "trading-workload"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

```bash
# Label nodes for trading workload
kubectl label nodes <node1> <node2> <node3> trading-workload=true
kubectl taint nodes <node1> <node2> <node3> trading-workload=true:NoSchedule
```

---

## Health Probes

### Liveness Probe
Kills and restarts the container if it fails.  
Path: `GET /health` → 200  
Parameters: `initialDelaySeconds=15`, `periodSeconds=10`, `failureThreshold=3`

### Readiness Probe
Removes the pod from load-balancer rotation if it fails.  
Path: `GET /ready` → 200  
Parameters: `initialDelaySeconds=10`, `periodSeconds=5`, `failureThreshold=2`

### Startup Probe
Prevents liveness probe from killing a slow-starting pod.  
Path: `GET /health` → 200  
Parameters: `initialDelaySeconds=5`, `periodSeconds=5`, `failureThreshold=12` (60s window)

---

## PodDisruptionBudget

```yaml
# Trading Engine: always keep 2 of 3 running
minAvailable: 2

# TimescaleDB: always keep 1 running
minAvailable: 1

# Redis Sentinel: always keep 2 of 3 running (quorum)
minAvailable: 2
```

This prevents voluntary disruptions (node drain, rolling updates) from taking the system below quorum.

---

## Rolling Updates

```bash
# Zero-downtime rolling update
kubectl -n trading set image statefulset/trading-engine engine=trading-engine:v2

# Monitor rollout
kubectl -n trading rollout status statefulset/trading-engine

# Rollback if needed
kubectl -n trading rollout undo statefulset/trading-engine
```

The StatefulSet `maxUnavailable: 1` ensures at least 2 pods are always running during updates.

---

## Secrets Management

### Approach 1: Vault Agent Injector (Recommended)
```yaml
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "trading-engine"
  vault.hashicorp.com/agent-inject-secret-db: "secret/trading/db"
```

### Approach 2: External Secrets Operator
```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: engine-secrets
  namespace: trading
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: engine-secrets
  data:
    - secretKey: DATABASE_URL
      remoteRef:
        key: secret/trading/db
        property: url
```

---

## Encryption Key Rotation

The AES-256 backup encryption key must be rotated quarterly:

```bash
# 1. Generate new key
NEW_KEY=$(openssl rand -hex 32)

# 2. Update secret
kubectl -n trading patch secret engine-secrets \
  --patch '{"data":{"BACKUP_ENCRYPTION_KEY":"'$(echo -n "$NEW_KEY" | base64)'"}}'

# 3. Re-encrypt existing backups (automated script)
go run ./cmd/backup/reencrypt/main.go \
  --old-key $OLD_KEY \
  --new-key $NEW_KEY \
  --backup-dir /data/backups

# 4. Restart engine to pick up new key
kubectl -n trading rollout restart statefulset/trading-engine
```

---

## Monitoring Stack

```bash
# Install kube-prometheus-stack
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.adminPassword=changeme \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false

# Import DR dashboard
kubectl -n monitoring create configmap dr-dashboard \
  --from-file=dr-dashboard.json
```

Add `ServiceMonitor` for the trading engine:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: trading-engine
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: trading-engine
  namespaceSelector:
    matchNames: [trading]
  endpoints:
    - port: http
      path: /metrics
      interval: 10s
```

---

## Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: trading-engine-netpol
  namespace: trading
spec:
  podSelector:
    matchLabels:
      app: trading-engine
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: trading
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - port: 8080
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: trading
    - to:  # External exchanges
        ports:
          - port: 443
          - port: 80
```

---

## Production Checklist

- [ ] Node anti-affinity verified across 3+ nodes
- [ ] PodDisruptionBudgets applied to all StatefulSets
- [ ] Secrets loaded from Vault (not hardcoded)
- [ ] Resource limits set on all containers
- [ ] Health probes verified end-to-end
- [ ] Rolling update tested in staging
- [ ] Backup encryption key stored in Vault
- [ ] NetworkPolicy applied
- [ ] ServiceMonitor configured for Prometheus
- [ ] DR dashboard imported to Grafana
- [ ] AlertManager rules loaded
- [ ] PagerDuty integration tested
- [ ] DR test suite run and passed
