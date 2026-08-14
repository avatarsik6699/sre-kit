# Contributing to sre-kit

Thanks for considering a contribution. This project follows a spec-driven development (SDD)
process — the source of truth for scope and rules is [`AGENTS.md`](AGENTS.md) and
[`docs/SPEC.md`](docs/SPEC.md), not this file. Read those first.

## Reporting bugs / requesting features

Open a GitHub issue. Include:

- What you expected vs. what happened
- Repro steps (adapter config shape, if relevant — redact secrets)
- Relevant logs (`docs/SPEC.md` §8 covers what's logged)

## Proposing a change

1. Check [`docs/SPEC.md`](docs/SPEC.md) — is this in scope for the current version, or does it
   need a spec-level discussion first? Open an issue for anything that changes product behavior,
   the data contract, or the API surface before writing code.
2. Each unit of work is tracked as `docs/changes/NN-slug.md`: a goal, a backlog of concrete
   tasks, an explicit files-touched list, and gate checks. Shipped changes are archived under
   `docs/changes/archive/` — read a couple of those for the shape expected.
3. Open a PR against `main` from a `feature/NN-slug`-style branch. Keep the change's backlog and
   the PR's diff in sync — the change file is the record of what was intended and what shipped.

## Development setup

See [`README.md`](README.md) § Getting started for running the server and frontend locally, and
[`docs/STACK.md`](docs/STACK.md) for the concrete toolchain, module layout, and conventions
(`docs/FRONTEND_CONVENTIONS.md` for frontend-specific rules).

## Before opening a PR

- Backend: `go build ./...`, `go test ./...`, `gofmt -l .`
- Frontend (`web/`): `pnpm typecheck`, `pnpm lint`, `pnpm test`, `pnpm build`
- Keep the four-entity data contract (`internal/contract/contract.schema.json`) additive-only —
  do not remove or repurpose an existing field; new adapters should fit the existing `Metric` /
  `Check` / `Event` shapes without a contract change.
- New adapters: implement the stdin-config / stdout-NDJSON / non-zero-exit-on-failure contract
  described in the README, with a `manifest.json` declaring `config_schema` (mark credential
  fields `"format": "secret"`).

## Code of conduct

Be respectful and constructive. Disagreements about scope or design belong in the issue/PR
discussion, not in personal remarks.
