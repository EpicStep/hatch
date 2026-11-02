#!/bin/bash
set -e

# Generate host key if missing (runs as nonroot, keys in home dir)
if [ ! -f /home/nonroot/.ssh/ssh_host_ed25519_key ]; then
    ssh-keygen -t ed25519 -f /home/nonroot/.ssh/ssh_host_ed25519_key -N ""
fi

# Export pod env vars to SSH environment file.
# SSH sessions (and any process started through them) inherit these vars.
# This is how K8s secrets become visible to remotely launched processes.
env | grep -Ev '^(HOME|HOSTNAME|PATH|TERM|SHLVL|PWD|SHELL|USER|LOGNAME|AUTHORIZED_KEYS|_)=' \
    > /home/nonroot/.ssh/environment || true

# Setup authorized_keys from env var (injected by dev-up.sh via K8s patch)
if [ -n "${AUTHORIZED_KEYS}" ]; then
    echo "${AUTHORIZED_KEYS}" > /home/nonroot/.ssh/authorized_keys
    chmod 644 /home/nonroot/.ssh/authorized_keys
fi

SSHD=(/usr/sbin/sshd -D -e -f /home/nonroot/sshd_config)

# Pass CAP_SYS_PTRACE to sshd and every SSH session as an ambient capability,
# so debuggers such as delve work. setpriv gets it from its file capability,
# which only applies when the pod adds SYS_PTRACE and allows privilege
# escalation (hatch up does both). Otherwise start sshd without it.
if setpriv --inh-caps=+sys_ptrace --ambient-caps=+sys_ptrace true 2>/dev/null; then
    exec setpriv --inh-caps=+sys_ptrace --ambient-caps=+sys_ptrace -- "${SSHD[@]}"
fi

exec "${SSHD[@]}"
