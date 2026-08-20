# CHANGE 06 — fail2ban-ssh adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `06` |
| Slug | `fail2ban-ssh-adapter` |
| Title | fail2ban-ssh adapter |
| Status | `archived` |
| Branch | `feature/06-fail2ban-ssh-adapter` |

---

## Goal

Deliver the `fail2ban-ssh` pull-mode adapter (SPEC.md §1.3's "optional, time-permitting" v1 item —
the last unbuilt piece of v1 scope before `M6` dogfooding): SSH into a configured host and emit
`event` lines for recent fail2ban ban/unban activity, per SPEC.md §2.2's own worked example for the
Event entity ("a fail2ban ban"). Reuses change-01's Runner/Scheduler pipeline, change-02's SSH
client pattern and `secret_ref` resolver as-is (no core changes expected) — this is adapter-only
work, third adapter proving the contract needs no changes for a third source shape (log/event
occurrences, vs. change-02's continuous metrics and change-03's discrete checks).

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/platform/secrets`: confirm change-02's resolver (walks `config_schema` for
      `"format": "secret"` fields) needs no changes for this adapter's SSH credential field — if
      the schema shape already covers it, this item is a verification-only no-op documented in
      Implementation Notes, not new code (same pattern as change-03's `B1`). — _Depends on:_ —

### Infra
- [x] `I1` `adapters/fail2ban-ssh/manifest.json` — pull-mode manifest, `emits: ["event"]`,
      `config_schema` covering `host`, `port` (default `22`), `username`, `auth_method`
      (`password` | `private_key`), a `format: "secret"` field for the password/private key
      (mirrors change-02's `host-metrics-ssh` schema shape), optional `jail` (default `""` = all
      jails), and `lookback_seconds` (default `120`) bounding how far back each poll looks for
      ban/unban lines. — _Depends on:_ —
- [x] `I2` `adapters/fail2ban-ssh/main.go` — Go binary: reads resolved config JSON from stdin,
      opens an SSH client (`golang.org/x/crypto/ssh`, reusing change-02's dial/auth pattern),
      reads fail2ban's ban/unban log lines from the last `lookback_seconds` (e.g. via a remote
      `journalctl -u fail2ban --since` or `fail2ban.log` tail/grep — pick whichever needs no
      assumptions about the remote log destination beyond fail2ban's default logging), optionally
      filtered to `jail`, parses each into an `event` line (`level: "warn"` for a ban, `"info"` for
      an unban; `message` naming the jail/IP; `labels: {jail, ip, action}`), and exits non-zero
      only on genuine adapter-level failure (SSH connect/auth failure, unparsable config) so the
      existing Runner marks the source `unreachable` (SPEC.md §4) — mirrors change-02's semantics
      (unlike change-03's uptime-http, here SSH failure to the monitored host itself is the
      condition this adapter should report as unreachable, not a target-down check status).
      — _Depends on:_ `I1`
- [x] `I3` `adapters/fail2ban-ssh/main_test.go` covering: ban/unban log-line parsing from fixture
      text (varying fail2ban log formats/timestamps), NDJSON `event` line shape, and the
      connection/auth-failure path against an in-process fake SSH server (`golang.org/x/crypto/ssh`
      server mode, same technique as change-02's `I3`) rather than a real host. — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/fail2ban-ssh/manifest.json
adapters/fail2ban-ssh/main.go
adapters/fail2ban-ssh/main_test.go
Dockerfile
~~~

### Do NOT touch
- `internal/alertrouter/**` — no alert-rule/notification-channel changes; an architect wanting
  alerts on fail2ban activity configures an `alert_rules` row (`condition: status_is` doesn't apply
  to events — out of scope; event-triggered alert rules aren't part of this change or SPEC's
  current `alert_rules` shape).
- `internal/platform/wshub`, `internal/telemetry/**`, any `/api/stream` handling — this adapter
  only emits `event` lines through the existing Ingest pipeline, unchanged.
- Any `web/src/{pages,widgets,features,entities}` content — backend/adapter only, no UI work.
- `adapters/host-metrics-ssh`, `adapters/uptime-http` — other changes' adapters, not touched here.
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
# echo '{"host":"<test-vps>","port":22,"username":"<user>","auth_method":"private_key","secret":"<resolved-key-pem>","lookback_seconds":120}' \
#   | go run ./adapters/fail2ban-ssh
# expected: zero or more NDJSON `event` lines (one per recent ban/unban), exit 0. Real-VPS run is
# manual/architect-driven since it needs a live reachable host with fail2ban installed; automated
# tests (I3) use the fake in-process SSH server instead.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- `B1` verified as a no-op: `internal/platform/secrets.ResolveConfig` walks `config_schema`
  generically by field name + `"format": "secret"`, so `fail2ban-ssh`'s `secret` field is resolved
  with zero changes to that package.
- Chose to tail and parse `/var/log/fail2ban.log` (fail2ban's own default `logtarget`) rather than
  `journalctl`, since journal capture depends on `logtarget` being set to syslog/journal, which
  isn't fail2ban's out-of-the-box default — matches the Backlog note to avoid assumptions beyond
  fail2ban's default logging.
- All time-window and `jail` filtering happens in Go after a plain remote `tail -n 2000`, not via
  remote `awk`/`date`, so the ban/unban parsing logic (`parseLog`/`filterEvents`) is testable
  against fixture text without depending on remote shell quirks — this is what `I3` exercises
  directly, on top of the fake-SSH-server connect/auth-failure path.
- The wire `timestamp` field is the log line's own parsed timestamp (adapter-local, best-effort);
  the core always re-stamps the persisted `ts` at ingest time regardless
  (`internal/adapterengine/application/runner.go`), so this only affects what's visible in the
  NDJSON line itself, not what's stored.

---

## Commit Message

```
feat(change-06): fail2ban-ssh adapter — SSH ban/unban events
```
