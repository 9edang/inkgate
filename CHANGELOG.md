# Changelog

All notable changes to inkgate are written here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the versions follow
[Semantic Versioning](https://semver.org/). Docker image tags use the same version
(see `scripts/docker-release.sh`).

## [Unreleased]

## [0.1.0-beta.2] - 2026-11-04

### Added
- Multi-architecture Docker image for `linux/amd64` and `linux/arm64/v8`, and release binaries for both.
  The image is cross-compiled with cgo (no emulation needed) and still ships glibc 2.36, so plugins built on
  Debian bookworm load on both.

### Changed
- The runtime image no longer runs commands at build time (non-root user is set by numeric id), so it can be
  assembled for another CPU.
- `scripts/docker-release.sh` builds and pushes a manifest list for `PLATFORMS`
  (default `linux/amd64,linux/arm64/v8`) with podman or docker buildx.

## [0.1.0-beta.1] - 2026-11-04

First public pre-release. It is a beta: configuration keys and behavior can still change before 0.1.0.

### Added
- The gateway on the gorilla/mux router: security and traffic rules, calls to one or more backends, merged
  responses.
- Server-Sent Events and WebSocket proxying.
- Chainable `plugin/http-client` plugins, and the `plugin/http-server` and `plugin/req-resp-modifier` plugin types.
- Commands `run`, `check`, `check-plugin`, `test-plugin`, `audit` and `version`.
- `Dockerfile` and `.dockerignore`: multi-stage build on Debian, non-root user, plugin folder `/opt/inkgate/plugins`.
- `scripts/docker-release.sh`: computes the next version from the git tags and builds or pushes the image.
  Releases get `X.Y.Z`, `X.Y`, `X` and `latest`; pre-releases get their own tag and `beta` or `rc`, never `latest`.
- `make lint`, `make sec`, `make vuln`, `make check-all` and `make tools`, with reports in `bin/reports`
  and CPU and memory limits for the scans; `.golangci.yml` configuration.
- [Custom plugins guide](docs/guides/custom-plugins.md): examples, build and CLI commands, and the flow with
  several plugins on one request.

### Known issues
- `make lint`, `make sec` and `make vuln` still report findings, including vulnerabilities in dependencies
  that the code calls. See `bin/reports` after running `make check-all`.

[Unreleased]: https://github.com/9edang/inkgate/compare/v0.1.0-beta.2...HEAD
[0.1.0-beta.2]: https://github.com/9edang/inkgate/compare/v0.1.0-beta.1...v0.1.0-beta.2
[0.1.0-beta.1]: https://github.com/9edang/inkgate/releases/tag/v0.1.0-beta.1
