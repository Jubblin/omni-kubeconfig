# omni-kubeconfig

[![CI](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/ci.yml/badge.svg)](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/ci.yml)
[![Release](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/release.yml/badge.svg)](https://github.com/Jubblin/omni-kubeconfig/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![golangci-lint](https://img.shields.io/github/actions/workflow/status/Jubblin/omni-kubeconfig/ci.yml?branch=main&label=golangci-lint&logo=go)](https://github.com/Jubblin/omni-kubeconfig/actions/workflows/ci.yml)

Download admin kubeconfigs for every cluster on a [Sidero Omni](https://docs.siderolabs.com/omni) server and merge them into a single file for `kubectl`.

## Features

- **`auth`** — SideroV1 PGP + browser login (same flow as `omnictl`)
- **`sync`** — List all Omni clusters, download OIDC admin kubeconfigs, merge into one file
- **`kubeconfig`** — Download one cluster kubeconfig; optional `--service-account` for token-based access (no kubelogin)
- **`update`** — Install the latest release (self-update) or check for updates
- One-click install via `scripts/install.sh` / `scripts/install.ps1` with SHA256 verification
- Interactive update prompt when a newer stable release exists (opt-out flags available)
- Incremental merge into existing output by default (`--merge-existing`); full replace with `--merge-existing=false`
- Name conflicts overwrite by default; `--rename-on-conflict` keeps both (incoming → `name-1`)
- Preserves existing `current-context` by default (cold starts / empty `current-context` still get one); `--activate-context` sets it to the last merged cluster
- Backups existing output as `*.bak.<timestamp>` before overwrite
- Omni API v2 compatible

## Quick start

### One-click install (recommended)

```bash
curl -fsSL https://github.com/Jubblin/omni-kubeconfig/releases/latest/download/install.sh | bash
```

Windows (PowerShell):

```powershell
irm https://github.com/Jubblin/omni-kubeconfig/releases/latest/download/install.ps1 | iex
```

Installs a checksum-verified binary to `~/.local/bin` (Unix) or `%LOCALAPPDATA%\Programs\omni-kubeconfig` (Windows). Add the directory to your `PATH` if prompted.

Upgrade later with `omni-kubeconfig update` or re-run the install script. On interactive runs, the tool prompts when a newer stable release is available (disable with `--no-update-check` or `OMNI_KUBECONFIG_SKIP_UPDATE_CHECK=1`).

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
export KUBECONFIG=~/.kube/config
kubectl config get-contexts
```

See [Usage](#usage) and [Reference](#reference) below for all flags.

## Run with Docker

Container images are published to [GitHub Container Registry](https://github.com/Jubblin/omni-kubeconfig/pkgs/container/omni-kubeconfig) as `ghcr.io/jubblin/omni-kubeconfig` on every push to `main` (snapshot) and on `vX.Y.Z` release tags. GoReleaser `dockers_v2` pushes an immutable `sha-<commit>` tag first; Trivy scans that digest, then mutable tags are promoted. [docker.yml](.github/workflows/docker.yml) only lints the Dockerfile when it changes.

The image contains only the `omni-kubeconfig` binary (distroless, no shell). Mount your host Omni credentials and kube output directory; run `kubectl` on the host against the merged file.

### Image tags

| Tag | When |
|-----|------|
| `v0.3.3-snapshot` | Floating latest from `main` for the current release line (after Trivy) |
| `v0.3.3-snapshot.N` | Immutable numbered snapshot for a specific `main` build (after Trivy) |
| `latest` | Most recent release (after Trivy) |
| `0.1.2` | Exact semver (no `v` prefix), after Trivy |
| `0.1` | Major.minor alias on release |
| `sha-<commit>` | Immutable digest pointer (pushed before scan; safe if scan fails) |

```bash
# After merging to main (release line v0.3.3)
docker pull ghcr.io/jubblin/omni-kubeconfig:v0.3.3-snapshot
docker pull ghcr.io/jubblin/omni-kubeconfig:v0.3.3-snapshot.3

# After tagging v0.1.2
docker pull ghcr.io/jubblin/omni-kubeconfig:0.1.2
docker pull ghcr.io/jubblin/omni-kubeconfig:latest
```

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

# 2) Sync all clusters into ~/.kube/config on the host
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  -e HOME="$HOME" \
  -v "$HOME/.talos:$HOME/.talos" \
  -v "$HOME/.kube:$HOME/.kube" \
  "$OKC_IMAGE" sync

# 3) Use kubectl on the host
export KUBECONFIG="$HOME/.kube/config"
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
| `~/.kube` | `$HOME/.kube` | Merged kubeconfig (default `~/.kube/config`) |

### Build and run locally

```bash
make docker-build                    # cross-compile linux/amd64 binary, then image
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
| [ci.yml](.github/workflows/ci.yml) | Push/PR to `main` (skips docs / `.cursor`-only) | Test, vet, [golangci-lint](.golangci.yml) (v2.12.2), GoReleaser snapshot build (`--skip=publish`), Trivy FS vuln scan; on `main` calls Release |
| [docker.yml](.github/workflows/docker.yml) | Dockerfile path changes | Hadolint + Dockerfile Trivy config scan |
| [release.yml](.github/workflows/release.yml) | CI success on `main`; tag `vX.Y.Z` | GoReleaser binaries + GH release + GHCR (`sha-` then promote after Trivy), Cosign |

[Dependabot](.github/dependabot.yml) updates Go modules and GitHub Actions weekly.

### Releasing (maintainers)

```bash
# Ensure CHANGELOG.md is updated, then:
git tag v0.3.3
git push origin v0.3.3
```

GoReleaser publishes bare binaries (naming matches `omnictl-*`) for darwin/linux amd64+arm64 and windows amd64 only, and builds/pushes the multi-arch container image via `dockers_v2`. See [.goreleaser.yaml](.goreleaser.yaml).

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

Sync all clusters into `~/.kube/config`:

```bash
omni-kubeconfig sync
```

Use the merged config:

```bash
export KUBECONFIG=~/.kube/config
kubectl get nodes --context <cluster-name>
```

On success, `sync` prints `export KUBECONFIG=<path>` when `-o` is not the default `~/.kube/config` (disable with `--print-export=false`).

### Service-account kubeconfig (token)

For CI or other non-interactive cluster access, mint a **Kubernetes** service-account kubeconfig (not an Omni API service account):

```bash
omni-kubeconfig kubeconfig \
  --service-account \
  --cluster prod \
  --user ci-deploy \
  --ttl 720h \
  -o ./prod-ci.kubeconfig \
  --merge-existing=false \
  --force
```

Use that file with `kubectl` directly (embedded token; no `kubelogin`). See the [Omni guide](https://docs.siderolabs.com/omni/omni-cluster-setup/create-a-kubeconfig-for-a-service-account).

To authenticate *this tool* to Omni without a browser, use an Omni service account key (`OMNI_SERVICE_ACCOUNT_KEY`) instead — that is separate from cluster SA kubeconfigs.

### Docker

Run the same commands inside the published image; mount `~/.talos` and `~/.kube`, use `--user "$(id -u):$(id -g)"`, and keep `kubectl` on the host. See [Run with Docker](#run-with-docker).

## Reference

`omni-kubeconfig --help`, `auth --help`, `sync --help`, `kubeconfig --help`, and `update --help` mirror this section.

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--omniconfig` | `OMNICONFIG` or `~/.talos/omni/config` | Path to omniconfig |
| `--context` | Selected context | Omni context name |
| `--insecure-skip-tls-verify` | `false` | Skip TLS verification for Omni API |
| `--siderov1-keys-dir` | `SIDEROV1_KEYS_DIR` or `~/.talos/keys` | SideroV1 PGP keys directory |
| `--no-update-check` | `false` | Disable check for newer releases before running a command |
| `--check-updates` | `false` | Force update check (ignore 24h cache) |
| `-h`, `--help` | | Show help |
| `-v`, `--version` | | Semver, build metadata, Omni API level |

### `auth` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Delete PGP key and force new browser login |

### `sync` flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o`, `--output` | `~/.kube/config` | Merged kubeconfig path |
| `-c`, `--cluster` | all | Sync only these clusters (repeatable) |
| `--merge-existing` | `true` | Load existing output and merge; `false` replaces with clusters synced this run |
| `--rename-on-conflict` | `false` | Rename conflicting entries instead of overwriting (default overwrites) |
| `--activate-context` | `false` | Set `current-context` to the last cluster merged; default preserves existing (activates on empty/`--merge-existing=false` cold start) |
| `--grant-type` | *(omitted)* | OIDC grant: `auto`, `authcode`, `authcode-keyboard`; omit for kubelogin defaults (matches `omnictl`) |
| `--dry-run` | `false` | List clusters only |
| `--print-export` | `true` | Print `export KUBECONFIG=...` when `-o` is not `~/.kube/config` |

### `kubeconfig` flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o`, `--output` | `~/.kube/config` | Kubeconfig path |
| `-c`, `--cluster` | (required) | Omni cluster name |
| `--service-account` | `false` | Mint a Kubernetes SA token kubeconfig instead of OIDC |
| `--user` | | Token `sub` (required with `--service-account`) |
| `--ttl` | `8760h` (365d) | SA token TTL |
| `--groups` | `system:masters` | SA token groups |
| `--merge-existing` | `true` | Merge into existing output; `false` writes only this cluster |
| `--force` | `false` | Overwrite existing file when `--merge-existing=false` |
| `--rename-on-conflict` | `false` | Rename conflicting entries instead of overwriting |
| `--activate-context` | `false` | Set `current-context` to this cluster |
| `--grant-type` | *(omitted)* | OIDC grant for non-SA kubeconfigs; set explicitly when needed |
| `--break-glass` | `false` | Bypass Omni when enabled for the account |
| `--print-export` | `true` | Print `export KUBECONFIG=...` when `-o` is not `~/.kube/config` |

### `update` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | latest stable | Install a specific release tag (e.g. `v0.3.3`) |
| `--install-dir` | running executable | Install to this directory instead of self-replace |
| `--check` | `false` | Report if a newer stable release exists (exit 1 if outdated) |

### Environment variables

| Variable | Description |
|----------|-------------|
| `OMNICONFIG` | Omniconfig path |
| `OMNI_ENDPOINT` | Override Omni API URL |
| `OMNI_SERVICE_ACCOUNT_KEY` | Base64 Omni API service account (non-interactive tool auth; also `SIDERO_SERVICE_ACCOUNT_KEY`) |
| `OMNI_KUBECONFIG_SKIP_UPDATE_CHECK` | Set to `1` to disable update prompts/checks |
| `SIDEROV1_KEYS_DIR` | PGP keys directory |
| `BROWSER` | Set to `echo` to print login URL instead of opening a browser |

### Examples

```bash
BROWSER=echo omni-kubeconfig auth
omni-kubeconfig sync --dry-run
omni-kubeconfig sync -o ~/.kube/omni-prod -c prod -c staging
omni-kubeconfig sync --rename-on-conflict      # keep both on name clash (incoming → name-1)
omni-kubeconfig sync --activate-context       # set current-context to last merged cluster
omni-kubeconfig sync --merge-existing=false   # drop contexts from prior syncs not in this run
omni-kubeconfig sync --grant-type authcode-keyboard
omni-kubeconfig kubeconfig -c prod
omni-kubeconfig kubeconfig --service-account -c prod --user ci-deploy -o ./prod-ci.kubeconfig --merge-existing=false --force
curl -fsSL https://github.com/Jubblin/omni-kubeconfig/releases/latest/download/install.sh | bash
omni-kubeconfig update
omni-kubeconfig update --check
omni-kubeconfig sync --no-update-check
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
2. **`sync`**: lists `Clusters.omni.sidero.dev`, downloads each OIDC admin kubeconfig, merges
3. **`kubeconfig`**: downloads one cluster kubeconfig (OIDC, or SA token with `--service-account`)
4. Merges with [go-kubeconfig](https://github.com/siderolabs/go-kubeconfig) (into existing output when `--merge-existing`, default)
5. Backs up existing output before writing

## Project layout

```
.
├── cmd/omni-kubeconfig/    # CLI entrypoint
├── internal/omni/          # Omni client, auth, sync
├── internal/version/       # Semver metadata
├── .devcontainer/          # Dev Container spec
├── Dockerfile              # Distroless image (copies GoReleaser linux binaries from dist/)
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
| `sbom.cyclonedx.json` | [cyclonedx-gomod](https://github.com/CycloneDX/cyclonedx-gomod) via GoReleaser | Releases (source of truth); local `make sbom` |
| `sbom-image.cyclonedx.json` | [Trivy](https://trivy.dev/) image scan | [release.yml](.github/workflows/release.yml) after GHCR publish |

**Locally:**

```bash
make sbom         # dist/sbom.cyclonedx.json (gitignored)
make hadolint
make docker-build && make trivy-image   # dist/sbom-image.cyclonedx.json
```

**CI:** [ci.yml](.github/workflows/ci.yml) runs Trivy filesystem vuln scan (`CRITICAL`/`HIGH`, unfixed ignored). Module SBOM is not uploaded as a CI artifact — GoReleaser owns it on release. [docker.yml](.github/workflows/docker.yml) runs Hadolint and Dockerfile config scan when the Dockerfile changes.

**Releases:** GoReleaser attaches `sbom.cyclonedx.json` to [GitHub Releases](https://github.com/Jubblin/omni-kubeconfig/releases). [release.yml](.github/workflows/release.yml) Cosign-signs that SBOM bundle, pushes `ghcr.io/jubblin/omni-kubeconfig:sha-<commit>`, Trivy-scans the digest, promotes mutable tags only after a clean scan, then keyless-signs the image plus container SBOM with [Sigstore Cosign](https://docs.sigstore.dev/).

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
