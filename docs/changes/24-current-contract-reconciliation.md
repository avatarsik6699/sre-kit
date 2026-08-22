# CHANGE 24 — Current Contract Reconciliation

<!-- TOKEN BUDGET: keep this file under 10,000 tokens. Be concise. -->

## Change Metadata

| Field | Value |
|-------|-------|
| Change | `24` |
| Slug | `current-contract-reconciliation` |
| Title | Current Contract Reconciliation |
| Status | `active` |
| Branch | `feature/24-current-contract-reconciliation` |

---

## Goal

Align sre-kit's executable contract metadata, roadmap, setup requirements and local dogfood state
with the shipped Change 22–23 implementation and its read-only observability boundary. Remove the
last promise of a core-owned action primitive and document the protected empty `default` Project
alongside the seven intended infraegev2 Sources under their single populated Project.

---

## Backlog

### Backend
- [x] `B1` Remove the retired future `action` promise from the embedded wire-contract description and describe the three adapter-emitted records plus core-generated Alert unambiguously — _Depends on:_ —
- [x] `B2` Add focused application coverage proving the migration-owned `default` Project is rejected before repository deletion — _Depends on:_ —

### Frontend
None

### Infra
- [x] `I1` Pin the Docker build to the documented Go 1.26.5 patch release and align README prerequisites with STACK's Go, Node and pnpm requirements — _Depends on:_ —

### Data
- [x] `D1` Prove the empty local dogfood Project is the migration-owned protected `default` sentinel, document that expected state, and verify the intended seven enabled Sources remain healthy under the single populated infraegev2 Project — _Depends on:_ —

### Other
- [x] `T1` Refresh README and SPEC status/roadmap/current-sequence wording for the session-scoped active dogfood model, implemented password recovery and completed M9 outputs — _Depends on:_ B1
- [x] `T2` Validate Markdown links, generated API-contract drift, backend tests, Docker tag availability and all seven production manifests against infraegev2's example after both repositories are aligned — _Depends on:_ B1, I1, D1, T1

---

## Files

### Create / modify
~~~
README.md
Dockerfile
docs/SPEC.md
docs/STACK.md
internal/contract/contract.schema.json
internal/projects/application/service_test.go
docs/changes/24-current-contract-reconciliation.md
~~~

### Do NOT touch
- Target deployment/apply/rollback controls or infraegev2 credentials/runtime
- Source configurations, telemetry rows, secrets.enc.json or either migration-owned/populated dogfood Project
- Go API behavior, frontend UI behavior or historical files under `docs/changes/archive/`

---

## Contracts

See `docs/SPEC.md` §1–§4 and §7–§11 and the Files list above. The existing read-only core boundary
and shipped API are unchanged.

---

## Gate Checks

In addition to the affected Fast Gate, validate Markdown links, generated API-contract drift, the
exact Docker tag, and all seven production manifests against infraegev2's Source example. Runtime
verification is read-only; the protected `default` Project must not be deleted.

---

## Architect Review Notes

- [x] No architect review issues recorded

---

## Implementation Notes

- The audited empty Project is the migration-owned protected `default` sentinel, so it was retained
  and documented instead of being treated as stale runtime data.

---

## Commit Message

```
docs(change-24): reconcile current core contracts
```
