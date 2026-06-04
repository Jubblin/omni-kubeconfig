# syntax=docker/dockerfile:1

# Multi-stage build: static Linux binary, minimal runtime image.
# At runtime mount host ~/.talos (omniconfig + SideroV1 PGP keys) and optionally ~/.kube for sync output.

FROM golang:1.25-alpine AS builder

WORKDIR /src

# Versions from golang:1.25-alpine (Alpine 3.23); bump when rebasing the builder image.
RUN apk add --no-cache \
    ca-certificates=20260413-r0 \
    git=2.52.0-r0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=0.0.0-dev
ARG COMMIT=unknown
ARG DATE=unknown

ENV CGO_ENABLED=0 GOOS=linux

RUN go build -buildvcs=false -trimpath \
    -ldflags "-s -w \
      -X github.com/Jubblin/omni-kubeconfig/internal/version.Version=${VERSION} \
      -X github.com/Jubblin/omni-kubeconfig/internal/version.Commit=${COMMIT} \
      -X github.com/Jubblin/omni-kubeconfig/internal/version.Date=${DATE}" \
    -o /out/omni-kubeconfig ./cmd/omni-kubeconfig

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/omni-kubeconfig /usr/local/bin/omni-kubeconfig

USER nonroot:nonroot
WORKDIR /home/nonroot

ENTRYPOINT ["/usr/local/bin/omni-kubeconfig"]
CMD ["--help"]

# Verify the binary is present and executable (no network or host mounts required).
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/omni-kubeconfig", "--version"]
