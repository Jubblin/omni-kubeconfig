# syntax=docker/dockerfile:1

# Distroless runtime. Expects GoReleaser dockers_v2 context layout:
#   linux/amd64/omni-kubeconfig
#   linux/arm64/omni-kubeconfig
# `make docker-build` stages the same layout under .docker-context/.

FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/omni-kubeconfig /usr/local/bin/omni-kubeconfig

USER nonroot:nonroot
WORKDIR /home/nonroot

ENTRYPOINT ["/usr/local/bin/omni-kubeconfig"]
CMD ["--help"]

# Verify the binary is present and executable (no network or host mounts required).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/omni-kubeconfig", "--version"]
