# CHANGE 16 — Pull Source Status Lifecycle

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `16` |
| Slug | `pull-source-status-lifecycle` |
| Title | Pull Source Status Lifecycle |
| Status | `archived` |
| Branch | `feature/16-pull-source-status-lifecycle` |

---

## Goal

Make a Source's connectivity state reflect every pull invocation, including successful quiet
polls and subprocess failures. Keep the behavior generic so event adapters with no new records and
any future pull adapter work correctly without target-specific logic.

---

## Backlog

### Backend

- [x] `B1` Add an adapter-engine outcome port and deterministic status mapping: a successful
  zero-record poll marks the Source `ok`, subprocess/spawn failures mark it `unreachable`, invalid
  adapter output marks it `error`, and successful telemetry keeps its existing record-derived
  status — _Depends on:_ —
- [x] `B2` Wire pull outcomes at the composition root to Source persistence and connectivity-alert
  evaluation, with focused regression coverage for quiet success, recovery, failure debounce and
  invalid output — _Depends on:_ B1

### Frontend

None.

### Infra

None.

### Data

None.

### Other

- [x] `T1` Replace the stale pull-failure gotcha with the implemented lifecycle and verify the
  six infraegev2 dogfood Sources from a fresh local data store without copying historical
  telemetry — _Depends on:_ B2

---

## Files

### Create / modify

~~~
docs/changes/16-pull-source-status-lifecycle.md
docs/KNOWN_GOTCHAS.md
internal/adapterengine/application/ports.go
internal/adapterengine/application/scheduler.go
internal/adapterengine/application/scheduler_test.go
cmd/server/main.go
cmd/server/main_test.go
~~~

### Do NOT touch

- Source HTTP contracts, SQLite schema or adapter NDJSON contract
- adapter-specific implementations or manifests
- provisioning, deployment, backup or target-host lifecycle concerns
- infraegev2 production services or historical sre-kit telemetry

---

## Contracts

See `docs/SPEC.md` §3–§4 and §6 and the Files list above.

---

## Gate Checks

In addition to the repository Fast Gate, prove a successful empty pull transitions a new Source
to `ok`, a failed pull transitions it to `unreachable` and participates in connectivity debounce,
invalid output transitions it to `error`, and a successful pull resolves a prior connectivity
failure without overriding a non-`ok` status already derived from emitted telemetry.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- The change was discovered by real infraegev2 dogfooding: `fail2ban-ssh` completed successfully
  with no events but remained `unreachable`. The fix belongs to the generic pull lifecycle, not to
  that adapter or target integration.
- Fresh dogfood state contains six healthy Sources and new telemetry only. Private HTTP sources
  are reached through an operator-side tunnel; Source configuration holds encrypted secret refs,
  not plaintext credentials. The existing TanStack Start dev-client hydration warning reproduced
  during browser verification and remains outside this backend-only change.

---

## Commit Message

~~~
fix(change-16): report every pull source outcome
~~~
