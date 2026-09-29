# Yuheng Helm Chart

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/yuheng)](https://artifacthub.io/packages/helm/yuheng/yuheng)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Helm chart for deploying [Yuheng](https://github.com/magicyuan876/yuheng) - an AI-powered Knowledge RAG Platform.

## Overview

Yuheng is an intelligent knowledge base platform that combines:
- Document parsing and understanding
- Vector search with BM25 hybrid retrieval
- LLM integration for conversational AI
- Multi-tenant support with encryption

## Prerequisites

- Kubernetes 1.25+
- Helm 3.10+
- PV provisioner support in the underlying infrastructure
- Ingress controller (nginx-ingress recommended) for external access

## Quick Start

```bash
# Add required secrets
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set secrets.dbPassword=<your-db-password> \
  --set secrets.redisPassword=<your-redis-password> \
  --set secrets.jwtSecret=<your-jwt-secret>
```

## Architecture

```
                    ┌─────────────┐
                    │   Ingress   │
                    └──────┬──────┘
                           │
           ┌───────────────┴───────────────┐
           │                               │
           ▼                               ▼
    ┌─────────────┐                 ┌─────────────┐
    │  Frontend   │                 │   Backend   │
    │  (Vue.js)   │                 │   (Go/Gin)  │
    └─────────────┘                 └──────┬──────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
                    ▼                      ▼                      ▼
             ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
             │  Docreader  │        │  PostgreSQL │        │    Redis    │
             │   (gRPC)    │        │  (ParadeDB) │        │   (Queue)   │
             └─────────────┘        └─────────────┘        └─────────────┘
```

## Installation

### Basic Installation

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### With Ingress

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set ingress.enabled=true \
  --set ingress.host=yuheng.example.com \
  --set ingress.tls.enabled=true \
  --set ingress.tls.secretName=yuheng-tls \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### With External LLM (Ollama)

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set app.extraEnv[0].name=OLLAMA_BASE_URL \
  --set app.extraEnv[0].value=http://ollama.ollama:11434 \
  --set app.extraEnv[1].name=INIT_LLM_MODEL_NAME \
  --set app.extraEnv[1].value=qwen2.5:7b \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### Production Installation

For production, use a values file:

```yaml
# values-production.yaml
global:
  storageClass: "fast-ssd"

app:
  replicaCount: 3
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2
      memory: 4Gi

postgresql:
  persistence:
    size: 100Gi

ingress:
  enabled: true
  host: yuheng.company.com
  tls:
    enabled: true
    secretName: yuheng-tls

secrets:
  existingSecret: yuheng-secrets  # Use pre-created secret
```

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  -f values-production.yaml
```

## Configuration

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.storageClass` | Storage class for PVCs | `""` |
| `global.imagePullSecrets` | Image pull secrets | `[]` |
| `global.podSecurityContext` | Pod security context | See values.yaml |
| `global.containerSecurityContext` | Container security context | See values.yaml |

### ServiceAccount

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |

### App (Backend)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `app.enabled` | Enable backend | `true` |
| `app.replicaCount` | Number of replicas | `1` |
| `app.image.repository` | Image repository | `magicyuan876/yuheng-app` |
| `app.image.tag` | Image tag | `""` (uses appVersion) |
| `app.resources` | Resource limits | See values.yaml |
| `app.env` | Environment variables | See values.yaml |
| `app.extraEnv` | Additional env vars | `[]` |

### Frontend

| Parameter | Description | Default |
|-----------|-------------|---------|
| `frontend.enabled` | Enable frontend | `true` |
| `frontend.replicaCount` | Number of replicas | `1` |
| `frontend.image.repository` | Image repository | `magicyuan876/yuheng-ui` |
| `frontend.image.tag` | Image tag | `latest` |

### PostgreSQL (ParadeDB)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL | `true` |
| `postgresql.image.repository` | Image repository | `paradedb/paradedb` |
| `postgresql.image.tag` | Image tag | `v0.18.9-pg17` |
| `postgresql.persistence.enabled` | Enable persistence | `true` |
| `postgresql.persistence.size` | PVC size | `10Gi` |

### Redis

| Parameter | Description | Default |
|-----------|-------------|---------|
| `redis.enabled` | Enable Redis | `true` |
| `redis.image.repository` | Image repository | `redis` |
| `redis.image.tag` | Image tag | `7-alpine` |
| `redis.persistence.enabled` | Enable persistence | `true` |
| `redis.persistence.size` | PVC size | `1Gi` |

### Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class | `nginx` |
| `ingress.host` | Hostname | `yuheng.example.com` |
| `ingress.tls.enabled` | Enable TLS | `false` |
| `ingress.tls.secretName` | TLS secret name | `""` |

### Secrets

| Parameter | Description | Default |
|-----------|-------------|---------|
| `secrets.dbUser` | Database username | `postgres` |
| `secrets.dbPassword` | Database password | `""` (required) |
| `secrets.dbName` | Database name | `yuheng` |
| `secrets.redisPassword` | Redis password | `""` (required) |
| `secrets.jwtSecret` | JWT signing secret | `""` (required) |
| `secrets.existingSecret` | Use existing secret | `""` |

### Optional Components

These map to docker-compose profiles:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `neo4j.enabled` | Enable Neo4j (GraphRAG) | `false` |
| `docs.enabled` | Enable online documents | `false` |
| `collab.enabled` | Enable collaborative editing for online documents | `false` |

### Online documents

Two switches, and they are independent in one direction only.

`docs.enabled=true` turns the module on. That is enough for a complete
product: spaces, pages, permissions, comments, history, search, export and
import all work, and the editor uses **exclusive editing** — one writer per
page at a time, holding a five-minute lease that renews while they type, with
everybody else reading and seeing who has it.

`collab.enabled=true` adds the collaboration service, which replaces that with
real-time multi-writer editing. It is an upgrade to the same editor and the
same build: only the transport differs. It requires `docs.enabled`, and the
chart refuses to render without it rather than deploying a service with
nothing to serve.

```bash
helm install yuheng ./helm   --set docs.enabled=true   --set collab.enabled=true   --set secrets.collabSharedSecret="$(openssl rand -base64 32)"   --set ingress.enabled=true   --set ingress.host=docs.example.com
```

`docs.collabUrl` defaults to `ws(s)://<ingress.host>/collab`, which is where
the chart's own ingress rule sends the WebSocket. Set it explicitly when the
browser reaches the cluster by some other route.

**Two things to check on your ingress controller.** A WebSocket needs the
protocol upgrade to survive the proxy, and it needs not to be cut off by the
default read timeout — an editing session holds one connection open for as
long as the document is on screen. On ingress-nginx:

```yaml
ingress:
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
```

**More than one replica needs Redis.** Set `collab.redis.enabled=true` before
raising `collab.replicaCount`. Without it two replicas holding the same
document never see each other's edits: both are internally consistent, both
believe they are correct, and whichever stores last overwrites the other.
That failure is silent, which is why the chart templates the Redis URL rather
than leaving it to be remembered.

## Security Best Practices

### Secret Management

**Never commit secrets to Git!** Use one of these approaches:

1. **Helm --set flags** (for testing)
   ```bash
   helm install yuheng ./helm --set secrets.dbPassword=xxx
   ```

2. **External Secrets Operator** (recommended for production)
   ```yaml
   secrets:
     existingSecret: yuheng-external-secret
   ```

3. **Sealed Secrets** (for GitOps)
   ```bash
   kubeseal < secret.yaml > sealed-secret.yaml
   ```

### Pod Security

The chart follows CNCF security best practices:
- Runs as non-root user
- Read-only root filesystem where possible
- Drops all capabilities
- Uses seccomp profiles

## Upgrading

```bash
helm upgrade yuheng ./helm \
  --namespace yuheng \
  --reuse-values
```

## Uninstalling

```bash
helm uninstall yuheng --namespace yuheng

# Optional: Remove PVCs
kubectl delete pvc -n yuheng -l app.kubernetes.io/instance=yuheng
```

## Troubleshooting

### Check Pod Status
```bash
kubectl get pods -n yuheng
```

### View Logs
```bash
# Backend logs
kubectl logs -n yuheng -l app.kubernetes.io/component=app -f

# Frontend logs
kubectl logs -n yuheng -l app.kubernetes.io/component=frontend -f
```

### Common Issues

**Pod stuck in Pending**
- Check if PVCs are bound: `kubectl get pvc -n yuheng`
- Verify storage class exists: `kubectl get sc`

**Connection refused errors**
- Wait for all pods to be Ready
- Check service endpoints: `kubectl get endpoints -n yuheng`

**Database connection errors**
- Verify secrets are correct
- Check PostgreSQL logs: `kubectl logs -n yuheng -l app.kubernetes.io/component=database`

## Contributing

See [CONTRIBUTING.md](https://github.com/magicyuan876/yuheng/blob/main/CONTRIBUTING.md) in the main repository.

## References

This Helm chart follows best practices from:
- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [ArgoCD Helm Chart](https://github.com/argoproj/argo-helm)
- [Prometheus Helm Charts](https://github.com/prometheus-community/helm-charts)
- [cert-manager Helm Chart](https://github.com/cert-manager/cert-manager)

## License

This chart is licensed under the MIT License - see the [LICENSE](https://github.com/magicyuan876/yuheng/blob/main/LICENSE) file for details.
