# CHANGE 11 — umami-http adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `11` |
| Slug | `umami-http-adapter` |
| Title | umami-http adapter |
| Status | `active` |
| Branch | `feature/11-umami-http-adapter` |

---

## Goal

Deliver `umami-http`, a sixth pull-mode adapter pulling aggregate traffic stats from a self-hosted
Umami instance's REST API and emitting `metric` lines — the web-analytics adapter SPEC.md §10
originally deferred until the Metric/Check/Event contract was proven on the first 2–3 adapters.
That condition is now satisfied by 5 shipped adapters across SSH, HTTP-log-export, and
PocketBase-REST transports, so `docs/SPEC.md` §1.3/§10 were updated in this change's `/plan` step
to move it into v1 scope. Fits the existing Metric contract as-is — no new primitive, no schema
change.

---

## Design References

<!-- none provided -->

---

## Backlog

### Infra
- [x] `I1` `adapters/umami-http/manifest.json` — pull-mode manifest, `emits: ["metric"]`,
      `config_schema` covering `base_url` (Umami's URL, e.g. `http://10.77.0.1:3001`), `website_id`
      (found in the Umami UI's website settings — description must explain this, same pattern as
      `beszel-api`'s `system_id`), `username`/`password` as `format: "secret"` (Umami's
      `/api/auth/login` is the only auth mode, no long-lived API key), and `lookback_seconds`
      (default `3600` — a shorter window would read mostly zeros for a low-traffic site, unlike the
      120s default used by the infra adapters). — _Depends on:_ —
- [x] `I2` `adapters/umami-http/main.go` — Go binary: reads resolved config JSON from stdin, calls
      `POST {base_url}/api/auth/login` with `{username, password}` every run (stateless, no token
      caching — same precedent as `journal-http`/`beszel-api`) to get a bearer token, then
      `GET {base_url}/api/websites/{website_id}/stats?startAt=<now-lookback_seconds in ms>&endAt=<now
      in ms>&unit=hour&timezone=UTC` with `Authorization: Bearer <token>`, parsing the response's
      `{pageviews, visitors, visits, bounces, totaltime}` integer fields into one metric line each
      (`analytics.pageviews`, `analytics.visitors`, `analytics.visits`, `analytics.bounces`,
      `analytics.totaltime_seconds`), all stamped with the current poll time as a snapshot for the
      lookback window — does **not** call Umami's `/pageviews` time-series endpoint (that would
      re-emit the same historical buckets on every poll, unlike `beszel-api`'s single-latest-record
      design, since `/stats` is already a pre-aggregated snapshot with no equivalent "latest
      record" to dedupe against). Exits non-zero only on genuine adapter-level failure (login
      rejected, connection failure, non-2xx `/stats` response) — an all-zero stats response (no
      traffic in the window) is not a failure. — _Depends on:_ `I1`
- [x] `I3` `adapters/umami-http/main_test.go` covering: `/stats` response parsing into metric
      lines, the all-zero-traffic case, and the login-failure/non-2xx path against an in-process
      `httptest.Server` fake Umami responder (mirrors `beszel-api`'s `I3` technique). — _Depends
      on:_ `I2`
- [x] `I4` `Dockerfile` — add the `umami-http` adapter binary build/copy step, matching the existing
      five adapters' pattern. — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/umami-http/manifest.json
adapters/umami-http/main.go
adapters/umami-http/main_test.go
Dockerfile
docs/SPEC.md
~~~

### Do NOT touch
- `internal/alertrouter/**`, `internal/platform/wshub`, `internal/telemetry/**`, `/api/stream` —
  this adapter only emits `metric` lines through the existing Ingest pipeline, unchanged (same
  boundary as prior adapter changes).
- Any `web/src/{pages,widgets,features,entities}` content — backend/adapter only, no UI work.
- `adapters/host-metrics-ssh`, `adapters/uptime-http`, `adapters/fail2ban-ssh`,
  `adapters/journal-http`, `adapters/beszel-api` — other changes' adapters, not touched here.
- `internal/platform/secrets` — reused as-is via the existing generic `format: "secret"` resolver;
  no changes expected.

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
# echo '{"base_url":"http://10.77.0.1:3001","website_id":"<id>","username":"<user>","password":"<pw>","lookback_seconds":3600}' \
#   | go run ./adapters/umami-http
# expected: 5 NDJSON `metric` lines, exit 0. Real-Umami run is manual/architect-driven and needs
# admin credentials provisioned on the reference VPS's Umami instance first (confirmed reachable at
# http://10.77.0.1:3001 over WireGuard during /plan, but no credentials exist there yet — a
# separate manual operator step, same precedent as change-09's beszel-api gap, tracked in
# Implementation Notes if still missing at ship time). Automated tests (I3) use httptest.Server
# instead and don't depend on this.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Live smoke-test against the reference VPS's real Umami instance is deferred, same as
  `beszel-api`'s original change-09 gap (closed later in change-10): Umami is confirmed running and
  reachable (`10.77.0.1:3001`, health check `200`, v3.2.0), but no admin credentials are
  provisioned there. Unlike Beszel, Umami has no CLI for account creation/password reset — a
  password reset would require a direct `UPDATE` against its Postgres `user` table, which is a
  production-auth-data mutation the architect chose to defer rather than push through right now
  (attempted mid-session but the shell-quoting/sudo mechanics made it more trouble than it was
  worth; can be revisited later the same way change-10 closed the beszel-api gap). Automated tests
  (`I3`) fully cover the adapter's logic against a fake Umami `httptest.Server` and don't depend on
  this.

---

## Commit Message

```
feat(change-11): umami-http adapter — web analytics via Umami REST API
```
