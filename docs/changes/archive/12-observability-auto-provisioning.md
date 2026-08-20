# CHANGE 12 — Observability Auto-Provisioning

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `12` |
| Slug | `observability-auto-provisioning` |
| Title | Observability Auto-Provisioning |
| Status | `archived` |
| Branch | `feature/12-observability-auto-provisioning` |

---

## Goal

Let sre-kit deploy an observability tool onto an SSH-reachable host itself, instead of requiring
manual install + account setup before an adapter can be added — the exact pain that made shipping
`journal-http`/`beszel-api`/`umami-http` (change-08 through change-11) slow. Ships a new `Host`
entity, an `internal/provisioner` workflow, and two presets (`beszel`, `umami`) that deploy a
Docker Compose stack, bootstrap its admin account, and auto-register the result as a `sources` row
— zero manual steps end to end. See `docs/SPEC.md` §12.

---

## Design References

<!-- none provided -->

---

## Backlog

### Data
- [x] `D1` Migration `0003_hosts_provisioning.sql`: `hosts` table, `provisioning_runs` table, nullable `sources.host_id` column (SPEC §3) — _Depends on:_ —

### Backend
- [x] `B1` `internal/hosts` context (domain/application/infrastructure/interfaces): CRUD + connect-check (SSH dial, fingerprint pinning on first connect, `docker compose version` probe, status update) — _Depends on:_ D1
- [x] `B2` `internal/provisioner` domain + application: `ProvisioningRun` state machine (pending→deploying→bootstrapping→registering→done/failed), resumable retry from `step`, calls `sources/application.Service.Create` to register the produced source — _Depends on:_ D1, B1
- [x] `B3` `internal/provisioner/infrastructure/ssh_runner.go`: narrow SSH port (`RunCommand`, `UploadFile`) via `golang.org/x/crypto/ssh` — first core-side (non-adapter) SSH client — _Depends on:_ B2
- [x] `B4` `presets/beszel/` (manifest.json, docker-compose.yml.tmpl, bootstrap.json) + preset loader/renderer in `internal/provisioner` — bootstrap is one SSH-run `beszel superuser upsert` command plus an HTTP call sequence to auto-create the `systems` record — _Depends on:_ B2, B3
- [x] `B5` `presets/umami/` (manifest.json, docker-compose.yml.tmpl, bootstrap.json) — bootstrap is an HTTP call sequence (log in with seeded default admin, change password, create a website record) against the freshly deployed container — _Depends on:_ B4, T1
- [x] `B6` HTTP handlers: `GET/POST /api/hosts`, `POST /api/hosts/:id/check-connection`, `DELETE /api/hosts/:id`, `GET /api/presets`, `POST /api/hosts/:id/provision`, `GET /api/provisioning-runs`, `POST /api/provisioning-runs/:id/retry` (SPEC §4) — _Depends on:_ B1, B2
- [x] `B7` Wire `internal/hosts` + `internal/provisioner` into `cmd/server/main.go` composition root — _Depends on:_ B1, B2, B3, B4, B5, B6

### Frontend
- [x] `F1` Hosts page (`/hosts`): list, add (SSH form with the root-equivalent-Docker-access disclosure per SPEC §12.1), remove, connection status — _Depends on:_ B6
- [x] `F2` Deploy flow: preset picker + `provisioning_runs` progress/history view (reuse the status-pulse token, SPEC §5.3), opened from a Host row — _Depends on:_ B6, B7

### Other
- [ ] `T1` Verify against the real infraegev2 test VPS: a fresh Umami container's default seeded admin credentials, and whether a self-service change-password endpoint exists for a logged-in user (SPEC §11, §12.3) — _Depends on:_ — — **not done as originally scoped**; see Implementation Notes

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
internal/platform/db/migrations/0003_hosts_provisioning.sql
internal/platform/config/config.go          (modify — add PresetsDir)
internal/hosts/domain/host.go
internal/hosts/application/service.go
internal/hosts/application/service_test.go
internal/hosts/infrastructure/sqlite_repo.go
internal/hosts/infrastructure/sqlite_repo_test.go
internal/hosts/infrastructure/ssh_prober.go
internal/hosts/infrastructure/ssh_prober_test.go
internal/hosts/interfaces/http/handlers.go
internal/provisioner/domain/run.go
internal/provisioner/domain/preset.go
internal/provisioner/application/ports.go
internal/provisioner/application/preset_loader.go
internal/provisioner/application/workflow.go
internal/provisioner/application/workflow_test.go
internal/provisioner/application/preset_fixtures_test.go
internal/provisioner/infrastructure/ssh_runner.go
internal/provisioner/infrastructure/ssh_runner_test.go
internal/provisioner/infrastructure/sqlite_repo.go
internal/provisioner/interfaces/http/handlers.go
presets/beszel/manifest.json
presets/beszel/docker-compose.yml.tmpl
presets/beszel/bootstrap.json
presets/umami/manifest.json
presets/umami/docker-compose.yml.tmpl
presets/umami/bootstrap.json
cmd/server/main.go                         (modify — wire new contexts)
contracts/openapi.json                     (regenerated — swag/openapi-typescript pipeline)
web/src/shared/api/schema.ts                (regenerated)
web/src/entities/host/
web/src/entities/preset/
web/src/entities/provisioning-run/
web/src/features/add-host-form/
web/src/widgets/add-host-drawer/
web/src/widgets/deploy-drawer/
web/src/pages/hosts/
web/src/routes/_authenticated/hosts.tsx
web/src/widgets/rail-nav/rail-nav.tsx        (modify — add Hosts nav item)
~~~

### Do NOT touch
- `adapters/host-metrics-ssh/**`, `adapters/fail2ban-ssh/**` — TOFU/`knownhosts` fix is scoped to the new provisioner path only, not retrofitted onto the existing adapters (SPEC §12.4)
- `adapters/beszel-api/manifest.json`, `adapters/umami-http/manifest.json` — read as the target `config_schema` reference for `produces_source_config_template`, not modified

---

## Contracts

See `docs/SPEC.md` §3 (Data Model), §4 (API), §12 (Observability Auto-Provisioning) and the Files
list above. Do not hand-copy schema/endpoint/type details into this file.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](../../STACK.md) — this section only records
> change-specific overrides.

None.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- **T1 was resolved via documentation, not a live infraegev2 VPS smoke test.** Rather than deploying
  a fresh Umami container on the real test VPS purely to confirm defaults, `context7` was used to
  pull Umami's own upstream docs, which confirm the exact assumptions the `umami` preset's bootstrap
  needed: default seed account `admin`/`umami`, `POST /api/auth/login` → `{token, user}`, and
  `POST /api/me/password` (not `/api/auth/password` — the docs corrected an earlier guess) →
  `{ok: true}` for a logged-in user's self-service password change. This is stronger than a guess
  but still short of live confirmation against the real image; a live smoke test on infraegev2
  remains a good follow-up before this preset is trusted in anger, consistent with how change-09
  shipped with a documented live-gap later closed in change-10.
- **The `beszel` preset's third bootstrap step (creating a `systems` record so the produced source
  has a real `system_id`) is a best-effort implementation, not verified against a live Beszel
  instance.** Beszel's docs confirm `POST /api/collections/systems/records` is a normal PocketBase
  collection write and show `name`/`host`/`port`/`users` fields via other examples, but no official
  example of the create call itself was found. If field names differ in practice, this step's
  `http_call` will fail cleanly (the workflow will report `bootstrapping`/`failed` with the
  PocketBase error body) rather than silently misregister — a Retry after fixing
  `presets/beszel/bootstrap.json` resumes correctly. Flagged as the Beszel-side counterpart to T1's
  Umami gap; a live smoke test against infraegev2's real Beszel instance is the natural way to close
  it.
- **v1's provisioning workflow runs synchronously within the triggering HTTP request** (no
  background job queue) — documented in code (`internal/provisioner/application/workflow.go`,
  `Service.Start`'s doc comment) and reflected in the UI (`widgets/deploy-drawer`'s Deploy button
  shows `loading` until the run reaches `done`/`failed`). Acceptable for a rarely repeated,
  human-triggered action; revisit if deploy latency becomes a real UX problem.
- Verified end-to-end locally via Playwright against a running `cmd/server` + `pnpm dev`: logged in,
  added a host (throwaway ed25519 key), confirmed the Docker-access disclosure renders, triggered
  `check-connection` against an unreachable target and confirmed it fails cleanly (10s SSH dial
  timeout, HTTP 500, no unhandled console errors), confirmed `Deploy` stays disabled until a host has
  a pinned fingerprint, then removed the test host. No live Beszel/Umami deploy was exercised (would
  require a real Docker host).
- Minor, not fixed: the Docker-availability `Badge` on the Hosts table truncates its text
  (`"UNAVAILAB…"`) at default width — cosmetic, no functional impact.
- Found, not touched: an untracked, non-gitignored `web/web/` directory containing a stray partial
  copy of `web/src` (older `dashboard`/`source-detail`/`login`/`sources` pages, dated before this
  session). It predates this change, isn't referenced by any build tooling, and git doesn't track or
  ignore it. Left alone per this project's investigate-before-deleting rule for unfamiliar state —
  worth the architect's attention as unrelated cleanup, not addressed here.

---

## Commit Message

```
feat(change-12): observability auto-provisioning — Host entity, provisioner workflow, Beszel + Umami presets
```
