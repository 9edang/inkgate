# syntax=docker/dockerfile:1

# Build and runtime stages use the same Debian release, so the glibc version is
# the same. That matters for Go plugins (.so): they load only into a binary built
# with the same Go version, dependencies and glibc.
ARG GO_VERSION=1.26
ARG DEBIAN=bookworm

FROM golang:${GO_VERSION}-${DEBIAN} AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=undefined
# cgo stays on: the plugin loader needs it
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GLIBC="$(ldd --version | head -n1 | grep -oE '[0-9]+\.[0-9]+$')" && \
    CGO_ENABLED=1 go build -trimpath \
      -ldflags="-s -w \
        -X github.com/9edang/inkgate/engine/core.InkgateVersion=${VERSION} \
        -X github.com/9edang/inkgate/engine/core.GlibcVersion=${GLIBC}" \
      -o /out/inkgate ./cmd/inkgate

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

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin inkgate \
 && mkdir -p /etc/inkgate /opt/inkgate/plugins \
 && chown -R inkgate /etc/inkgate /opt/inkgate

COPY --from=build /out/inkgate /usr/local/bin/inkgate

USER inkgate
WORKDIR /etc/inkgate

# config:  mount your inkgate.json at /etc/inkgate/inkgate.json
# plugins: mount the .so files at /opt/inkgate/plugins and set
#          "plugin": {"folder": "/opt/inkgate/plugins", "pattern": ".so"}
VOLUME ["/etc/inkgate", "/opt/inkgate/plugins"]
EXPOSE 8080

ENTRYPOINT ["inkgate"]
CMD ["run", "-c", "/etc/inkgate/inkgate.json"]
