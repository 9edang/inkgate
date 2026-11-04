# Inkgate, The Plugin-Ready API Gateway

Inkgate is an API gateway on the gorilla/mux router. It receives an HTTP request, applies security and traffic rules,
calls one or more backends, merges their answers and returns one response.

> **Pre-release.** Configuration keys and behavior can still change before 0.1.0.

## Features
- Routing to one or more backends, with merged responses
- Server-Sent Events and WebSocket proxying
- Go plugins: `plugin/http-server`, `plugin/http-client` (chainable), `plugin/req-resp-modifier`
- Commands: `run`, `check`, `check-plugin`, `test-plugin`, `audit`, `version`
- Runs as a non-root user (uid 10001)

## Tags
| Tag | Meaning |
|---|---|
| `0.1.0-beta.2` | a specific pre-release |
| `beta` | the latest pre-release |

Images are multi-architecture: `linux/amd64` and `linux/arm64/v8`. Docker pulls the one for your CPU.

## Quick start
```bash
docker run --rm -p 8080:8080 \
  -v "$PWD/inkgate.json:/etc/inkgate/inkgate.json:ro" \
  120200/inkgate:0.1.0-beta.2
```

| Path in the container | Use |
|---|---|
| `/etc/inkgate/inkgate.json` | your configuration (the default command is `inkgate run -c /etc/inkgate/inkgate.json`) |
| `/opt/inkgate/plugins` | your plugin `.so` files |
| port `8080` | the gateway (the port set in your config) |

## Plugins
Mount your `.so` files and point the config at them:
```bash
docker run --rm -p 8080:8080 \
  -v "$PWD/inkgate.json:/etc/inkgate/inkgate.json:ro" \
  -v "$PWD/out:/opt/inkgate/plugins:ro" \
  120200/inkgate:0.1.0-beta.2
```
```json
"plugin": { "folder": "/opt/inkgate/plugins", "pattern": ".so" }
```
Plugins must be built with the same Go version, dependency versions and glibc as the image
(Go 1.26, Debian bookworm, glibc 2.36). Check them with `inkgate test-plugin`.

## Links
- Source, docs and releases: https://github.com/9edang/inkgate
- Plugin guide: https://github.com/9edang/inkgate/blob/main/docs/guides/custom-plugins.md
- Changelog: https://github.com/9edang/inkgate/blob/main/CHANGELOG.md

License: Apache-2.0
