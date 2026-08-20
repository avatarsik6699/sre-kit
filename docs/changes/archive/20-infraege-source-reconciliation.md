# CHANGE 20 — infraege Source Reconciliation

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `20` |
| Slug | `infraege-source-reconciliation` |
| Title | infraege Source Reconciliation |
| Status | `active` |
| Branch | `feature/20-infraege-source-reconciliation` |

---

## Goal

Reconcile the stale local dogfood state with infraegev2's six current Source templates through
sre-kit's supported runtime contracts. Prove fresh polling, quiet success, a controlled reversible
failure/recovery journey and authenticated dashboard rendering without mutating the target VPS or
exposing credentials. Turn observed gaps into an evidence-ranked backlog instead of fixing them
off-list.

---

## Backlog

### Backend

- [x] `B1` Add source-tagged, secret-safe pull failure diagnostics so an `unreachable` outcome
  identifies its stable failure class without logging resolved adapter config or raw stderr —
  _Depends on:_ I1

### Frontend

- [x] `F1` Verify through the authenticated web UI that the Dashboard, Sources list and each
  relevant Source detail render the reconciled state and fresh telemetry without console/runtime
  errors — _Depends on:_ D2, F2
- [x] `F2` Diagnose and fix the reproduced TanStack Start development hydration mismatch before
  accepting F1 browser evidence; keep the fix generic and append any contract expansion before
  implementation — _Depends on:_ I1

### Infra

- [x] `I1` Establish a recoverable local preflight before mutation: prove process/port ownership,
  validate the configured DB and encrypted secret store, create permission-preserving snapshots,
  and inventory Source identities/statuses without reading secret values into logs — _Depends on:_ —
- [x] `I2` Verify the six infraegev2 target prerequisites read-only, including public uptime,
  public root/password SSH inputs and WireGuard-only HTTP endpoints; if target-side repair is
  required, stop and open a coordinated infraegev2 Backlog instead of mutating the VPS here —
  _Depends on:_ I1
- [x] `I3` Correct the local `.env` permission from world-readable `0644` to owner-only `0600`
  before starting a process that consumes the encrypted-store key — _Depends on:_ I1
- [x] `I4` Rotate the unrecoverable local admin hash to the architect-approved owner-only
  credential file through the existing encrypted-store contract, then prove authenticated login
  without printing or persisting plaintext elsewhere — _Depends on:_ I1, I3

### Data

- [x] `D1` Reconcile Source records through the authenticated Source API/UI to exactly one enabled
  instance for each current infraegev2 template, preserving valid encrypted secret refs, adding
  the missing uptime Source and avoiding direct SQLite writes or historical telemetry copying —
  _Depends on:_ I1, I2
- [x] `D2` Run a bounded soak covering at least three scheduler outcomes per Source and prove all
  six have fresh status/telemetry, including quiet-success behavior for event adapters that emit
  no records — _Depends on:_ D1
- [x] `D3` Exercise one controlled reversible failure on the public uptime Source, prove the
  expected `critical` Check and Source `error` lifecycle, restore the exact valid configuration
  and prove recovery to fresh `ok` without leaving test state behind — _Depends on:_ D2

### Other

- [x] `T1` Record sanitized verification evidence and prioritized dogfood findings in this change,
  update SPEC's stale snapshot/current sequence to the proven state, and append every runtime bug
  to this Backlog before implementing any fix — _Depends on:_ F1, D3
- [x] `T2` Synchronize README's stale five-Source/pre-cutover claims with Change 20's sanitized
  six-Source proof while preserving the longer M6 dogfood window — _Depends on:_ T1
- [x] `T3` Correct the malformed frontend affected-test command in `STACK.md` to invoke the local
  Vitest binary through pnpm with explicit changed-file filters and deterministic no-related-test
  handling — _Depends on:_ T2

---

## Files

### Create / modify

~~~
docs/SPEC.md
docs/STACK.md
docs/KNOWN_GOTCHAS.md (only for a recurring newly proven pitfall)
docs/changes/20-infraege-source-reconciliation.md
README.md
data/sre-kit.db (runtime state, ignored)
data/secrets.enc.json (only through supported Source secret handling, ignored)
data/*.before-change20-* (permission-preserving local recovery snapshots, ignored)
~~~

### Do NOT touch

- infraegev2 VPS services, accounts, firewall, Compose/systemd lifecycle or target credentials
- Plaintext secrets in git, command output, evidence text, SQLite or copied artifacts
- Direct SQLite mutation, destructive deletion of historical telemetry or retired migration tables
- Adapter/API/frontend implementation unless an observed defect is appended to this Backlog first
- M9 projects/push ingress, M10 extension hardening or M11 distribution work

---

## Contracts

See `docs/SPEC.md` §3–§7 and §9.1/§12 and the Files list above. Existing Source API, adapter
manifests, encrypted-secret storage and target-ownership boundaries remain authoritative.

---

## Gate Checks

In addition to affected Fast Gate rows, capture sanitized evidence for: recoverable pre-mutation
snapshots; exactly six manifest-matching enabled Sources; at least three fresh scheduler outcomes
per Source; one public-uptime failure and recovery; authenticated Dashboard/Sources/detail browser
rendering; and zero target-side mutations. A Source is not proven by an old `ok` row or by adapter
process exit alone.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Snapshot `20260820T163423Z` preserves the pre-change DB (`0644`) and encrypted secret store
  (`0600`). A bounded core start proved the store key and runtime bootstrap; scheduler polling
  updated only Source outcomes before shutdown: both public SSH Sources recovered to `ok`, while
  the three WireGuard-only HTTP Sources became `unreachable` because the local tunnel was down.
- The architect-approved admin credential matched no current or retained hash; rotation snapshot
  `20260820T180717Z` preserves the previous store. The hash now derives from the existing
  owner-only credential file, and authenticated Source access succeeds without exposing plaintext.
- Reconciliation produced exactly six unique enabled Sources matching the infraegev2 templates.
  Each Source completed three independent fresh scheduler cycles with status `ok`; fail2ban also
  proved quiet success when a cycle emitted no new event. A reversible uptime failure produced
  critical Checks and Source `error`, then the exact restored config returned it to fresh `ok`.
- Authenticated browser verification covered Dashboard, Sources and all six Source-detail routes;
  the current console was clean on every route. The reproduced development hydration mismatch was
  fixed by keeping the document shell in TanStack Start's `shellComponent` boundary.
- Highest-priority follow-ups after this change are operational rather than target-side: keep the
  bounded dogfood window running before selecting M9, and specify a supported owner-only admin
  password rotation/recovery command in a future change. The one-off recovery here is deliberately
  not retained as an undocumented command. No infraegev2 VPS state was mutated.
- The corrected affected-test command found no unit tests related to the root route, so browser
  evidence is the behavioral proof for F2. Vitest also reports that Vite can replace
  `vite-tsconfig-paths` with native `resolve.tsconfigPaths`; treat removal as a separately verified
  dependency-cleanup candidate, not an unreviewed part of this runtime reconciliation.

---

## Commit Message

```
chore(change-20): reconcile infraege monitoring sources
```
