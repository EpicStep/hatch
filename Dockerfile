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
    && mkdir -p /home/nonroot/.ssh \
    && chown -R nonroot:nonroot /home/nonroot \
    && chmod 700 /home/nonroot/.ssh

# sshd config: runs as nonroot on port 2222, no PAM, no privsep
RUN cat > /home/nonroot/sshd_config << 'EOF'
Port 2222
HostKey /home/nonroot/.ssh/ssh_host_ed25519_key
AuthorizedKeysFile /home/nonroot/.ssh/authorized_keys
PasswordAuthentication no
PubkeyAuthentication yes
PermitUserEnvironment yes
StrictModes no
UsePAM no
PidFile /home/nonroot/sshd.pid
EOF
RUN chown nonroot:nonroot /home/nonroot/sshd_config

COPY hack/entrypoint-dev.sh /home/nonroot/entrypoint-dev.sh
RUN chmod +x /home/nonroot/entrypoint-dev.sh

EXPOSE 2222

USER nonroot
WORKDIR /home/nonroot

ENTRYPOINT ["/home/nonroot/entrypoint-dev.sh"]
