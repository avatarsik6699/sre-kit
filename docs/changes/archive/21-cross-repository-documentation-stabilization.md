# CHANGE 21 — Cross-repository documentation stabilization

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `21` |
| Slug | `cross-repository-documentation-stabilization` |
| Title | Cross-repository documentation stabilization |
| Status | `archived` |
| Branch | `feature/21-cross-repository-documentation-stabilization` |

---

## Goal

Make sre-kit's public and operator documentation agree with its current API, first-run auth,
six-Source dogfood state and pull-adapter ownership boundary. Repair lifecycle and workflow
instructions that allowed an archived change to remain active, while preserving the accepted
workstation/offline-alert limitation and keeping target deployment outside sre-kit.

---

## Backlog

### Operations

- [x] `I1` Describe M6 as a valid bounded proof whose longer evidence window is currently paused
  while the workstation core is off, and make the supported owner-only admin password
  rotation/recovery command an explicit prerequisite for continued dogfooding — _Depends on:_ —

### Documentation and workflow

- [x] `T1` Correct README/SPEC transport, notification-channel request/response and first-run auth
  prose to match the implemented contracts without changing API behavior — _Depends on:_ —
- [x] `T2` Correct archived Change 20's status and require `/ship` to set `Status: archived` before
  moving future change files — _Depends on:_ —
- [x] `T3` Replace stale workflow capability names with installed architecture/UI-review skills
  and clarify the source-package bootstrap playbook's non-runnable status in an integrated repo —
  _Depends on:_ —
- [x] `T4` Repair current docs/source comments that still point to pre-archive change paths —
  _Depends on:_ —
- [x] `T5` Validate Markdown links, generated API-contract consistency, all six Source manifests
  against the infraegev2 example and the affected Fast Gate — _Depends on:_ I1, T1, T2, T3, T4

---

## Files

### Create / modify

~~~
README.md
docs/SPEC.md
docs/STACK.md
docs/KNOWN_GOTCHAS.md
docs/changes/archive/20-infraege-source-reconciliation.md
docs/playbooks/plan.md
docs/playbooks/work.md
docs/playbooks/ship.md
docs/playbooks/workflow-init.md
current source comments that link shipped docs/changes files
docs/changes/21-cross-repository-documentation-stabilization.md
~~~

### Do NOT touch

- API behavior, schemas, migrations, secrets, Source rows or runtime data
- Alert routing, adapter execution or notification delivery behavior
- Target deployment/apply/rollback controls or infraegev2 credentials
- Files in the sibling `infraegev2` repository (owned by linked infraegev2 Change 47)

---

## Contracts

The six current infraegev2 integrations are pull Sources registered in sre-kit and collected over
SSH, private HTTP or TCP. sre-kit owns adapter execution, normalization, alerting and monitoring;
infraegev2 owns target lifecycle. A generic push ingress remains planned M9 work. Notification
channel responses expose no secret reference, and first-run auth generates a password when no
stored hash exists.

---

## Gate Checks

In addition to the affected Fast Gate, run the generated API-contract drift check and validate all
six manifests against infraegev2's example Source template. Runtime and secrets inspection remains
read-only and sanitized.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

None

---

## Commit Message

```
docs(change-21): stabilize cross-repository contracts
```
