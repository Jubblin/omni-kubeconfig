MODULE := github.com/Jubblin/omni-kubeconfig
BINARY := omni-kubeconfig
CMD := ./cmd/omni-kubeconfig

# Same matrix as omnictl (siderolabs/omni Dockerfile omnictl-* targets).
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

# Semantic version from git: strip leading "v" (e.g. v0.1.0-3-gabc1234 -> 0.1.0-3-gabc1234).
RAW_VERSION := $(shell git describe --tags --always --dirty --match 'v*' 2>/dev/null | sed 's/^v//')
VERSION ?= $(if $(RAW_VERSION),$(RAW_VERSION),0.1.0-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

GOFLAGS := -buildvcs=false
LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

IMAGE ?= ghcr.io/jubblin/omni-kubeconfig
IMAGE_TAG ?= local
DOCKER ?= docker

# Extra args passed to the binary in docker-run (e.g. ARGS="sync --dry-run").
ARGS ?=

# CycloneDX SBOM from go.mod (https://github.com/CycloneDX/cyclonedx-gomod).
CYCLONEDX_GOMOD := github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.10.0
SBOM_DIR := dist
SBOM_FILE := $(SBOM_DIR)/sbom.cyclonedx.json

HADOLINT ?= docker run --rm -i hadolint/hadolint:v2.12.0-alpine hadolint
TRIVY_IMAGE ?= aquasec/trivy:latest

# Dev container (matches .devcontainer/devcontainer.json)
DEVCONTAINER_IMAGE ?= mcr.microsoft.com/devcontainers/go:1.25
DC_WORKSPACE := /workspaces/omni-kubeconfig
DC_RUN := docker run --rm \
	-v "$(CURDIR):$(DC_WORKSPACE)" \
	-v /var/run/docker.sock:/var/run/docker.sock \
	-w $(DC_WORKSPACE) \
	$(DEVCONTAINER_IMAGE)

.PHONY: build install build-all build-platform test lint check version clean \
	docker-build docker-run hadolint trivy-image sbom-image sbom bom \
	dc-shell dc-check dc-setup pre-commit pre-commit-install

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

# Cross-compile all omnictl-supported platforms into bin/.
build-all:
	@set -e; \
	for platform in $(PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		$(MAKE) build-platform GOOS=$$goos GOARCH=$$goarch; \
	done

# Usage: make build-platform GOOS=linux GOARCH=amd64
build-platform:
	@ext=""; \
	if [ "$(GOOS)" = "windows" ]; then ext=".exe"; fi; \
	echo "Building $(BINARY)-$(GOOS)-$(GOARCH)$$ext"; \
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o bin/$(BINARY)-$(GOOS)-$(GOARCH)$$ext $(CMD)

install:
	go install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(CMD)

test:
	go test $(GOFLAGS) ./...
	bash scripts/next-snapshot-tag_test.sh

lint:
	golangci-lint run ./...

# Run all pre-commit hooks (install hooks first: make pre-commit-install).
pre-commit:
	pre-commit run --all-files

pre-commit-install:
	pre-commit install

check: test lint build build-all

version:
	@echo "version=$(VERSION) commit=$(COMMIT) date=$(DATE)"
	@echo "platforms=$(PLATFORMS)"

# Run an interactive shell in the dev container.
dc-setup:
	$(DC_RUN) bash .devcontainer/post-create.sh

dc-shell:
	$(DC_RUN) bash

# CI-parity checks inside the dev container (installs tools then runs checks).
dc-check:
	$(DC_RUN) bash -lc 'bash .devcontainer/post-create.sh && bash .devcontainer/run-checks.sh'

# Lint Dockerfile (host: docker hadolint; in devcontainer: use make dc-check).
hadolint:
	$(HADOLINT) - < Dockerfile

# Generate CycloneDX JSON SBOM from go.mod (output: dist/sbom.cyclonedx.json).
sbom bom: $(SBOM_FILE)

$(SBOM_FILE):
	@mkdir -p $(SBOM_DIR)
	go run $(CYCLONEDX_GOMOD) mod -licenses -json -output $@ .

clean:
	rm -f bin/$(BINARY)
	rm -f bin/$(BINARY)-*
	rm -rf $(SBOM_DIR)

# Scan built image for vulns and write dist/sbom-image.cyclonedx.json (run docker-build first).
trivy-image sbom-image: docker-build
	@mkdir -p $(SBOM_DIR)
	docker run --rm -v /var/run/docker.sock:/var/run/docker.sock $(TRIVY_IMAGE) \
		image --severity HIGH,CRITICAL --exit-code 1 --ignore-unfixed \
		$(IMAGE):$(IMAGE_TAG)
	docker run --rm \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(CURDIR)/$(SBOM_DIR):/out" \
		$(TRIVY_IMAGE) image --format cyclonedx \
		--output /out/sbom-image.cyclonedx.json \
		$(IMAGE):$(IMAGE_TAG)

# Container image (linux/amd64). Stages GoReleaser dockers_v2 context layout.
DOCKER_CONTEXT := .docker-context

docker-build:
	rm -rf $(DOCKER_CONTEXT)
	mkdir -p $(DOCKER_CONTEXT)/linux/amd64
	$(MAKE) build-platform GOOS=linux GOARCH=amd64 VERSION=$(VERSION)
	cp bin/$(BINARY)-linux-amd64 $(DOCKER_CONTEXT)/linux/amd64/$(BINARY)
	cp Dockerfile $(DOCKER_CONTEXT)/
	$(DOCKER) build \
		--platform linux/amd64 \
		-t $(IMAGE):$(IMAGE_TAG) $(DOCKER_CONTEXT)

# Mount host Omni credentials and kube output dir; run as host user so writes succeed.
docker-run:
	$(DOCKER) run --rm -it \
		--user "$$(id -u):$$(id -g)" \
		-e HOME="$(HOME)" \
		-v "$(HOME)/.talos:$(HOME)/.talos" \
		-v "$(HOME)/.kube:$(HOME)/.kube" \
		$(IMAGE):$(IMAGE_TAG) $(ARGS)
