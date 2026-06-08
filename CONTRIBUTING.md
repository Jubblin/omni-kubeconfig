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
- Optional: [golangci-lint](https://golangci-lint.run/) v2.12.2 (match CI)
- Optional: [pre-commit](https://pre-commit.com/) for git hook checks

```bash
pip install pre-commit   # or: brew install pre-commit
make pre-commit-install  # installs the git pre-commit hook
make pre-commit          # run all hooks manually
```

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
3. Run checks before opening a PR:
   - **Pre-commit (optional):** hooks run on each commit (fmt, vet, golangci-lint, yaml, Dockerfile hadolint); `make pre-commit` for a full manual pass.
   - **Dev Container (recommended):** `make dc-check` — same image as `.devcontainer/`; runs test, lint, GoReleaser, Hadolint, build, SBOM.
   - **Host:** `make check` (plus `make hadolint` / `make docker-build` for Docker changes).
4. Update [CHANGELOG.md](CHANGELOG.md) under **Unreleased** for user-visible changes.
5. Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages when practical (`feat:`, `fix:`, `docs:`).

## Versioning

This project uses [Semantic Versioning](https://semver.org/). Releases are tagged `vMAJOR.MINOR.PATCH` and built by GitHub Actions (GoReleaser).

Maintainers: update `internal/version/version.go` default only when cutting a new baseline; release tags drive production binaries.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
