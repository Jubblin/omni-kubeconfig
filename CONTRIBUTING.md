# Contributing

Thank you for contributing to omni-kubeconfig.

## Development setup

```bash
git clone https://github.com/Jubblin/omni-kubeconfig.git
cd omni-kubeconfig
git tag v0.1.0   # optional: enables make VERSION from tag
```

### Dev Container (recommended)

Open the repository in VS Code / Cursor and choose **Reopen in Container**. The devcontainer installs Go, golangci-lint, and GitHub CLI.

### Local setup

Requirements:

- Go 1.25+ (see `go.mod`)
- Make
- Optional: [golangci-lint](https://golangci-lint.run/)

```bash
git clone https://github.com/Jubblin/omni-kubeconfig.git
cd omni-kubeconfig
make test
make build
make build-all   # optional: verify all omnictl platforms cross-compile
```

Configure Omni access on your machine (`~/.talos/omni/config` and PGP keys) before running integration tests against a live server.

## Pull requests

1. Fork the repository and create a branch from `main`.
2. Make focused changes; keep PRs small when possible.
3. Run `make check` before opening a PR. For Docker changes, also run `make hadolint` and `make docker-build` (CI runs Hadolint, Trivy, and SBOM/signing separately).
4. Update [CHANGELOG.md](CHANGELOG.md) under **Unreleased** for user-visible changes.
5. Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages when practical (`feat:`, `fix:`, `docs:`).

## Versioning

This project uses [Semantic Versioning](https://semver.org/). Releases are tagged `vMAJOR.MINOR.PATCH` and built by GitHub Actions (GoReleaser).

Maintainers: update `internal/version/version.go` default only when cutting a new baseline; release tags drive production binaries.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
