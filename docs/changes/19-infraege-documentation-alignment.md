# CHANGE 19 — infraege Documentation Alignment

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `19` |
| Slug | `infraege-documentation-alignment` |
| Title | infraege Documentation Alignment |
| Status | `active` |
| Branch | `feature/19-infraege-documentation-alignment` |

---

## Goal

Audit sre-kit's current documentation against the shipped core and the live infraegev2 split
operations topology. Synchronize Source ownership, the six-Source registration handoff and the
accepted root/password SSH inputs without moving target lifecycle or credentials into sre-kit.
This change is linked to infraegev2 Change 44.

---

## Backlog

### Backend

None

### Frontend

None

### Infra

- [x] `I1` Inventory current sre-kit documentation, shipped change history and implemented Source/adapter contracts; classify stale plans and unresolved infraege dogfood gaps — _Depends on:_ —
- [x] `I2` Align the infraege first-party integration documentation with the live independent operations stack and the ownership boundary recorded by infraegev2 Change 44 — _Depends on:_ I1
- [x] `I3` Document the six intended infraege Sources, their target-owned prerequisites and their current registration/verification state without copying credentials or deployment authority into sre-kit docs — _Depends on:_ I1
- [x] `I4` Treat root/password SSH as an accepted target input for infraege's SSH adapters and administration horizon, not as a temporary migration phase or sre-kit-owned access policy — _Depends on:_ I1

### Data

None

### Other

- [x] `T1` Repair stale lifecycle claims, placeholders, broken references and contradictions in the current documentation while preserving archived history — _Depends on:_ I1
- [x] `T2` Align sre-kit's roadmap with the remaining dogfood registration proof and the later runtime-distribution boundary; do not claim deployment or always-on alerting before those outputs exist — _Depends on:_ I2, I3, I4, T1
- [x] `T3` Run the documentation-relevant Fast Gate and cross-repository terminology/link checks; record residual uncertainty that repository evidence cannot settle — _Depends on:_ T2

---

## Files

### Create / modify

~~~
README.md
docs/SPEC.md
docs/STACK.md
docs/KNOWN_GOTCHAS.md
docs/FRONTEND_CONVENTIONS.md
docs/playbooks/*.md
docs/changes/19-infraege-documentation-alignment.md
~~~

### Do NOT touch

- Go/TypeScript runtime code, adapters, schemas, migrations or deployment artifacts
- Protected Source credentials, SSH passwords, hashes or live sre-kit data
- infraegev2 target lifecycle ownership or live VPS state
- Archived change contents except narrow lifecycle-metadata corrections that remove false active state

---

## Contracts

See `docs/SPEC.md` §1, §7–§11 and the linked infraegev2 Change 44. This audit changes documentation
only; existing Source, adapter, HTTP and telemetry contracts remain unchanged.

---

## Gate Checks

Documentation-only Fast Gate: run repository-supported link/format checks where available and
explicit cross-repository terminology checks. Runtime suites are not required unless an audit
finding expands into code, which must first be appended to this Backlog.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- Read-only local evidence is partial and stale: five enabled Source rows exist, uptime is absent,
  both SSH Sources are `unreachable` with no `last_seen_at`, and the other three were last seen on
  2026-08-15. No local core process was running, so this is not current production monitoring proof.

---

## Commit Message

```
docs(change-19): align infraege integration contracts
```
