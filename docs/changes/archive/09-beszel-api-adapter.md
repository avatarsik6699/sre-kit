# CHANGE 09 — beszel-api adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `09` |
| Slug | `beszel-api-adapter` |
| Title | beszel-api adapter |
| Status | `archived` |
| Branch | `feature/09-beszel-api-adapter` |

---

## Goal

Deliver `beszel-api`, a fifth pull-mode adapter that pulls host and per-container metrics from a
self-hosted Beszel instance's PocketBase-backed HTTP API and emits `metric` lines — proving the
Metric entity's contract carries genuine per-container breakdown that `host-metrics-ssh`'s
SSH+/proc sampling has no visibility into at all (confirmed by reading `host-metrics-ssh/main.go`:
host-level cpu/mem/disk % only, no container awareness). Generic to any self-hosted Beszel
instance, not tied to a specific project's infrastructure.

---

## Design References

<!-- none provided -->

---

## Backlog

### Infra
- [x] `I1` `adapters/beszel-api/manifest.json` — pull-mode manifest, `emits: ["metric"]`,
      `config_schema` covering `base_url` (Beszel's PocketBase URL, e.g. `http://10.77.0.1:8090`),
      `system_id` (Beszel's internal identifier for the monitored host — description must explain
      it's found in the Beszel UI's system detail view, since an architect adding a source needs to
      look it up there), `email`/`password` as `format: "secret"` (PocketBase's
      `auth-with-password` is the only auth mode this API exposes — no long-lived API-key option),
      and `lookback_seconds` (default `120`, bounding the `system_stats`/`container_stats` query
      window via `filter`/`sort`/`perPage`, mirroring the other adapters' polling-window
      convention). — _Depends on:_ —
- [x] `I2` `adapters/beszel-api/main.go` — Go binary: reads resolved config JSON from stdin, calls
      `POST {base_url}/api/collections/users/auth-with-password` with
      `{identity: email, password}` every run (stateless, no token caching between invocations —
      same precedent as `journal-http`'s stateless design) to get a bearer token, then two
      `GET {base_url}/api/collections/{system_stats,container_stats}/records` calls with
      `Authorization: <token>`, `filter=system="<system_id>"`, `sort=-created`, `perPage` sized to
      the polling window, parses the latest `system_stats` record's `stats.{cpu,mp,dp,la[0]}` into
      `cpu.usage_percent`/`mem.usage_percent`/`disk.usage_percent`/`load.avg_1m` metric lines
      (matching `host-metrics-ssh`'s naming for the three overlapping concepts, so both adapters'
      output can share one dashboard chart), and the latest `container_stats` record's per-entry
      `{n, c, m}` into one `container.cpu_percent`/`container.mem_mib` metric pair per container
      with `labels: {container: <n>}`. Exits non-zero only on genuine adapter-level failure (auth
      rejected — PocketBase returns `400`/`401` on bad credentials; connection failure; non-2xx on
      the records calls) so the Runner marks the source `unreachable` — a `system_stats` query that
      succeeds but returns zero records (system exists in Beszel but has no data yet) is not a
      failure, it just emits no metric lines this poll. — _Depends on:_ `I1`
- [x] `I3` `adapters/beszel-api/main_test.go` covering: `system_stats`/`container_stats` JSON
      response parsing into metric lines (fixture responses matching PocketBase's list-records
      shape), the container-metrics labeling, and the auth-failure/non-2xx path against an
      in-process `httptest.Server` fake PocketBase responder (mirrors `journal-http`'s `I3`
      technique) rather than a real Beszel instance. — _Depends on:_ `I2`
- [x] `I4` `Dockerfile` — add the `beszel-api` adapter binary build/copy step, matching the existing
      four adapters' pattern. — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/beszel-api/manifest.json
adapters/beszel-api/main.go
adapters/beszel-api/main_test.go
Dockerfile
~~~

### Do NOT touch
- `internal/alertrouter/**`, `internal/platform/wshub`, `internal/telemetry/**`, `/api/stream` —
  this adapter only emits `metric` lines through the existing Ingest pipeline, unchanged (same
  boundary as changes 06/08).
- Any `web/src/{pages,widgets,features,entities}` content — backend/adapter only, no UI work.
- `adapters/host-metrics-ssh`, `adapters/uptime-http`, `adapters/fail2ban-ssh`,
  `adapters/journal-http` — other changes' adapters, not touched here.
- `internal/platform/secrets` — reused as-is via the existing generic `format: "secret"` resolver
  (proven adapter-agnostic by change-07); no changes expected.

---

## Contracts

See `docs/SPEC.md` §3–§4 (and §5–§7 where relevant) and the Files list above. Do not hand-copy the
schema, endpoints, types, or env vars into this file — the codebase and `SPEC.md` are the source
of truth; this file only tracks what to build and what's left.

---

## Gate Checks

> Fast Gate runs per task in `/work`; Full Gate and (with `--release`) Release Gate run once in
> `/ship`. Both are defined in [docs/STACK.md](../../STACK.md) — this section only records
> change-specific overrides.

```bash
# Optional change-specific smoke override
# echo '{"base_url":"http://10.77.0.1:8090","system_id":"<id>","email":"<email>","password":"<pw>","lookback_seconds":300}' \
#   | go run ./adapters/beszel-api
# expected: zero or more NDJSON `metric` lines, exit 0. Real-Beszel run is manual/architect-driven
# and needs admin credentials provisioned on the reference VPS's Beszel instance first (confirmed
# reachable at http://10.77.0.1:8090 over WireGuard during /plan, but no credentials exist there
# yet — a separate manual setup step, tracked as a note in Implementation Notes if still missing at
# ship time, not a Backlog item since it's operator setup, not code). Automated tests (I3) use
# httptest.Server instead and don't depend on this.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Only the single latest `system_stats`/`container_stats` record per poll is fetched
  (`perPage=1`, `sort=-created`) rather than the full window — matches `host-metrics-ssh`'s
  "emit current sample" semantics rather than backfilling history through the adapter. A record
  older than `lookback_seconds` is treated as stale and silently dropped (no metric lines emitted
  that poll, not a failure) via `freshTimestamp`, covering the case where Beszel has stopped
  receiving fresh data from the agent without falsely reporting the source `unreachable`.
- `container_stats`' `stats` field is a JSON array (`[{n,c,m}, ...]`), unlike `system_stats`' single
  object — both are decoded via `json.RawMessage` at the record level and type-asserted separately
  in `toNDJSON`, rather than a shared struct.
- Live smoke-test against the reference VPS's real Beszel instance is still pending: confirmed
  during `/plan` that Beszel itself is running and reachable (`10.77.0.1:8090`, health check `200`),
  but no admin/user credentials are provisioned there yet — that's a manual operator step (creating
  a Beszel account), not something this change's code can do. Automated tests (`I3`) fully cover the
  adapter's logic against a fake PocketBase `httptest.Server` and don't depend on this.

---

## Commit Message

```
feat(change-09): beszel-api adapter — host and per-container metrics via PocketBase API
```
