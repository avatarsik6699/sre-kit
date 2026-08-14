# CHANGE 07 — source secret_ref fix

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `07` |
| Slug | `source-secret-ref-fix` |
| Title | source secret_ref fix |
| Status | `active` |
| Branch | `feature/07-source-secret-ref-fix` |

---

## Goal

Fix a live bug found while dogfooding change-06 against a real VPS: creating a source through
`POST /api/sources` never converts `config_schema` fields marked `"format": "secret"` into a
`secret_ref` — it persists whatever JSON it's given verbatim, including the raw plaintext
password/private key. This breaks `docs/SPEC.md` §3's "`config_json` only ever stores a
`secret_ref`, never a plaintext value" rule, and breaks the adapter at spawn time: confirmed live
as `adapterengine: source ...: resolve config secrets: secrets: resolve field "secret": secrets:
not found`. `internal/alertrouter/application/service.go`'s `CreateChannel` already does this
correctly (`secrets.Store.Put(botToken)` before persisting) — `sources` needs the same treatment.
Also folds in a related, lower-severity finding from the same live session: metric/event-only
sources never show `last_status: "ok"` even while actively ingesting real data, because
`telemetry/application.Service` only passes a non-empty status to `MarkSeen` for check-type
ingests — making a healthy source visually indistinguishable from a broken one in the UI.

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/sources/application/service.go`: add two narrow ports (mirroring
      alertrouter's pattern, not a direct `internal/platform/secrets` import) — a `SecretsPutter`
      (`Put(value string) (string, error)`) and an `AdapterConfigSchemaLookup`
      (`ConfigSchema(ctx, adapterName string) (json.RawMessage, error)`). In `Create`, before
      persisting: parse the schema's `properties` for `"format": "secret"` field names, and for
      each present in `configJSON` as a non-empty string, call `Put` and replace the field's value
      with the returned ref before storing. Unknown adapter name / lookup failure should not block
      creation of a source referencing an as-yet-uninstalled adapter — log/skip resolution in that
      case (mirrors `reconcileSchedule`'s own "unknown/uninstalled adapter" tolerance in
      `cmd/server/main.go`). — _Depends on:_ —
- [x] `B2` `internal/sources/application/service.go`'s `Update`: same resolution as `B1`, but only
      call `Put` for a secret field whose incoming value differs from the value already stored at
      that key in the source's current `config_json` — an unchanged field (already holding a
      previously-issued ref, round-tripped by a future edit UI) must not be re-wrapped into a
      second ref pointing at the ref string itself. — _Depends on:_ `B1`
- [x] `B3` `internal/telemetry/application/service.go`: metric/event ingest paths call
      `markSeen(ctx, sourceID, "")`, so a source's `last_status` is stuck at the `Create`-time
      default `"unreachable"` forever even while it's actively and successfully ingesting — a
      healthy metric/event-only source is currently indistinguishable from a broken one in
      `/api/sources`/the UI. Pass a non-empty status (e.g. `"ok"`) on successful metric/event
      ingest, analogous to `sourceStatusForCheck`'s existing check-type mapping. Confirm this
      doesn't fight the adapter engine's own error-path status writes (`unreachable`/`error` on
      spawn/auth failure) — ingest-driven `"ok"` should only ever move a source *out* of
      `unreachable`/`error`, never mask a genuine current failure. — _Depends on:_ —

### Infra
- [x] `I1` `cmd/server/main.go`: wire `secretsStore` and an `AdapterConfigSchemaLookup`
      implementation (reuses `adapterengineapp.ListInstalled(cfg.AdaptersDir)`, same lookup
      `reconcileSchedule` already does by adapter name) into `sourcesapp.NewService(...)`.
      — _Depends on:_ `B1`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
internal/sources/application/service.go
internal/sources/application/service_test.go
internal/telemetry/application/service.go
internal/telemetry/application/service_test.go
cmd/server/main.go
~~~

### Do NOT touch
- `web/src/features/add-source-form/**`, `web/src/widgets/add-source-drawer/**` — the frontend
  already sends the raw secret value in `config` as the correct input; this change is entirely
  backend-side (the API must not require a client-side change to be correct).
- `internal/alertrouter/**` — already implements the correct `secrets.Put`-before-persist pattern;
  not touched, only referenced as the model to follow.
- `internal/platform/secrets/**` — reused as-is, `Store.Put`/`ResolveConfig` need no changes.
- `adapters/**` — adapter binaries are unaffected; this is a core-side persistence bug.

---

## Contracts

See `docs/SPEC.md` §3 (`config_json` / `secret_ref` model) and §4, and the Files list above. Do
not hand-copy schema/endpoint/type details into this file.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](./STACK.md) — this section only records
> change-specific overrides.

```bash
# Manual smoke override (live-verified once already during the bug's discovery — re-run after the
# fix as the definitive check):
#   1. POST /api/sources with a host-metrics-ssh or fail2ban-ssh config containing a real
#      plaintext "secret" value.
#   2. GET /api/sources and confirm the returned config's "secret" field is a UUID-shaped ref, not
#      the plaintext value.
#   3. Confirm no "secrets: resolve field ... not found" line appears in server logs on the next
#      scheduler tick, and that metrics/events actually land in /api/metrics or /api/events.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- `B1`/`B2` used a functional-options `Option`/`With...` pattern on `sourcesapp.NewService`
  (mirroring `telemetryapp`'s existing `WithPublisher`/`WithSourceStatusUpdater` style) rather than
  widening the constructor's positional signature — keeps every existing bare
  `NewService(repo)` call site (tests, etc.) compiling unchanged.
- `B3`: no interlock was needed against the adapter engine's `unreachable`/`error` writes. Those
  only ever get written on a spawn/auth failure, which by construction means no successful ingest
  happens in that run — so `markSeen(ctx, sourceID, "ok")` on a successful ingest can never race
  with or mask a same-tick failure; the two paths are mutually exclusive per adapter invocation.
- Live-verified end-to-end against a real VPS (not just unit tests): before the fix, creating a
  `host-metrics-ssh` source via `POST /api/sources` with a real plaintext private key reproduced
  the exact reported failure (`secrets: resolve field "secret": secrets: not found`, source stuck
  `unreachable`); after the fix, the same request returns a UUID `secret_ref` in `config`, the
  scheduler resolves it with no error, and `last_status` correctly flips to `"ok"` once metrics
  land.

---

## Commit Message

```
fix(change-07): resolve source secrets to refs on create/update, surface ingest-driven ok status
```
