# syntax=docker/dockerfile:1

# Distroless runtime image. Expects GoReleaser linux binaries in dist/:
#   dist/omni-kubeconfig-linux-amd64
#   dist/omni-kubeconfig-linux-arm64
# CI and `make docker-build` stage the amd64 binary; release builds multi-arch.

FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETARCH
COPY dist/omni-kubeconfig-linux-${TARGETARCH} /usr/local/bin/omni-kubeconfig

USER nonroot:nonroot
WORKDIR /home/nonroot

ENTRYPOINT ["/usr/local/bin/omni-kubeconfig"]
CMD ["--help"]

# Verify the binary is present and executable (no network or host mounts required).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/omni-kubeconfig", "--version"]
