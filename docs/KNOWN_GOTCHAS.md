# Known Gotchas

> Project memory file. Capture recurring pitfalls that repeatedly waste time during coding,
> testing, or deploys.

## How To Use

- Add only issues that are likely to happen again.
- Prefer concrete symptoms, root cause, and the shortest reliable fix.
- Remove entries that are no longer relevant.

## Gotcha Log

### Docker-owned files break host operations (`EACCES` / `EPERM` / read-only)

> Keep this entry only if the project uses Docker bind mounts. Otherwise delete it.

- **Symptoms**: file operations fail with `EACCES`, `EPERM`, "Permission denied", or "Read-only file system". Most common paths: container-generated build/cache directories on the host (`.nuxt/`, `.output/`, `node_modules/.cache/`, `__pycache__/`).
- **Root cause**: a Docker container wrote to a bind-mounted host directory as root.
- **Fix (host)**:
  ```bash
  sudo chown -R $USER:$USER <path>   # reclaim ownership, keep files
  sudo rm -rf <path>                 # OR discard the generated artefact
  ```
- **Agent protocol**: agents MUST NOT run `sudo`, `chmod -R 777`, or loop the failing operation. Instead, stop and post this exact handoff to the user (substituting real `<path>` and `<cmd>`):

  > ⛔ **Permission denied.** I cannot modify `<path>` while running `<cmd>`.
  >
  > This usually happens when a Docker container wrote files to a bind-mounted host directory as root. Please run one of the following on the host:
  >
  > ```bash
  > sudo chown -R $USER:$USER <path>
  > sudo rm -rf <path>
  > ```
  >
  > When the fix is applied, reply with the single word **`continue`** and I will retry the failed operation from the same step.

  On receiving `continue` (case-insensitive), retry the failed operation once. If it fails a second time with the same error, stop again and ask the user to confirm the fix was actually applied — do not loop a third time.

- **Prevention**: run Docker with a matching host UID/GID or use named volumes for cache directories that containers own.

### `host-metrics-ssh` skips SSH host key verification (v1)

- **Symptoms**: not a failure today — flagged so it isn't mistaken for an oversight later. The
  adapter (`adapters/host-metrics-ssh/main.go`) dials with `ssh.InsecureIgnoreHostKey()`, so it
  will happily connect to a spoofed host (MITM) with no warning.
- **Root cause**: v1 has no `known_hosts` store or first-connection TOFU (trust-on-first-use)
  pinning mechanism; building one was out of scope for `M2` (docs/changes/02-host-metrics-ssh-adapter.md).
- **Fix**: none yet — acceptable for a solo-developer/small-team SSH-to-own-VPS use case in v1,
  but revisit before `M6` dogfooding widens exposure (e.g. persist the host key fingerprint on
  first successful connect, alongside the source's config, and fail loudly on mismatch after).
- **Prevention**: n/a until a TOFU/pinning implementation exists.

### Pull-adapter subprocess failure never marks a source `unreachable`

- **Symptoms**: a source whose adapter subprocess exits non-zero or times out keeps whatever
  `sources.last_status` it last had (or the `unreachable` default from `Create`, if it never
  successfully reported) — it never transitions to `unreachable` on a live failure, and
  `internal/alertrouter`'s connectivity-alert debounce (docs/SPEC.md §6) never sees a status to
  react to for this path.
- **Root cause**: `internal/adapterengine/application.Runner.RunOnce` only calls its `disable`
  hook after 10 consecutive invalid NDJSON lines (`maxConsecutiveInvalidLines`); a subprocess spawn
  error, non-zero exit, or timeout just returns an `error` from `RunOnce` with no status-hook call
  at all. SPEC §4 documents "non-zero exit or timeout marks the source `unreachable` (with
  debounce)" but this was never wired.
- **Fix**: none yet — out of scope for every change through `05-alert-router-telegram` (all
  explicitly exclude `internal/adapterengine/**`). Fix by having `Runner`/`Scheduler` call a
  status-hook (mirroring `SourceDisabler`) on subprocess failure, wired in `cmd/server/main.go` to
  both `sourcesService.MarkSeen(ctx, id, "unreachable")` and
  `alertrouterService.EvaluateSourceStatus(ctx, id, "unreachable")`.
- **Prevention**: n/a until wired; see `docs/changes/05-alert-router-telegram.md` (or its archived
  location once shipped) Implementation Notes for the alert-router-side context.

<!--
### [Title — short, punchy, searchable]

- **Symptoms**: [what fails, what error message]
- **Root cause**: [why it happens]
- **Fix**: [shortest reliable fix]
- **Prevention**: [optional — how to avoid hitting it again]
- **Links**: [optional — docs / issue / PR]
-->
