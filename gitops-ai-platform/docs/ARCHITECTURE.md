# AI-Powered GitOps Deployment Platform — Architecture & Roadmap

## Vision
Developer pushes code to a repo → the platform containerizes it, generates Kubernetes manifests, reviews the change for risk, and hands off to ArgoCD/Flux for reconciliation → then watches the rollout and can auto-rollback with a root-cause explanation.

## Design principles
- **Don't reinvent reconciliation.** ArgoCD/Flux are mature, boring, and trusted. Sit on top of them, don't replace them.
- **AI where it adds judgment, not glue code.** Containerization heuristics, manifest defaults, and risk scoring are where an LLM earns its keep — not YAML templating that a script could do.
- **Every AI action is a diff, not a mutation.** The AI never applies directly to the cluster. It writes to Git; GitOps tooling applies. This gives you audit trail, rollback-via-revert, and a human approval gate for free.
- **Fail closed.** If the AI can't confidently generate/review something, it opens a PR for human review instead of guessing.

---

## Pipeline

```
git push
   │
   ▼
[1] Detect Service ──────────► language, framework, entrypoint, port
   │
   ▼
[2] Containerize ────────────► generate/validate Dockerfile
   │
   ▼
[3] Build & Push ────────────► image → registry (existing CI: Buildkit/Kaniko)
   │
   ▼
[4] Generate Manifests ──────► Deployment/Service/Ingress + resource limits
   │
   ▼
[5] AI Risk Review ──────────► score diff vs cluster state, policy checks
   │             │
   │        risk too high?
   │             │
   │             ▼
   │        open PR for human approval (don't auto-merge)
   ▼
[6] Commit to GitOps Repo ───► ArgoCD/Flux syncs (existing, untouched)
   │
   ▼
[7] Post-Deploy Watch ───────► pod health, error rate, crash loops
   │
   ▼
   healthy? done. unhealthy? ─► auto-revert commit + open issue w/ root cause
```

---

## Component breakdown

| # | Component | What it does | AI involved? |
|---|---|---|---|
| 1 | **detector** | Inspects repo (package.json, go.mod, requirements.txt, etc.) to identify language/framework/port | No — deterministic |
| 2 | **containerizer** | Generates a Dockerfile; falls back to Cloud Native Buildpacks for well-known stacks, AI only for ambiguous/custom cases | Selective |
| 3 | **build-runner** | Wraps Kaniko or BuildKit to build & push image — don't build this yourself | No |
| 4 | **manifest-generator** | Produces K8s YAML from image + a lightweight `platform.yaml` (port, env, replicas, secrets refs) | Yes — fills defaults, resource sizing |
| 5 | **risk-reviewer** | Diffs proposed manifest against live cluster state (via K8s API/ArgoCD API), flags misconfig, policy violations, blast radius | Yes — core differentiator |
| 6 | **gitops-writer** | Commits generated manifests to the GitOps repo (separate from app repo — standard GitOps pattern) | No |
| 7 | **health-watcher** | Controller or CronJob polling rollout status, pod events, error/crash metrics post-sync | Yes — root cause synthesis |
| 8 | **web/CLI** | Dashboard: pending risk reviews, deployment history, rollback button | No |

---

## Where the AI (Gemini API) plugs in

Keep AI calls narrow and structured — never "free chat" against your cluster:

- **Manifest generation**: structured JSON-mode prompt → `{resources: {...}, replicas: N, probes: {...}}`, merged into a manifest template. Never let the model free-write raw YAML that gets applied.
- **Risk review**: prompt = (proposed manifest diff + relevant cluster state + org policy rules) → structured output `{risk_score, findings: [{severity, message, rule}]}`. This is deterministic-checkable — pair it with OPA/Kyverno for hard policy rules, and use the LLM for the fuzzier "does this look right" layer (e.g., missing resource limits, oddly large replica jump, exposed port that wasn't there before).
- **Root cause on failure**: prompt = (pod events + logs excerpt + the diff that caused it) → plain-English explanation + suggested fix, attached to the auto-filed issue.

**Guardrail:** always validate AI-generated YAML against the K8s OpenAPI schema before writing to Git. Never trust generated YAML blindly.

---

## Tech stack

| Layer | Choice | Why |
|---|---|---|
| Control-plane services | **Go** | Matches ArgoCD/Flux/K8s client-go ecosystem; easy to ship as a K8s controller later |
| AI orchestration layer | **Go** or a small **Python** sidecar | Go if you want one language; Python only if you want LangChain-style prompt tooling — Gemini API is simple enough that raw Go HTTP calls are fine |
| Risk policy engine | **OPA (Rego)** or **Kyverno** | Don't hand-roll policy rules — use the K8s-native standard |
| GitOps reconciliation | **ArgoCD** (or Flux) | Untouched, existing — you integrate, not replace |
| Web dashboard | **React + TypeScript** | Standard, good component ecosystem for a deploy-history/risk-review UI |
| Storage | **Postgres** (deployment history, risk review records) | Simple, well understood |
| CI trigger | **GitHub Actions / GitLab CI** webhook → your API | Don't build your own CI runner |

---

## Repo structure (suggested)

```
gitops-ai-platform/
├── cmd/
│   ├── detector/
│   ├── manifest-generator/
│   ├── risk-reviewer/
│   └── health-watcher/
├── pkg/
│   ├── ai/              # Gemini client, prompt templates, structured-output parsing
│   ├── k8s/              # client-go wrappers, schema validation
│   ├── gitops/            # commit-to-repo logic
│   └── policy/           # OPA/Kyverno integration
├── web/                  # React dashboard
├── deploy/
│   ├── helm/              # install the platform itself via Helm
│   └── manifests/
├── docs/
└── examples/
    └── sample-app/        # end-to-end demo repo
```

---

## MVP milestones

1. **Week 1–2**: `detector` + `containerizer` — push a repo, get a working Dockerfile + built image
2. **Week 3–4**: `manifest-generator` — image → deployable K8s manifest with sane defaults
3. **Week 5–6**: `gitops-writer` + ArgoCD integration — manifest lands in cluster via existing GitOps flow
4. **Week 7–8**: `risk-reviewer` v1 — OPA policy checks + Gemini-based fuzzy review, posts findings as a PR comment
5. **Week 9–10**: `health-watcher` — post-deploy monitoring + auto-revert + root-cause issue filing
6. **Week 11+**: dashboard, docs, sample app, polish for public release=

---
