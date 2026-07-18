# gitops-ai-platform

Push code → get a running app, deployed via GitOps. This repo implements
weeks 1–6 of the platform roadmap (see `docs/ARCHITECTURE.md`):

```
detect  →  containerize  →  generate manifests  →  commit to GitOps repo  →  (ArgoCD/Flux syncs)
```

AI (risk review + post-deploy root-cause analysis) is weeks 7–10 and not
included here — everything in this repo is deterministic, template-based
generation, which is intentional: manifest *shape* shouldn't be
probabilistic, only judgment about whether the *values* are risky should be.

## Components

| Path | What it does |
|---|---|
| `pkg/detector` | Inspects a repo, infers language/framework/entrypoint/port |
| `pkg/containerizer` | Generates a Dockerfile from the detected service |
| `pkg/platformconfig` | Reads optional `platform.yaml` overrides from the app repo |
| `pkg/manifest` | Generates Deployment/Service/Ingress/Kustomization YAML |
| `pkg/gitopswriter` | Commits generated manifests to a GitOps repo; renders the ArgoCD `Application` |
| `cmd/pipeline` | Runs all of the above end-to-end |
| `cmd/detector`, `cmd/containerizer`, `cmd/manifest-generator`, `cmd/gitops-writer` | Same stages as standalone CLIs, for scripting/debugging one stage at a time |
| `examples/sample-app` | A minimal Express app used to exercise the whole pipeline |

## Quickstart

```bash
go build -o bin/pipeline ./cmd/pipeline

# Set up a local GitOps repo (in real usage this is your existing ArgoCD/Flux-watched repo)
git init /tmp/gitops-repo && cd /tmp/gitops-repo && git commit --allow-empty -m init

./bin/pipeline \
  -app-repo ./examples/sample-app \
  -gitops-repo /tmp/gitops-repo \
  -app sample-app \
  -image registry.example.com/sample-app:sha-abc123 \
  -env staging \
  -gitops-repo-url https://github.com/you/gitops-repo.git
```

This will:
1. Detect the app is Node/Express, listening on the port from `platform.yaml` (or a framework default)
2. Write a multi-stage, non-root `Dockerfile` into `examples/sample-app/`
3. Generate `deployment.yaml`, `service.yaml`, `ingress.yaml` (if `public: true`), `kustomization.yaml`
4. Commit them to `/tmp/gitops-repo/apps/sample-app/staging/`
5. Print the one-time `ArgoCD Application` resource to onboard the app

From there, ArgoCD/Flux — already watching that GitOps repo, unmodified —
picks up the commit and reconciles the cluster. This platform never talks
to the Kubernetes API directly; it only writes and commits files.

## `platform.yaml` (optional, lives in the app repo)

```yaml
port: 4000
replicas: 3
public: true
resources:
  cpuRequest: "150m"
  memoryRequest: "192Mi"
  cpuLimit: "750m"
  memoryLimit: "768Mi"
env:
  - name: LOG_LEVEL, value: info
  - name: DATABASE_URL, secretRef: sample-app-secrets/database-url
```

Every field is optional — omitted fields fall back to platform defaults
(2 replicas, conservative resource requests/limits, `public: false`).

## What's deliberately out of scope here

- **Actual `docker build`/registry push** — wrap Kaniko or BuildKit in CI; not reimplemented here.
- **Actual ArgoCD/Flux install or sync** — this repo produces the `Application` resource and the GitOps commits; reconciliation is ArgoCD/Flux's job, untouched.
- **AI risk review, health watching, auto-rollback** — weeks 7–10. The manifest/Dockerfile generation here is intentionally template-based (not LLM-generated) so it's deterministic; the AI layer sits on top, reviewing the diff this pipeline produces before/after it lands.

## Testing

```bash
go build ./...
go vet ./...
```

Package-level behavior is best verified by running `cmd/pipeline` against
`examples/sample-app` as shown above and inspecting the generated files —
there's no mocked Kubernetes cluster involved, so the fastest feedback loop
is reading the generated Dockerfile/YAML directly.
