# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `sync` merge conflicts now **overwrite** by default; use `--rename-on-conflict` to rename incoming entries instead (replaces sync `--force`)

## [0.1.0] - 2026-06-04

### Added

- `sync` command to download and merge Omni cluster kubeconfigs
- `auth` command for SideroV1 PGP + browser authentication
- Omni API version 2 compatibility
- Makefile build with semver injection from git tags

[Unreleased]: https://github.com/Jubblin/omni-kubeconfig/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Jubblin/omni-kubeconfig/releases/tag/v0.1.0
