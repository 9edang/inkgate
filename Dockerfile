# syntax=docker/dockerfile:1

# Build and runtime stages use the same Debian release, so the glibc version is
# the same, on amd64 and arm64. That matters for Go plugins (.so): they load only into a binary built
# with the same Go version, dependencies and glibc.
ARG GO_VERSION=1.26
ARG DEBIAN=bookworm

# The build stage always runs on the builder's own CPU and cross-compiles for the target,
# so an arm64 image can be built on an amd64 machine (and the reverse) without emulation.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-${DEBIAN} AS build
WORKDIR /src

ARG TARGETOS=linux
ARG TARGETARCH

# cgo needs a C cross compiler when the target is not the builder's CPU
RUN if [ "$TARGETARCH" != "$(dpkg --print-architecture)" ]; then \
      case "$TARGETARCH" in \
        arm64) PKGS="gcc-aarch64-linux-gnu libc6-dev-arm64-cross" ;; \
        amd64) PKGS="gcc-x86-64-linux-gnu libc6-dev-amd64-cross" ;; \
        *) echo "unsupported TARGETARCH $TARGETARCH" >&2; exit 1 ;; \
      esac; \
      apt-get update && apt-get install -y --no-install-recommends $PKGS && rm -rf /var/lib/apt/lists/*; \
    fi

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=undefined
# cgo stays on: the plugin loader needs it
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GLIBC="$(ldd --version | head -n1 | grep -oE '[0-9]+\.[0-9]+$')" && \
    if [ "$TARGETARCH" != "$(dpkg --print-architecture)" ]; then \
      case "$TARGETARCH" in arm64) export CC=aarch64-linux-gnu-gcc ;; amd64) export CC=x86_64-linux-gnu-gcc ;; esac; \
    fi && \
    CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
      -ldflags="-s -w \
        -X github.com/9edang/inkgate/engine/core.InkgateVersion=${VERSION} \
        -X github.com/9edang/inkgate/engine/core.GlibcVersion=${GLIBC}" \
      -o /out/inkgate ./cmd/inkgate

# folders owned by the runtime user; the final stage only copies, it never runs a command,
# so it can be assembled for another CPU without emulation
RUN mkdir -p /skel/etc/inkgate /skel/opt/inkgate/plugins \
 && chown -R 10001:10001 /skel

FROM debian:${DEBIAN}-slim

ARG VERSION=undefined
ARG REVISION=unknown
ARG CREATED=unknown
LABEL org.opencontainers.image.title="inkgate" \
      org.opencontainers.image.description="API gateway on gorilla/mux" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/9edang/inkgate"

COPY --from=build /out/inkgate /usr/local/bin/inkgate
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build --chown=10001:10001 /skel/ /

USER 10001:10001
WORKDIR /etc/inkgate

# config:  mount your inkgate.json at /etc/inkgate/inkgate.json
# plugins: mount the .so files at /opt/inkgate/plugins and set
#          "plugin": {"folder": "/opt/inkgate/plugins", "pattern": ".so"}
VOLUME ["/etc/inkgate", "/opt/inkgate/plugins"]
EXPOSE 8080

ENTRYPOINT ["inkgate"]
CMD ["run", "-c", "/etc/inkgate/inkgate.json"]
