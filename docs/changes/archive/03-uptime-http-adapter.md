# CHANGE 03 — uptime-http adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `03` |
| Slug | `uptime-http-adapter` |
| Title | uptime-http adapter |
| Status | `archived` |
| Branch | `feature/03-uptime-http-adapter` |

---

## Goal

Deliver the `uptime-http` pull-mode adapter (SPEC.md §9 `M3`): probe a configured HTTP(S) or TCP
endpoint and emit a `check` (`ok`/`warn`/`critical`) plus, for HTTPS targets, a TLS-expiry check
derived from the certificate's `NotAfter` date. This is the second adapter against the existing
Runner/Scheduler pipeline (change-01) and the existing `secret_ref` resolver (change-02, reused
as-is for optional Basic-Auth credentials on protected endpoints) — success for `M3` per SPEC.md
§1.2 means both adapters run concurrently with zero contract change.

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/platform/secrets`: confirm change-02's resolver (walks `config_schema` for
      `"format": "secret"` fields) needs no changes for an optional Basic-Auth
      username/password pair — if the schema shape already covers it, this item is a
      verification-only no-op documented in Implementation Notes, not new code. — _Depends on:_ —

### Infra
- [x] `I1` `adapters/uptime-http/manifest.json` — pull-mode manifest, `emits: ["check"]`,
      `config_schema` covering `url` (http/https/tcp target), `method` (default `GET`),
      `timeout_seconds` (default `10`), `expect_status` (default `200`), optional
      `basic_auth_secret` (`format: "secret"`), and `tls_expiry_warn_days` (default `14`,
      only meaningful for `https`). — _Depends on:_ —
- [x] `I2` `adapters/uptime-http/main.go` — Go binary: reads resolved config JSON from stdin,
      probes the target (HTTP(S) via `net/http` with the configured method/timeout, or a raw TCP
      dial for `tcp://` targets), and emits one NDJSON `check` line named `uptime.http` (or
      `uptime.tcp`) with `status: ok|critical` based on `expect_status`/dial success. For
      `https` targets, also emits a second `check` line named `tls.cert_expiry` with `status: ok`
      (more than `tls_expiry_warn_days` remaining), `warn` (within that window), or `critical`
      (already expired), carrying the computed days-remaining in `meta`. Exits non-zero only on a
      genuine adapter-level failure (unparsable config, missing `url`) — see Implementation Notes
      for why connection/DNS/TLS failures against the *target* are reported as a normal `ok` exit
      with a `critical`-status check line instead. — _Depends on:_ `I1`
- [x] `I3` `adapters/uptime-http/main_test.go` covering: status-code-to-check-status mapping,
      TLS-expiry-days-to-status mapping (fixture certs at various expiry offsets), and NDJSON line
      shape for both `uptime.*` and `tls.cert_expiry`. Use an in-process `httptest.Server`
      (plain and TLS) rather than a real remote host. — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/uptime-http/manifest.json
adapters/uptime-http/main.go
adapters/uptime-http/main_test.go
Dockerfile
~~~

### Do NOT touch
- `internal/alerts` — still deferred to `M5`; no Go code in this change reads or writes
  `alerts`/`alert_rules`.
- `internal/platform/wshub` and any `/api/stream` handler — still deferred to `M4`.
- Any `web/src/{pages,widgets,features,entities}` content — `M3` is backend/adapter only, no UI
  work.
- `adapters/host-metrics-ssh` — change-02's adapter, not touched by this change.
- `internal/platform/secrets` — reused, not modified, unless `B1`'s verification finds a real gap.

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
# echo '{"url":"https://example.com","method":"GET","timeout_seconds":10,"expect_status":200,"tls_expiry_warn_days":14}' \
#   | go run ./adapters/uptime-http
# expected: two NDJSON lines on stdout (uptime.http, tls.cert_expiry), exit 0.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- `I2` deliberately deviates from the original Backlog wording ("exit non-zero on transport-level
  failure e.g. DNS resolution failure"): every target-reachability outcome — DNS failure,
  connection refused, timeout, TLS handshake/verification failure, wrong status code — is reported
  as a normal exit-0 `critical`-status check line, not a non-zero process exit. A down/unreachable
  *target* is exactly the condition this adapter exists to observe; exiting non-zero would instead
  mark the whole *source* `unreachable` (docs/SPEC.md §6), which has separate debounced alert
  semantics and would prevent an alert rule targeting `uptime.http`/`uptime.tcp`/`tls.cert_expiry`
  (`condition: status_is`) from ever firing while the target is down. Non-zero exit is now reserved
  for genuine adapter-level failures (unparsable config, missing `url`, unsupported scheme).
- `tls.cert_expiry` is only emitted when the HTTP client completes a *verified* TLS handshake
  (`resp.TLS` populated) — a handshake/verification failure is reported solely via `uptime.http`'s
  `critical` status with the error in `meta`, rather than re-dialing with `InsecureSkipVerify` to
  force cert data out of an untrusted connection. Keeps the production client's TLS verification
  behavior standard/secure by default; `TestHTTPCheckLine_TLSState` verifies the populated path via
  `httptest.NewTLSServer` + its own trusting `server.Client()`, not by weakening verification.
- `B1`: change-02's `secrets.ResolveConfig` walks `config_schema` generically for any property
  marked `"format": "secret"` and swaps that field's `secret_ref` for plaintext — no adapter-name
  assumptions. `uptime-http`'s `basic_auth_secret` field (holding a resolved `"username:password"`
  string) is covered with zero code changes; confirmed by reading `internal/platform/secrets/resolve.go`
  rather than by new tests.
- `Dockerfile`: added a third build/copy stage pair for `adapters/uptime-http`, mirroring the
  existing `stub`/`host-metrics-ssh` entries.

---

## Commit Message

```
feat(change-03): uptime-http adapter — HTTP/TCP check + TLS expiry
```
