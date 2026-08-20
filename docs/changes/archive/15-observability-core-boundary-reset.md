# CHANGE 15 — Observability Core Boundary Reset

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `15` |
| Slug | `observability-core-boundary-reset` |
| Title | Observability Core Boundary Reset |
| Status | `archived` |
| Branch | `feature/15-observability-core-boundary-reset` |

---

## Goal

Restore sre-kit as a universal read-only observability core. Remove the experimental Host and
provisioning product surface, SSH deployment credentials and Compose presets without deleting
legacy SQLite data. Preserve every adapter, Source, telemetry, alert and notification behavior.

---

## Backlog

### Data

- [x] `D1` Preserve migration `0003` and prove an upgraded database containing legacy `hosts`, `provisioning_runs` and `sources.host_id` still opens without any destructive cleanup — _Depends on:_ —

### Backend

- [x] `B1` Remove Host/provisioner modules, composition-root wiring and HTTP/OpenAPI routes so the runtime has no write-capable target lifecycle or deployment-credential path — _Depends on:_ D1
- [x] `B2` Regenerate the API contract and add focused regression evidence that core Source/adapter/telemetry routes remain available while retired routes are absent — _Depends on:_ B1

### Frontend

- [x] `F1` Remove Hosts, presets, provisioning-run entities/features/widgets/page/route and navigation; retain the existing dashboard, Sources, detail and Notifications journeys — _Depends on:_ B2
- [x] `F2` Regenerate the route tree and verify there is no dead import, query or visible deployment action — _Depends on:_ F1

### Infra

- [x] `I1` Remove deployment presets and preset runtime configuration, then correct Dockerfile, README and STACK so API/adapter packaging and the separately built UI are described truthfully — _Depends on:_ B1

### Other

- [x] `T1` Synchronize SPEC, STACK and KNOWN_GOTCHAS with the observational trust boundary, inert legacy schema policy and the linked infraegev2 Change 31 handoff — _Depends on:_ B2, F2, I1

---

## Files

### Create / modify

~~~
docs/SPEC.md
docs/STACK.md
docs/KNOWN_GOTCHAS.md
README.md
docs/changes/15-observability-core-boundary-reset.md
cmd/server/main.go
scripts/verify-core-boundary.mjs
internal/platform/config/config.go
internal/platform/db/**
internal/hosts/** (delete)
internal/provisioner/** (delete)
presets/** (delete)
contracts/openapi.json
web/src/entities/{host,preset,provisioning-run}/** (delete)
web/src/features/add-host-form/** (delete)
web/src/widgets/{add-host-drawer,deploy-drawer}/** (delete)
web/src/pages/hosts/** (delete)
web/src/routes/_authenticated/hosts.tsx (delete)
web/src/routeTree.gen.ts
web/src/widgets/app-shell/**
web/tests/**
Dockerfile
~~~

### Do NOT touch

- adapter executables/manifests or the Metric/Check/Event/Alert wire contract
- Source, telemetry, alert-router and notification behavior except contract regeneration fallout
- legacy migration `0003` or production/user database contents
- infraegev2 runtime or production VPS state from this repository
- Projects, push ingress, automation tokens or adapter SDK work reserved for later changes

---

## Contracts

See `docs/SPEC.md` §3–§4 and §12 and the Files list above.

---

## Gate Checks

In addition to the repository Fast Gate, verify an upgraded legacy database opens without data
deletion, retired endpoints are absent, existing source/telemetry endpoints still work, generated
API types have no Host/provisioning shapes, and the authenticated UI exposes no deployment route.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Legacy schema is intentionally retained but inert. Removing tables or columns is a separate
  destructive migration requiring an inventory and explicit architect approval.
- Playwright verified the authenticated navigation and the retired `/hosts` 404. The Vite dev
  runtime still reports a TanStack Start hydration mismatch around its injected dev-client script;
  production build, type-check and lint pass, so this pre-existing dev-runtime warning remains a
  separate follow-up rather than widening the boundary-reset scope.
- Backend lint was not runnable because `golangci-lint` is not installed in the environment; the
  affected backend was instead covered by `go test ./... -short` and `git diff --check`.

---

## Commit Message

```
refactor(change-15): restore observability core boundary
```
