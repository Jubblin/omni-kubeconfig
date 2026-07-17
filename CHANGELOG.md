# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Snapshot tags are now immutable and numbered (`vX.Y.Z-snapshot.N`); GHCR still promotes floating `vX.Y.Z-snapshot` after Trivy
- CI GoReleaser job skips Docker (`--skip=publish,docker`); multi-arch `dockers_v2` runs on Release only
- Removed `SKIP_BEFORE_TEST` GoReleaser hook; tag releases run `go test` in `release.yml` instead

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

[Unreleased]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.1.2...v0.2.0
[0.1.0]: https://github.com/Jubblin/omni-kubeconfig/releases/tag/v0.1.0
