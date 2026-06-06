# omni-kubeconfig

[![CI](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/ci.yml)
[![Docker](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/docker.yml/badge.svg)](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/docker.yml)
[![Release](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/release.yml/badge.svg)](https://github.com/Jubblin/omni-kubeconfig/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Jubblin/omni-kubeconfig)](https://goreportcard.com/report/github.com/Jubblin/omni-kubeconfig)

Download admin kubeconfigs for every cluster on a [Sidero Omni](https://docs.siderolabs.com/omni) server and merge them into a single file for `kubectl`.

## Features

- **`auth`** — SideroV1 PGP + browser login (same flow as `omnictl`)
- **`sync`** — List all Omni clusters, download kubeconfigs, merge into one file
- Incremental merge into existing output by default (`--merge-existing`); full replace with `--merge-existing=false`
- Backups existing output as `*.bak.<timestamp>` before overwrite
- Omni API v2 compatible

## Quick start

### Install from release

Download the binary for your platform from [GitHub Releases](https://github.com/Jubblin/omni-kubeconfig/releases) (same platforms as [omnictl](https://github.com/siderolabs/omni/releases)):

| Platform | Asset |
|----------|--------|
| macOS Intel | `omni-kubeconfig-darwin-amd64` |
| macOS Apple Silicon | `omni-kubeconfig-darwin-arm64` |
| Linux x86_64 | `omni-kubeconfig-linux-amd64` |
| Linux arm64 | `omni-kubeconfig-linux-arm64` |
| Windows x86_64 | `omni-kubeconfig-windows-amd64.exe` |

Rename to `omni-kubeconfig` (or `omni-kubeconfig.exe` on Windows) and place on your `PATH`. Verify with `sha256sum.txt` on the release page.

### Install from source

```bash
git clone https://github.com/Jubblin/omni-kubeconfig.git
cd omni-kubeconfig
make install
```

### Install with Docker

```bash
docker pull ghcr.io/jubblin/omni-kubeconfig:latest
```

You still need a host `omniconfig` (from [omnictl](https://docs.siderolabs.com/omni/getting-started/install-and-configure-omnictl)) and `kubectl` + `kubelogin` on the machine where you run `kubectl`. Full container workflow: [Run with Docker](#run-with-docker).

### Binary Prerequisites

- [omnictl](https://docs.siderolabs.com/omni/getting-started/install-and-configure-omnictl) / valid `omniconfig` (default: `~/.talos/omni/config`)
- `kubectl`
- [`kubectl oidc-login`](https://github.com/int128/kubelogin) (`kubelogin`) in `PATH`

```bash
brew install siderolabs/tap/sidero-tools   # macOS/Linux: omnictl, kubelogin, talosctl
```

### Usage

```bash
omni-kubeconfig auth
omni-kubeconfig sync
export KUBECONFIG=~/.kube/omni-merged-config
kubectl config get-contexts
```

See [Usage](#usage) and [Reference](#reference) below for all flags.

## Run with Docker

Container images are published to [GitHub Container Registry](https://github.com/Jubblin/omni-kubeconfig/pkgs/container/omni-kubeconfig) as `ghcr.io/jubblin/omni-kubeconfig` when a `v*` release tag is pushed. CI builds the image on every push/PR but does not publish it.

The image contains only the `omni-kubeconfig` binary (distroless, no shell). Mount your host Omni credentials and kube output directory; run `kubectl` on the host against the merged file.

### Image tags

| Tag | When |
|-----|------|
| `latest` | Most recent release |
| `1.2.3` | Exact semver (no `v` prefix) |
| `1.2`, `1` | Major/minor aliases |

### Container Prerequisites

Same as the binary install, except the CLI runs inside Docker:

- Valid `omniconfig` on the host (default `~/.talos/omni/config`) — typically created with `omnictl` on the host
- Network access from the container to your Omni endpoint
- `kubectl` and [`kubectl oidc-login`](https://github.com/int128/kubelogin) on the **host** (not in the image)

### Typical workflow

```bash
export OKC_IMAGE=ghcr.io/jubblin/omni-kubeconfig:latest

docker pull "$OKC_IMAGE"

# 1) Authenticate (first time or expired key)
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  -e HOME="$HOME" \
  -e BROWSER=echo \
  -v "$HOME/.talos:$HOME/.talos" \
  "$OKC_IMAGE" auth

# 2) Sync all clusters into ~/.kube/omni-merged-config on the host
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  -e HOME="$HOME" \
  -v "$HOME/.talos:$HOME/.talos" \
  -v "$HOME/.kube:$HOME/.kube" \
  "$OKC_IMAGE" sync

# 3) Use kubectl on the host
export KUBECONFIG="$HOME/.kube/omni-merged-config"
kubectl config get-contexts
```

Open the login URL printed during `auth` when using `BROWSER=echo`. Omit `-e BROWSER=echo` if your environment can open a browser from inside the container.

### Recommended `docker run` options

Always mount credentials and match the host user so PGP keys and kubeconfig files remain readable/writable:

| Flag | Purpose |
|------|---------|
| `--user "$(id -u):$(id -g)"` | Write keys and kubeconfig as your user (image default is distroless `nonroot`) |
| `-e HOME="$HOME"` | Keep default paths (`~/.talos/...`, `~/.kube/...`) aligned with mount targets |
| `-v "$HOME/.talos:$HOME/.talos"` | Omniconfig + SideroV1 PGP keys (`auth` writes under `~/.talos/keys`) |
| `-v "$HOME/.kube:$HOME/.kube"` | Sync output and `.bak.*` backups (required for `sync`) |

### Commands

```bash
# Version / help
docker run --rm "$OKC_IMAGE" --version
docker run --rm "$OKC_IMAGE" sync --help

# Auth (re-login or force new key)
docker run --rm -it \
  --user "$(id -u):$(id -g)" -e HOME="$HOME" \
  -v "$HOME/.talos:$HOME/.talos" \
  "$OKC_IMAGE" auth --force

# Sync: dry-run, subset of clusters, custom output
docker run --rm \
  --user "$(id -u):$(id -g)" -e HOME="$HOME" \
  -v "$HOME/.talos:$HOME/.talos" -v "$HOME/.kube:$HOME/.kube" \
  "$OKC_IMAGE" sync --dry-run

docker run --rm \
  --user "$(id -u):$(id -g)" -e HOME="$HOME" \
  -v "$HOME/.talos:$HOME/.talos" -v "$HOME/.kube:$HOME/.kube" \
  "$OKC_IMAGE" sync -c prod -c staging -o "$HOME/.kube/omni-prod"

# Non-interactive service account (no browser)
docker run --rm \
  --user "$(id -u):$(id -g)" -e HOME="$HOME" \
  -e OMNI_SERVICE_ACCOUNT="$OMNI_SERVICE_ACCOUNT" \
  -v "$HOME/.talos:$HOME/.talos" -v "$HOME/.kube:$HOME/.kube" \
  "$OKC_IMAGE" sync
```

Override paths with flags or env vars (same as the native CLI): `--omniconfig`, `--context`, `-e OMNICONFIG=...`, `-e SIDEROV1_KEYS_DIR=...`.

### Volume reference

| Host path | Container path | Purpose |
|-----------|----------------|---------|
| `~/.talos` | `$HOME/.talos` | `omniconfig` (`~/.talos/omni/config`) and PGP keys (`~/.talos/keys`) |
| `~/.kube` | `$HOME/.kube` | Merged kubeconfig (default `~/.kube/omni-merged-config`) |

### Build and run locally

```bash
make docker-build                    # tags ghcr.io/jubblin/omni-kubeconfig:local
make docker-run ARGS="--version"
make docker-run ARGS="auth"
make docker-run ARGS="sync --dry-run"

# Custom image name/tag
make docker-build IMAGE=myregistry/omni-kubeconfig IMAGE_TAG=dev
make docker-run IMAGE=myregistry/omni-kubeconfig IMAGE_TAG=dev ARGS="sync"
```

## Development

### Dev Container

Open in VS Code / Cursor → **Reopen in Container**. Includes Go 1.25, golangci-lint, and GitHub CLI. Your host `~/.talos` directory is mounted for Omni authentication.

### Local

```bash
make check       # test + lint + host build + all platform builds
make build-all   # cross-compile all omnictl platforms into bin/
make dc-check    # CI-parity checks inside dev container (recommended)
make sbom        # CycloneDX SBOM (Go modules) → dist/sbom.cyclonedx.json
make hadolint    # Lint Dockerfile
make trivy-image # Scan local image + dist/sbom-image.cyclonedx.json (after docker-build)
make test
make lint
make build
./bin/omni-kubeconfig --version
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

### CI/CD

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| [ci.yml](.github/workflows/ci.yml) | Push/PR to `main` | Test, vet, lint, cross-compile, Go module SBOM (`cyclonedx-gomod`), Trivy dependency scan |
| [docker.yml](.github/workflows/docker.yml) | Push/PR / tag `v*` | Hadolint → build image → Trivy scan + image SBOM; on tag: push to GHCR, Cosign sign image + SBOM |
| [release.yml](.github/workflows/release.yml) | Tag `v*` | GoReleaser binaries + module SBOM; Cosign sign-blob for `sbom.cyclonedx.json` |

[Dependabot](.github/dependabot.yml) updates Go modules and GitHub Actions weekly.

### Releasing (maintainers)

```bash
# Ensure CHANGELOG.md is updated, then:
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser publishes bare binaries (naming matches `omnictl-*`) for darwin/linux amd64+arm64 and windows amd64 only. See [.goreleaser.yaml](.goreleaser.yaml).

Versioning follows [Semantic Versioning](https://semver.org/) — see [CHANGELOG.md](CHANGELOG.md).

### First publish to GitHub

```bash
cd omni-kubeconfig
git init -b main
git add .
git commit -m "feat: initial public release"
git remote add origin git@github.com:Jubblin/omni-kubeconfig.git
git push -u origin main
git tag v0.1.0
git push origin v0.1.0
```

## Usage

Authenticate to Omni first (opens browser when the key is missing or expired):

```bash
omni-kubeconfig auth
omni-kubeconfig auth --force   # delete PGP key and re-login
```

Sync all clusters into `~/.kube/omni-merged-config`:

```bash
omni-kubeconfig sync
```

Use the merged config:

```bash
export KUBECONFIG=~/.kube/omni-merged-config
kubectl get nodes --context <cluster-name>
```

On success, `sync` prints `export KUBECONFIG=<path>` (disable with `--print-export=false`).

### Docker

Run the same commands inside the published image; mount `~/.talos` and `~/.kube`, use `--user "$(id -u):$(id -g)"`, and keep `kubectl` on the host. See [Run with Docker](#run-with-docker).

## Reference

`omni-kubeconfig --help`, `auth --help`, and `sync --help` mirror this section.

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--omniconfig` | `OMNICONFIG` or `~/.talos/omni/config` | Path to omniconfig |
| `--context` | Selected context | Omni context name |
| `--insecure-skip-tls-verify` | `false` | Skip TLS verification for Omni API |
| `--siderov1-keys-dir` | `SIDEROV1_KEYS_DIR` or `~/.talos/keys` | SideroV1 PGP keys directory |
| `-h`, `--help` | | Show help |
| `-v`, `--version` | | Semver, build metadata, Omni API level |

### `auth` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Delete PGP key and force new browser login |

### `sync` flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o`, `--output` | `~/.kube/omni-merged-config` | Merged kubeconfig path |
| `-c`, `--cluster` | all | Sync only these clusters (repeatable) |
| `--merge-existing` | `true` | Load existing output and merge; `false` replaces with clusters synced this run |
| `--force` | `false` | Overwrite merge conflicts instead of renaming |
| `--grant-type` | `auto` | OIDC grant: `auto`, `authcode`, `authcode-keyboard` |
| `--dry-run` | `false` | List clusters only |
| `--print-export` | `true` | Print `export KUBECONFIG=...` on success |

### Environment variables

| Variable | Description |
|----------|-------------|
| `OMNICONFIG` | Omniconfig path |
| `OMNI_ENDPOINT` | Override Omni API URL |
| `OMNI_SERVICE_ACCOUNT` | Base64 service account (non-interactive) |
| `SIDEROV1_KEYS_DIR` | PGP keys directory |
| `BROWSER` | Set to `echo` to print login URL instead of opening a browser |

### Examples

```bash
BROWSER=echo omni-kubeconfig auth
omni-kubeconfig sync --dry-run
omni-kubeconfig sync -o ~/.kube/omni-prod -c prod -c staging --force
omni-kubeconfig sync --merge-existing=false   # drop contexts from prior syncs not in this run
omni-kubeconfig sync --grant-type authcode-keyboard
```

## Troubleshooting

| Error | Fix |
|-------|-----|
| `client API version mismatch` | Use a current release (Omni API 2) |
| `key expired` | `omni-kubeconfig auth` or `auth --force` |
| `authenticate with ...` | Check omniconfig URL and network |
| `unknown command "oidc-login"` | Install `kubectl-oidc-login` |
| Permission denied on `~/.talos` or `~/.kube` in Docker | Add `--user "$(id -u):$(id -g)"` and mount both directories |
| Browser does not open in container | Use `-e BROWSER=echo` and open the printed URL on the host |

## How it works

1. Connects to Omni (same client bootstrap as `omnictl`)
2. Lists `Clusters.omni.sidero.dev` resources
3. Downloads each cluster kubeconfig via the management API
4. Merges with [go-kubeconfig](https://github.com/siderolabs/go-kubeconfig) (into existing output when `--merge-existing`, default)
5. Backs up existing output before writing

## Project layout

```
.
├── cmd/omni-kubeconfig/    # CLI entrypoint
├── internal/omni/          # Omni client, auth, sync
├── internal/version/       # Semver metadata
├── .devcontainer/          # Dev Container spec
├── Dockerfile              # Multi-stage image (distroless runtime)
├── .github/workflows/      # CI, Docker, and release pipelines
├── CHANGELOG.md
├── CONTRIBUTING.md
├── LICENSE
├── NOTICE                  # Third-party licenses
└── SECURITY.md
```

## License

This project is licensed under the [MIT License](LICENSE).

Dependencies include [Sidero Omni client](https://github.com/siderolabs/omni) libraries under the [Mozilla Public License 2.0](https://www.mozilla.org/MPL/2.0/) — see [NOTICE](NOTICE).

Omni and Sidero Labs are trademarks of [Sidero Labs, Inc.](https://www.siderolabs.com/) This project is community-maintained and not affiliated with Sidero Labs.

## Supply chain / SBOM

| Artifact | Generator | When |
|----------|-----------|------|
| `sbom.cyclonedx.json` | [cyclonedx-gomod](https://github.com/CycloneDX/cyclonedx-gomod) | Go modules (CI + release) |
| `sbom-fs.cyclonedx.json` | [Trivy](https://trivy.dev/) filesystem scan | CI |
| `sbom-image.cyclonedx.json` | Trivy image scan | [docker.yml](.github/workflows/docker.yml) |

**Locally:**

```bash
make sbom         # dist/sbom.cyclonedx.json (gitignored)
make hadolint
make docker-build && make trivy-image   # dist/sbom-image.cyclonedx.json
```

**CI:** [ci.yml](.github/workflows/ci.yml) generates and uploads the module SBOM; Trivy scans dependencies (`CRITICAL`/`HIGH`, unfixed ignored). [docker.yml](.github/workflows/docker.yml) runs Hadolint, builds the image, Trivy-scans the Dockerfile and image, and uploads `sbom-image-cyclonedx`.

**Releases:** GoReleaser attaches `sbom.cyclonedx.json` to [GitHub Releases](https://github.com/Jubblin/omni-kubeconfig/releases). [release.yml](.github/workflows/release.yml) Cosign-signs that SBOM blob (`.sig`, `.pem`, `.bundle`). On `v*` tags, [docker.yml](.github/workflows/docker.yml) pushes the image to `ghcr.io/jubblin/omni-kubeconfig`, enables build provenance/SBOM attestations, and keyless-signs the image plus container SBOM with [Sigstore Cosign](https://docs.sigstore.dev/).

## Security

See [SECURITY.md](SECURITY.md). Do not commit kubeconfigs, omniconfig, or PGP keys.

## Fix: CI build failure (`go-api-signature` / gopenpgp)

Tracking issue: [#37](https://github.com/Jubblin/omni-kubeconfig/issues/37)

### Symptom

After Renovate merged `github.com/siderolabs/go-api-signature` **v0.3.13** ([#26](https://github.com/Jubblin/omni-kubeconfig/pull/26)), GitHub Actions on `main` failed across **Test**, **Lint**, all **Build** matrix jobs, and **Docker Build image** with:

```text
omni/client@v1.8.1/pkg/omni/resources/auth/public_key.go:66:21:
cannot use key (*gopenpgp/v2/crypto.Key) as *gopenpgp/v3/crypto.Key in pgp.NewKey
```

### Root cause

| Dependency | gopenpgp API |
|------------|----------------|
| `omni/client` v1.8.1 | **v2** (`public_key.go`) |
| `go-api-signature` v0.3.13 | **v3** (`pgp.NewKey`) |

The two versions are incompatible until `omni/client` aligns with gopenpgp v3 (or `go-api-signature` reverts to v2).

### Fix applied

1. **Pin** `github.com/siderolabs/go-api-signature` to **v0.3.12** in `go.mod` (v2-compatible).
2. **Block Renovate** from re-bumping to v0.3.13+ via `renovate.json` `packageRules` until upstream is compatible.

Verify locally:

```bash
go build ./...
make test
```

### Remove when

Drop the pin and Renovate rule when either:

- `github.com/siderolabs/omni/client` ships a release compatible with `go-api-signature` v0.3.13+, or
- `go-api-signature` documents a supported combination with `omni/client` v1.8.x.

After upgrading, run `go build ./...`, `make test`, and confirm GitHub Actions are green on `main` before closing the tracking issue.
