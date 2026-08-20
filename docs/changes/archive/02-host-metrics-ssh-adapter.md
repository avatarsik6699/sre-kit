# CHANGE 02 — host-metrics-ssh adapter

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `02` |
| Slug | `host-metrics-ssh-adapter` |
| Title | host-metrics-ssh adapter |
| Status | `archived` |
| Branch | `feature/02-host-metrics-ssh-adapter` |

---

## Goal

Deliver the `host-metrics-ssh` pull-mode adapter (SPEC.md §9 `M2`): SSH into a configured host and
emit `cpu.usage_percent`, `mem.usage_percent`, `disk.usage_percent` metrics that land in SQLite
through the existing Runner/Scheduler pipeline built in change-01. This also closes the one real
gap that pipeline left for a credentialed adapter: `sources.config_json` only ever holds a
`secret_ref` (SPEC.md §3), but nothing today resolves that ref back to a plaintext secret before
the adapter subprocess is spawned — SSH auth needs the real password/key on adapter stdin, never
the ref.

---

## Design References

<!-- none provided -->

---

## Backlog

### Backend
- [x] `B1` `internal/platform/secrets`: add a resolver that walks an adapter manifest's
      `config_schema` for properties marked `"format": "secret"`, and for each, replaces that
      field's value (a `secret_ref`) in a source's `config_json` with the plaintext from the
      `secrets.Store`, returning the resolved JSON bytes. Errors (missing ref, decrypt failure)
      must not leak the ref or plaintext into logs. — _Depends on:_ —
- [x] `B2` Wire `cmd/server/main.go`'s `reconcileSchedule` to call `B1`'s resolver on
      `source.ConfigJSON` before building `adapterengineapp.PullJob.Config`, so the adapter
      subprocess's stdin carries the real secret while SQLite and the HTTP API
      (`GET /api/sources`) keep seeing only the ref. — _Depends on:_ `B1`

### Infra
- [x] `I1` `adapters/host-metrics-ssh/manifest.json` — pull-mode manifest, `emits: ["metric"]`,
      `config_schema` covering `host`, `port` (default `22`), `username`, `auth_method`
      (`password` | `private_key`), and a `format: "secret"` field holding the `secret_ref` for
      the password or private key. — _Depends on:_ —
- [x] `I2` `adapters/host-metrics-ssh/main.go` — Go binary: reads resolved config JSON from stdin,
      opens an SSH client (`golang.org/x/crypto/ssh`), samples CPU/RAM/disk usage on the remote
      host (e.g. via `/proc/stat`, `/proc/meminfo`, `df` over a remote exec — pick whichever needs
      no assumptions about remote tooling beyond a POSIX shell), emits NDJSON `metric` lines
      matching `contract.schema.json`, and exits non-zero on connection/auth failure so the
      existing Runner marks the source `unreachable` (SPEC.md §4). — _Depends on:_ `I1`
- [x] `I3` `adapters/host-metrics-ssh/main_test.go` (or a package split with an internal
      `sample.go`/`sample_test.go`) covering: usage-percent parsing from fixture `/proc` output,
      and NDJSON line shape. Connection/auth failure path exercised against an in-process fake SSH
      server (`golang.org/x/crypto/ssh` supports serving, not just dialing) rather than a real
      host. — _Depends on:_ `I2`
- [x] `I4` `go.mod`/`go.sum`: add `golang.org/x/crypto` for the SSH client/test server.
      — _Depends on:_ `I2`

<!-- Test execution is governed by `docs/STACK.md`'s Fast Gate (per task) and Full Gate (per ship).
     Do not duplicate that list here. -->

---

## Files

### Create / modify
~~~
adapters/host-metrics-ssh/manifest.json
adapters/host-metrics-ssh/main.go
adapters/host-metrics-ssh/main_test.go
internal/platform/secrets/resolve.go
internal/platform/secrets/resolve_test.go
cmd/server/main.go
go.mod
go.sum
Dockerfile
~~~

### Do NOT touch
- `internal/alerts` — still deferred to `M5`; no Go code in this change reads or writes
  `alerts`/`alert_rules`.
- `internal/platform/wshub` and any `/api/stream` handler — still deferred to `M4`.
- Any `web/src/{pages,widgets,features,entities}` content — `M2` is backend/adapter only, no UI
  work (test-connection UI is part of the `M4` Add-source form, not this change).
- `adapters/stub` — leave the change-01 stub adapter as-is; it stays the pull-mode reference/test
  fixture.
- `uptime-http` adapter — that's `M3`, a separate change.

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
# echo '{"host":"<test-vps>","port":22,"username":"<user>","auth_method":"private_key","secret":"<resolved-key-pem>"}' \
#   | go run ./adapters/host-metrics-ssh
# expected: three NDJSON lines on stdout (cpu.usage_percent, mem.usage_percent,
# disk.usage_percent), exit 0. Real-VPS run is manual/architect-driven since it needs a live
# reachable host; automated tests (I3) use the fake in-process SSH server instead.
```

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Host key verification is intentionally skipped in v1 (`ssh.InsecureIgnoreHostKey()`) — there's
  no `known_hosts` store or first-connection TOFU pinning yet. Logged as a `docs/KNOWN_GOTCHAS.md`
  entry rather than solved here; it's a real gap worth closing before dogfooding (`M6`) but out of
  scope for getting real metrics flowing.
- CPU usage needs a delta between two `/proc/stat` reads; `sampleScript` takes both samples in one
  remote exec (with a 1s `sleep` in between) rather than two separate SSH round-trips per pull.
- Context7 (`ctx7 library`/`docs`) has no indexed documentation for `golang.org/x/crypto/ssh`
  specifically (tried 3 queries, best matches were unrelated X.Org/`golang.org/x/term` results) —
  implemented against training-data knowledge of the client (`Dial`/`ClientConfig`/`Session`) and
  server (`NewServerConn`/`ServerConfig`/channel+request handling) APIs instead, then verified
  correctness two ways: (1) `I3`'s tests run against a real in-process fake SSH server rather than
  trusting the API shape blind, and (2) cross-checked directly against
  `pkg.go.dev/golang.org/x/crypto/ssh` afterward — confirmed idiomatic and correct, no deprecated
  calls, no signature mismatches.
- `Dockerfile` wasn't in the original Files list but needed a matching `RUN`/`COPY` pair for the
  new adapter binary, mirroring the existing `adapters/stub` entries — same shipping pattern, no
  new decision.
- `go mod tidy` also reclassified `google/uuid`, `santhosh-tekuri/jsonschema/v6`, and
  `modernc.org/sqlite` from `// indirect` to direct requires — those were already imported
  directly by earlier change-01 code; `go.mod` just hadn't been tidied since.

---

## Commit Message

```
feat(change-02): host-metrics-ssh adapter — SSH metrics + secret_ref resolution
```
