# CHANGE 08 — journal-http adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `08` |
| Slug | `journal-http-adapter` |
| Title | journal-http adapter |
| Status | `archived` |
| Branch | `feature/08-journal-http-adapter` |

---

## Goal

Deliver `journal-http`, a fourth pull-mode adapter that pulls structured log entries from any host
running `systemd-journal-gatewayd` (systemd's built-in HTTP journal export server) and emits
`event` lines — proving the Event entity's contract on a second, HTTP-native transport alongside
`fail2ban-ssh`'s SSH-tail approach (SPEC.md §2.2, §10: "only adapters to existing tools/protocols").
Generic to any self-hosted host exposing gatewayd, not tied to a specific project's infrastructure.

---

## Design References

<!-- none provided -->

---

## Backlog

### Infra
- [x] `I1` `adapters/journal-http/manifest.json` — pull-mode manifest, `emits: ["event"]`,
      `config_schema` covering `host`, `port` (default `19531`), `https` (bool, default `false`),
      optional `client_cert`/`client_key` (`format: "secret"`, PEM, for gatewayd's mTLS — it has no
      username/password auth of its own, only optional `--cert/--key/--trust` mTLS per the upstream
      manpage), optional `unit` (systemd unit name filter, default `""` = all units), optional
      `min_priority` (syslog priority 0–7, default `6` = info-and-above), and `lookback_seconds`
      (default `120`) bounding each poll's `Range: realtime=` window. — _Depends on:_ —
- [x] `I2` `adapters/journal-http/main.go` — Go binary: reads resolved config JSON from stdin,
      issues `GET /entries` against `http(s)://host:port` with `Accept: application/json` and
      `Range: realtime=<now-lookback_seconds in usec>:` (open-ended upper bound), optionally
      appending `?_SYSTEMD_UNIT=<unit>` as a journal-field match query param when `unit` is set,
      configuring the HTTP client with the client cert/key when both are present (mTLS) or plain
      HTTP/HTTPS otherwise, parsing each JSON-per-line response entry
      (`MESSAGE`, `PRIORITY`, `__REALTIME_TIMESTAMP` usec, `_SYSTEMD_UNIT`/`SYSLOG_IDENTIFIER`) into
      an `event` line (`level`: `"warn"` for priority ≤ 4, else `"info"`; `message` from `MESSAGE`;
      `labels: {unit, priority}`; timestamp from `__REALTIME_TIMESTAMP`), filtering out entries
      below `min_priority` client-side, and exiting non-zero only on genuine adapter-level failure
      (connection refused/TLS handshake failure/non-2xx response) so the Runner marks the source
      `unreachable` — mirrors `fail2ban-ssh`'s unreachable-vs-check-status semantics (SPEC.md §4).
      Stateless per run (no cursor persistence between invocations), matching the existing
      `lookback_seconds`-window precedent set by `fail2ban-ssh`. — _Depends on:_ `I1`
- [x] `I3` `adapters/journal-http/main_test.go` covering: JSON journal-entry parsing from fixture
      lines (varying `PRIORITY`/`_SYSTEMD_UNIT` presence), NDJSON `event` line shape, priority
      filtering, and the connect/non-2xx-failure path against an in-process `httptest.Server`
      (mirrors `fail2ban-ssh`'s fake-SSH-server technique, adapted to HTTP) rather than a real
      gatewayd instance. — _Depends on:_ `I2`
- [x] `I4` `Dockerfile` — add the `journal-http` adapter binary build/copy step, matching the
      existing three adapters' pattern. — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/journal-http/manifest.json
adapters/journal-http/main.go
adapters/journal-http/main_test.go
Dockerfile
~~~

### Do NOT touch
- `internal/alertrouter/**`, `internal/platform/wshub`, `internal/telemetry/**`, `/api/stream` —
  this adapter only emits `event` lines through the existing Ingest pipeline, unchanged (same
  boundary as change-06).
- Any `web/src/{pages,widgets,features,entities}` content — backend/adapter only, no UI work.
- `adapters/host-metrics-ssh`, `adapters/uptime-http`, `adapters/fail2ban-ssh` — other changes'
  adapters, not touched here.
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
# echo '{"host":"10.77.0.1","port":19531,"https":false,"lookback_seconds":21600}' \
#   | go run ./adapters/journal-http
# expected: zero or more NDJSON `event` lines (one per matching journal entry), exit 0. Real-host
# run is manual/architect-driven (needs a live gatewayd reachable over the network, e.g. via
# WireGuard on the reference VPS); automated tests (I3) use httptest.Server instead.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- `min_priority` is `*int` in `config`, not `int` — distinguishes "field absent from JSON" (nil,
  falls back to the manifest's default `6`) from an explicit `0` (most severe only), which a plain
  `int` zero-value couldn't represent.
- `filterEntries` keeps any entry whose `PRIORITY` fails to parse, rather than dropping it — a
  malformed/missing severity field must never silently swallow a real event.
- Host TLS verification uses Go's default `crypto/tls` behavior (unlike the SSH adapters' TOFU
  skip) — gatewayd's HTTPS mode implies the operator already provisioned a real certificate for it,
  there's no equivalent "first connection, no CA yet" case to accommodate.
- **Deviation from the Backlog's planned `Range: realtime=<since>:` approach**: live-verified
  against a real systemd 255 (Ubuntu 24.04) `systemd-journal-gatewayd` instance that this header is
  accepted (`200 OK`) but silently ignored — the response always starts from the beginning of the
  journal regardless of the requested time window. `Range: entries=:-N:N` (last N entries) *is*
  honored correctly, so `fetchEntries` was changed to request the last `maxEntries` (2000, mirroring
  `fail2ban-ssh`'s `logLines` tail-then-filter pattern) and time-filter client-side via the new
  `filterByLookback`, instead of relying on server-side time-range filtering. `_SYSTEMD_UNIT=`
  query-param filtering was confirmed to work correctly and is unaffected by this change.
- Full end-to-end smoke test run against the reference VPS's `systemd-journal-gatewayd` (reached via
  an SSH local port-forward to its WireGuard-only `10.77.0.1:19531` listener, since this adapter's
  dev/build machine isn't itself on that VPN): real UFW-block kernel messages, Docker/uvicorn
  request logs, and systemd unit start/stop events all came through correctly classified
  (`warn` for priority ≤ 4, e.g. the UFW block; `info` otherwise), confirming the adapter against
  live, non-synthetic data end-to-end, not just the `httptest.Server`-based unit tests.

---

## Commit Message

```
feat(change-08): journal-http adapter — systemd journal events over HTTP
```
