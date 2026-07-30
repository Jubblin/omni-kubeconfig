# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Service-account `kubeconfig` names cluster `omni-{cluster}`, user/context `omni-{cluster}-sa-{user}`; omit `--user` to auto-generate a UUID v8 subject

## [0.3.3] - 2026-07-30

### Changed

- `sync` / `kubeconfig` no longer default `--grant-type` to `auto`; grant type is omitted from the Omni API unless set explicitly (matches `omnictl`, avoids extra `--oidc-redirect-url` in downloaded kubeconfigs)

## [0.3.2] - 2026-07-28

### Changed

- README quality badge now reflects CI **golangci-lint** (replaces Go Report Card); `make lint` falls back to `go run` when the binary is not installed
- Install scripts resolve the latest stable release via GitHub web redirect (with API fallbacks); docs recommend `releases/latest/download/install.sh`

### Fixed

- Install script no longer fails when GitHub API `/releases/latest` returns 404 (e.g. rate limits or missing latest marker)

## [0.3.1] - 2026-07-28

### Added

- One-click install scripts: `scripts/install.sh` (Unix) and `scripts/install.ps1` (Windows) with SHA256 verification against release `sha256sum.txt`
- `update` command to install the latest stable release (self-update) or check for updates (`update --check`)
- Interactive update prompt on `auth` / `sync` / `kubeconfig` when a newer stable release exists (opt-out: `--no-update-check`, `OMNI_KUBECONFIG_SKIP_UPDATE_CHECK=1`)

## [0.3.0] - 2026-07-28

### Added

- `kubeconfig` command to download one cluster kubeconfig, including Kubernetes service-account tokens (`--service-account --user`) matching [Omni docs](https://docs.siderolabs.com/omni/omni-cluster-setup/create-a-kubeconfig-for-a-service-account)

### Fixed

- `.gitignore` `kubeconfig` / `kubeconfig.*` patterns now apply only at the repo root so source files like `kubeconfig.go` are tracked

### Changed

- Snapshot tags are now immutable and numbered (`vX.Y.Z-snapshot.N`); GHCR still promotes floating `vX.Y.Z-snapshot` after Trivy
- CI GoReleaser job skips Docker (`--skip=publish,docker`); multi-arch `dockers_v2` runs on Release only
- Removed `SKIP_BEFORE_TEST` GoReleaser hook; tag releases run `go test` in `release.yml` instead
- Collapse Cosign/SBOM release steps into `scripts/release-attest.sh`
- Split `release.yml` into phased jobs: meta → publish → scan → promote → attest

## [0.2.0] - 2026-07-15

### Added

- `sync --activate-context` opt-in to set kubeconfig `current-context` to the last merged cluster (default preserves existing)
- GoReleaser `dockers_v2` builds and publishes multi-arch GHCR images as part of release/snapshot
- CI pipeline optimizations (scan-before-tag promotion, path filters, `SKIP_BEFORE_TEST` on CI-gated releases)

### Changed

- `sync` merge conflicts now **overwrite** by default; use `--rename-on-conflict` to rename incoming entries instead (replaces sync `--force`)
- `sync` no longer changes `current-context` unless `--activate-context` is set (except cold start / empty `current-context`, which still activates)
- Snapshot and release publishing centralized in GoReleaser (binaries + images)

### Fixed

- Release snapshot GoReleaser dirty-tree abort caused by Trivy writing `.cache/` in the workspace

## [0.1.0] - 2026-06-04

### Added

- `sync` command to download and merge Omni cluster kubeconfigs
- `auth` command for SideroV1 PGP + browser authentication
- Omni API v2 compatibility
- Makefile build with semver injection from git tags

[Unreleased]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.3.3...HEAD
[0.3.3]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.1.2...v0.2.0
[0.1.0]: https://github.com/Jubblin/omni-kubeconfig/releases/tag/v0.1.0
