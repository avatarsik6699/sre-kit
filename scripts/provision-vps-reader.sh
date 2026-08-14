#!/usr/bin/env bash
# Provisions a dedicated, low-privilege SSH user on a target VPS for sre-kit's SSH-based adapters
# (host-metrics-ssh, fail2ban-ssh) to connect as — separate from any existing operator/ops account
# on the host, so sre-kit's credentials and blast radius stay independent of other tooling.
#
# Run as root ON THE TARGET VPS (not from your workstation):
#   SRE_KIT_READER_SSH_PUBLIC_KEY="ssh-ed25519 AAAA... you@workstation" \
#     ./provision-vps-reader.sh
#
# Generate the keypair on your workstation first (never on the server):
#   ssh-keygen -t ed25519 -f ./sre-kit-reader -C sre-kit
# then paste the contents of sre-kit-reader.pub as SRE_KIT_READER_SSH_PUBLIC_KEY, and keep
# sre-kit-reader (the private key) to paste into the source's "secret" field when adding
# host-metrics-ssh / fail2ban-ssh through the sre-kit UI (auth_method: private_key).
#
# What this grants, and what it doesn't:
#   - A normal (non-sudo) shell account. host-metrics-ssh only needs /proc and `df`, world-
#     readable by default — no extra grants required for it.
#   - Membership in the `adm` group, so `tail`-ing /var/log/fail2ban.log (fail2ban's default log
#     path) works without sudo. If fail2ban logs elsewhere on this host, adjust accordingly.
#   - authorized_keys restricted with no-pty/no-agent-forwarding/no-X11-forwarding/
#     no-port-forwarding — sre-kit's adapters never need an interactive shell or tunneling, so
#     disabling those narrows what a leaked key is useful for.
#   - This account is NOT locked to a fixed command set (no ForceCommand) — sre-kit's adapters
#     send their own shell command per invocation (see adapters/host-metrics-ssh/main.go's
#     sampleScript, adapters/fail2ban-ssh/main.go's tailLog), so a fixed allowed-command list
#     isn't compatible with today's adapter design. This is a known v1 trade-off (see
#     docs/SPEC.md §11 "Adapter sandboxing") — if you need a tighter, ForceCommand-style model,
#     that requires making the adapters' remote command configurable first; ask before assuming
#     this script is sufficient for a security-sensitive host.

set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  echo "Run as root, on the target VPS." >&2
  exit 1
fi

: "${SRE_KIT_READER_SSH_PUBLIC_KEY:?Set SRE_KIT_READER_SSH_PUBLIC_KEY to the readers public key}"
reader_user="${SRE_KIT_READER_USER:-sre-kit-reader}"

if ! id "$reader_user" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "$reader_user"
  echo "Created user $reader_user"
else
  echo "User $reader_user already exists, reusing it"
fi

if getent group adm >/dev/null 2>&1; then
  usermod -aG adm "$reader_user"
else
  echo "Warning: no 'adm' group on this host — fail2ban-ssh may not be able to read" \
    "/var/log/fail2ban.log without further permission changes." >&2
fi

if [[ ! -r /var/log/fail2ban.log ]]; then
  echo "Warning: /var/log/fail2ban.log not found or not readable yet — fail2ban-ssh will fail" \
    "until fail2ban is installed and logging there (its default logtarget)." >&2
fi

ssh_dir="/home/$reader_user/.ssh"
install -d -m 700 -o "$reader_user" -g "$reader_user" "$ssh_dir"
printf 'no-pty,no-agent-forwarding,no-X11-forwarding,no-port-forwarding %s\n' \
  "$SRE_KIT_READER_SSH_PUBLIC_KEY" | install -m 600 -o "$reader_user" -g "$reader_user" \
  /dev/stdin "$ssh_dir/authorized_keys"

cat <<EOF

Done. In sre-kit, add sources with:
  host:        this VPS address
  username:    $reader_user
  auth_method: private_key
  secret:      the PRIVATE key matching the public key you passed in (sre-kit-reader, not .pub)

host-metrics-ssh needs nothing further. fail2ban-ssh needs fail2ban already installed and logging
to /var/log/fail2ban.log (its default) — if it isn't yet, set that up first, this script does not
install fail2ban itself.
EOF
