# hatch

A minimal, non-root SSH-enabled container image for remote development on Kubernetes clusters.

Designed for scenarios where your service requires infrastructure that can't be replicated locally (hardware devices, specific network topology, storage backends, etc.). Instead of setting up a parallel dev environment, this image **replaces your production image in an existing workload** and inherits all its mounts, secrets, and network configuration.

## How it works

```
Local machine                          K8s cluster
┌──────────────┐    kubectl            ┌───────────────────────────┐
│              │    port-forward       │  Existing Pod spec:       │
│  Your IDE    │◄──────────────────────│  - volumes, /dev mounts   │
│  builds      │    SSH (port 2222)    │  - secrets (env vars)     │
│  locally,    │                       │  - network config         │
│  uploads     │                       │                           │
│  binary,     │                       │  Dev image replaces prod: │
│  runs on     │                       │  - debian:trixie-slim     │
│  remote      │                       │  - sshd (nonroot, 2222)   │
│              │                       │  - your binary runs here  │
└──────────────┘                       └───────────────────────────┘
```

## Install

```bash
# Via krew
kubectl krew install hatch

# Via go install
go install github.com/EpicStep/hatch/cmd/kubectl-hatch@latest

# Or download from GitHub Releases
```

## Quick start

Add `.hatch.yaml` to your project:

```yaml
namespace: default
kind: daemonset          # or deployment, statefulset
workload: vm-daemon
container: vm-daemon
image: ghcr.io/epicstep/hatch:latest
```

Then:

```bash
kubectl hatch up                        # swap image, pick any ready pod, port-forward
kubectl hatch up --node worker-3        # pick pod on a specific node (DaemonSet)
kubectl hatch up --pod myapp-0          # pick a specific pod (StatefulSet)
kubectl hatch status                    # check current state
kubectl hatch down                      # restore original image
```

### Custom image with extra packages

```bash
docker build --build-arg EXTRA_PACKAGES="strace gdb libfoo-dev" \
  -t my-dev:latest .
```

Update `image` in `.hatch.yaml` accordingly.

## Features

- **Non-root** — runs as uid 65532 (`nonroot`), compatible with `runAsNonRoot` pod security policies
- **Secrets passthrough** — K8s-injected env vars are exported to SSH sessions via `PermitUserEnvironment`
- **SSH key injection** — public key passed through env var at deploy time, no baking keys into the image
- **known_hosts cleanup** — automatically removes stale host key entries on `up`, no MITM warnings
- **Multi-workload** — supports DaemonSet, Deployment, and StatefulSet
- **Extensible** — add runtime dependencies via `EXTRA_PACKAGES` build arg
- **Signed images** — GHCR releases are signed with cosign (keyless/OIDC)

## CLI flags

All flags override `.hatch.yaml` values:

```
Global:
  --config         path to .hatch.yaml (default: .hatch.yaml in cwd)
  --kind           workload kind: daemonset, deployment, statefulset
  --workload       workload name
  --container      container name in the pod spec
  --image          dev image reference
  --namespace, -n  kubernetes namespace
  --kubeconfig     path to kubeconfig
  --context        kubernetes context

up:
  --node           select pod on a specific node (useful for DaemonSets)
  --pod            select a specific pod by name
  --ssh-key        path to SSH public key (default: ~/.ssh/id_ed25519.pub)
  --local-port     local port for SSH forwarding (default: 2222)
```

## Extending the image

```dockerfile
FROM ghcr.io/epicstep/hatch:latest
USER root
RUN apt-get update && apt-get install -y --no-install-recommends <your-packages> \
    && rm -rf /var/lib/apt/lists/*
USER nonroot
```

## Verifying image signatures

```bash
cosign verify \
  --certificate-identity-regexp="github.com/EpicStep/hatch" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/epicstep/hatch:latest
```

## License

MIT
