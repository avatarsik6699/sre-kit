# CHANGE 10 — beszel-api auth collection fix

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `10` |
| Slug | `beszel-auth-collection-fix` |
| Title | beszel-api auth collection fix |
| Status | `active` |
| Branch | `feature/10-beszel-auth-collection-fix` |

---

## Goal

Fix `beszel-api`'s `authenticate()` (shipped in change-09, commit `4deea26` on `main`), which
hardcodes the PocketBase auth path as `/api/collections/users/auth-with-password`. Live-verified
against the real reference VPS's Beszel instance (v0.18.7) that this is wrong: Beszel's CLI
(`beszel superuser upsert`, the only account-provisioning path without its web UI) creates accounts
in PocketBase's built-in `_superusers` collection, not `users` — confirmed a CLI-provisioned
account gets a `400 "Failed to authenticate."` against `users` but authenticates successfully
against `_superusers`, and that token correctly reads `system_stats`/`container_stats` (response
shapes already match `main.go`'s existing structs — no other part of the adapter is wrong).

---

## Design References

<!-- none provided -->

---

## Backlog

### Infra
- [x] `I1` `adapters/beszel-api/manifest.json` — add `auth_collection` to `config_schema`
      (`type: string`, `default: "_superusers"`), describing it as the PocketBase collection to
      authenticate against — `_superusers` for a CLI-provisioned account (the only kind
      provisionable without the Beszel web UI), or `users` for a regular dashboard account created
      through the web UI. Not `required` (defaults cover the common case). — _Depends on:_ —
- [x] `I2` `adapters/beszel-api/main.go` — add `AuthCollection string
      \`json:"auth_collection"\`` to `config`, default it to `"_superusers"` in `main()` when
      empty (same pattern as `LookbackSeconds`'s zero-value default), and use it in place of the
      hardcoded `"users"` segment in `authenticate()`'s request URL. — _Depends on:_ `I1`
- [x] `I3` `adapters/beszel-api/main_test.go` — update `TestAuthenticate_Success` /
      `TestAuthenticate_RejectedCredentials` and the `fakePocketBase` helper to route on the
      collection segment of the request path instead of assuming `users`, and add a case covering
      an explicit non-default `auth_collection` value alongside the default `_superusers` path.
      — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/beszel-api/manifest.json
adapters/beszel-api/main.go
adapters/beszel-api/main_test.go
~~~

### Do NOT touch
- Anything outside `adapters/beszel-api/` — this is a scoped fix to one adapter's auth request, not
  a broader change. `system_stats`/`container_stats` parsing, container-metric labeling, and the
  freshness/lookback logic are already correct (live-verified during `/plan`) and untouched.

---

## Contracts

See `docs/SPEC.md` §3–§4 (and §5–§7 where relevant) and the Files list above. Do not hand-copy the
schema, endpoints, types, or env vars into this file — the codebase and `SPEC.md` are the source
of truth; this file only tracks what to build and what's left.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](./STACK.md) — this section only records
> change-specific overrides.

```bash
# Optional change-specific smoke override
# echo '{"base_url":"http://10.77.0.1:8090","system_id":"<real-system-id>","email":"sre-kit@internal.local","password":"<real-password>","auth_collection":"_superusers","lookback_seconds":300}' \
#   | go run ./adapters/beszel-api
# expected: NDJSON `metric` lines (host + per-container), exit 0. Real credentials/system_id exist
# on the reference VPS from change-09's /plan provisioning step — reachable over WireGuard via an
# SSH tunnel to 10.77.0.1:8090, same technique used for journal-http's smoke test.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Live end-to-end smoke test against the reference VPS's real Beszel instance now succeeds: a
  `sre-kit@internal.local` superuser was provisioned via `beszel superuser upsert` on the VPS
  (operator setup, not part of this change's code) and the adapter, built from this fix, correctly
  authenticated and emitted the full expected metric set — 4 host metrics
  (`cpu.usage_percent`/`mem.usage_percent`/`disk.usage_percent`/`load.avg_1m`) plus 2 metrics for
  each of 8 running containers, all with correct `container` labels, exit 0. This closes the
  live-verification gap change-09 left open.

---

## Commit Message

```
fix(change-10): beszel-api authenticates against the correct PocketBase collection
```
