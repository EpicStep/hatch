# CLAUDE.md

## Project

**hatch** (`kubectl-hatch`) — kubectl plugin for remote development on K8s clusters.
Temporarily replaces a workload's container image with an SSH-enabled dev container,
inheriting all pod configuration (mounts, secrets, network topology).

Module: `github.com/EpicStep/hatch`
Go: 1.26
License: MIT

## Architecture

Two deliverables from one repo:

- **CLI plugin** (`cmd/kubectl-hatch`) — Go binary, installed via krew or GitHub Releases
- **Dev image** (`Dockerfile`) — debian:trixie-slim + openssh, runs as nonroot (uid 65532), SSH on port 2222

### Packages

| Package | Purpose |
|---------|---------|
| `internal/cli` | Cobra commands: `up`, `down`, `status` + helpers |
| `internal/config` | `.hatch.yaml` loading, defaults |
| `internal/workload` | Workload interface abstracting DaemonSet/Deployment/StatefulSet + strategic merge patch |
| `internal/knownhosts` | SSH known_hosts entry cleanup |

### Key abstractions

`workload.Workload` interface unifies DaemonSet, Deployment, StatefulSet behind
common Annotations/Selector/PodSpec/Patch methods. Factory: `workload.New(ctx, client, ns, kind, name)`.

### Annotations

All state tracked via workload annotations:

- `hatch.dev/active` — "true" when dev mode is on
- `hatch.dev/original-image` — image before swap
- `hatch.dev/user` — who activated dev mode
- `hatch.dev/node`, `hatch.dev/pod` — reconnect hints

### Config

`.hatch.yaml` fields: `namespace`, `kind`, `workload`, `container`, `image`.
`user` is NOT in config — comes from `--dev-user` flag or `$USER`.

Defaults: namespace=`default`, kind=`daemonset`, image=`ghcr.io/epicstep/hatch:latest`.

## Validation

After any non-trivial code change, run:

```sh
make test
make lint
```

Fix all failures before considering the task done.

## Testing

- `testify/assert` + `testify/require` for assertions
- `fake.NewSimpleClientset()` for K8s API mocking
- Table-driven tests, PascalCase names, `t.Parallel()` everywhere

## Code style

Separate logical blocks with blank lines. Never stack a function call,
error check, and the next operation together without breathing room.

Good:

```go
result, err := doSomething()
if err != nil {
    return err
}

next, err := doNext(result)
if err != nil {
    return err
}
```

Bad:

```go
result, err := doSomething()
if err != nil {
    return err
}
next, err := doNext(result)
if err != nil {
    return err
}
```

## Linting

golangci-lint v2 with revive (most rules enabled), goheader, misspell, nilnil, paralleltest.
Formatters: goimports with local prefix `github.com/EpicStep/hatch`, gofmt.

## CI/CD

- **CI** (`ci.yml`) — runs `go test` + `golangci-lint` on every push and PRs to main
- **Dev Image** (`build.yml`) — builds Docker image, pushes to GHCR, signs with cosign. Triggered on `v*` tags
- **CLI Release** (`release.yml`) — runs CI first, then goreleaser. Triggered on `v*` tags. Publishes GitHub Release + krew index update (`EpicStep/krew-index`, requires `KREW_GITHUB_TOKEN` secret)

Release flow: `git tag v1.0.0 && git push origin v1.0.0` triggers all three workflows.
