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

Yuheng publishes no container images. Build its images and push them to a
registry the cluster can pull from first (see [Images](#images)), then:

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set global.imageRegistry=<your-registry>/yuheng \
  --set secrets.dbPassword=<your-db-password> \
  --set secrets.redisPassword=<your-redis-password> \
  --set secrets.jwtSecret=<your-jwt-secret>
```

## Images

The chart runs four images that come from this repository — `yuheng-app`,
`yuheng-ui` (frontend), `yuheng-docreader` and, with `collab.enabled`,
`yuheng-collab` — and pulls the third-party ones (ParadeDB, Redis, Neo4j) from
Docker Hub as usual. Nothing publishes the first four: the release workflow
only runs on version tags and none has been cut. So the chart has no default
for them, and `helm install` / `helm template` stop with an error naming
`global.imageRegistry` until you say where they are, instead of installing
pods that can never pull.

Build them for the cluster's architecture and push them under one prefix:

```bash
REGISTRY=registry.example.com/yuheng   # anything your nodes can pull from
TAG=v0.1.0                             # the chart's appVersion, or pick your own

./scripts/build_images.sh --all        # app, docreader, frontend (tags magicyuan876/<name>:latest)
./scripts/build_images.sh --collab     # only with collab.enabled=true

for name in yuheng-app yuheng-ui yuheng-docreader yuheng-collab; do
  docker tag "magicyuan876/$name:latest" "$REGISTRY/$name:$TAG"
  docker push "$REGISTRY/$name:$TAG"
done
```

`build_images.sh` builds for the machine it runs on (`linux/amd64` or
`linux/arm64`); for nodes of the other architecture build with
`docker buildx build --platform ...` from the same Dockerfiles
(`docker/Dockerfile.app`, `docker/Dockerfile.docreader`, `frontend/Dockerfile`
after `scripts/build_frontend_dist.sh`, `collab/Dockerfile`).

Then install with `--set global.imageRegistry=$REGISTRY`. The chart pulls
`$REGISTRY/<name>:<appVersion>`; `<component>.image.tag` overrides the tag and
`<component>.image.repository` the whole repository of one component, for
images kept under different paths. A private registry needs
`global.imagePullSecrets`.

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
  --set global.imageRegistry=registry.example.com/yuheng \
  --set secrets.dbPassword=secure-password \
  --set secrets.redisPassword=secure-password \
  --set secrets.jwtSecret=$(openssl rand -base64 32)
```

### With Ingress

```bash
helm install yuheng ./helm \
  --namespace yuheng \
  --create-namespace \
  --set global.imageRegistry=registry.example.com/yuheng \
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
  --set global.imageRegistry=registry.example.com/yuheng \
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
  imageRegistry: registry.example.com/yuheng
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
| `global.imageRegistry` | Registry/path of Yuheng's own images (see [Images](#images)) | `""` (required) |
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
| `app.image.repository` | Image repository | `""` (`<global.imageRegistry>/yuheng-app`) |
| `app.image.tag` | Image tag | `""` (appVersion) |
| `app.resources` | Resource limits | See values.yaml |
| `app.env` | Environment variables | See values.yaml |
| `app.extraEnv` | Additional env vars | `[]` |

### Frontend

| Parameter | Description | Default |
|-----------|-------------|---------|
| `frontend.enabled` | Enable frontend | `true` |
| `frontend.replicaCount` | Number of replicas | `1` |
| `frontend.image.repository` | Image repository | `""` (`<global.imageRegistry>/yuheng-ui`) |
| `frontend.image.tag` | Image tag | `""` (appVersion) |

### PostgreSQL (ParadeDB)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL | `true` |
| `postgresql.image.repository` | Image repository | `paradedb/paradedb` |
| `postgresql.image.tag` | Image tag (same as docker-compose.yml) | `v0.22.2-pg17` |
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
helm install yuheng ./helm   --set global.imageRegistry=registry.example.com/yuheng   --set docs.enabled=true   --set collab.enabled=true   --set secrets.collabSharedSecret="$(openssl rand -base64 32)"   --set ingress.enabled=true   --set ingress.host=docs.example.com
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

What the defaults actually set (`global.podSecurityContext`,
`global.containerSecurityContext` and the per-component `securityContext` in
values.yaml):

- every pod uses the `RuntimeDefault` seccomp profile, and every container has
  `allowPrivilegeEscalation: false`;
- only the collaboration service runs with `runAsNonRoot: true`. The app and
  docreader images start as root and drop to an unprivileged user in their
  entrypoint (after fixing volume ownership); the frontend, PostgreSQL and
  Redis images run as their upstream images do;
- root filesystems are writable and capabilities are not dropped.

Tighten these per component in your values file where your images allow it.

## Upgrading

```bash
helm upgrade yuheng ./helm \
  --namespace yuheng \
  --reuse-values
```

**Chart 0.1.x → 0.2.0.** Two changes need attention on an existing release:

- Yuheng's own images no longer default to `magicyuan876/*`, which were never
  published; set `global.imageRegistry` (or each component's
  `image.repository`), or the upgrade stops with an error saying so.
- ParadeDB moves from `v0.18.9-pg17` to `v0.22.2-pg17`, the version
  docker-compose runs and the schema is tested against. The data directory
  carries over (both are PostgreSQL 17), but the `pg_search` extension inside
  the database stays at its old version until it is updated. Back the volume
  up first (website-docs/01-getting-started/05-backup-and-upgrade.md); once the
  postgres pod runs the new image, and before relying on search, run (for a
  release named `yuheng` with the default `secrets.dbUser` / `secrets.dbName`):

  ```bash
  kubectl exec -n yuheng deploy/yuheng-postgres -- \
    psql -U postgres -d yuheng -c 'ALTER EXTENSION pg_search UPDATE;'
  ```

  To stay on the old ParadeDB instead, pin `postgresql.image.tag=v0.18.9-pg17`.

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
