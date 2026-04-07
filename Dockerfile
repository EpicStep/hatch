FROM debian:trixie-slim

ARG EXTRA_PACKAGES=""

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        openssh-server \
        procps \
        curl \
        # Add service-specific runtime deps below:
        # libspdk-dev libnuma-dev ...
        ${EXTRA_PACKAGES} \
    && rm -rf /var/lib/apt/lists/*

# nonroot user (uid 65532) — matches distroless nonroot for prod consistency
RUN groupadd -g 65532 nonroot \
    && useradd -u 65532 -g nonroot -m -d /home/nonroot -s /bin/bash nonroot \
    && usermod -p '*' nonroot \
    && mkdir -p /home/nonroot/.ssh \
    && chown -R nonroot:nonroot /home/nonroot \
    && chmod 700 /home/nonroot/.ssh

# sshd config: runs as nonroot on port 2222, no PAM, no privsep
COPY <<EOF /home/nonroot/sshd_config
Port 2222
HostKey /home/nonroot/.ssh/ssh_host_ed25519_key
AuthorizedKeysFile /home/nonroot/.ssh/authorized_keys
AuthenticationMethods publickey
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
PermitUserEnvironment yes
StrictModes no
UsePAM no
PidFile /home/nonroot/sshd.pid
EOF

COPY --chmod=755 hack/entrypoint-dev.sh /home/nonroot/entrypoint-dev.sh

RUN chown -R nonroot:nonroot /home/nonroot/sshd_config /home/nonroot/entrypoint-dev.sh

EXPOSE 2222

USER 65532
WORKDIR /home/nonroot

ENTRYPOINT ["/home/nonroot/entrypoint-dev.sh"]