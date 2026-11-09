#!/usr/bin/env bash
# Benchmarks inkgate against KrakenD CE with the same config, the same CPU and memory limits and the same
# load generator, all in containers (docker or podman). The gateways run one at a time, never together.
#
#   ./run.sh                          build the inkgate image, run everything, write results/report.md
#   DURATION=60s ROUNDS=5 ./run.sh    longer runs
#   SKIP_BUILD=1 ./run.sh             reuse the inkgate image already built
set -euo pipefail
cd "$(dirname "$0")"

DOCKER="${DOCKER:-docker}"
INKGATE_IMAGE="${INKGATE_IMAGE:-localhost/inkgate-bench:dev}"
KRAKEND_IMAGE="${KRAKEND_IMAGE:-docker.io/library/krakend:2.13.11}"
NGINX_IMAGE="${NGINX_IMAGE:-docker.io/library/nginx:1.31.8-alpine}"
OHA_IMAGE="${OHA_IMAGE:-ghcr.io/hatoo/oha:1.16.0}"
INKGATE_BUILD_IMAGE="${INKGATE_BUILD_IMAGE:-localhost/inkgate-bench-build:dev}"   # the Dockerfile build stage, builds the plugins
KRAKEND_BUILDER_IMAGE="${KRAKEND_BUILDER_IMAGE:-docker.io/krakend/builder:2.13.11}" # builds plugins that KrakenD can load

DURATION="${DURATION:-30s}"     # measured run
WARMUP="${WARMUP:-10s}"         # not measured
CONNECTIONS="${CONNECTIONS:-128}"
ROUNDS="${ROUNDS:-3}"           # each scenario runs this many times, the median is reported
GATEWAY_CPUS="${GATEWAY_CPUS:-2}"
GATEWAY_MEMORY="${GATEWAY_MEMORY:-512m}"
BACKEND_CPUS="${BACKEND_CPUS:-2}"
LOAD_CPUS="${LOAD_CPUS:-4}"
SCENARIOS=(passthrough json merge plugin-modifier plugin-server plugin-client)

# What a scenario's answer must contain (headers included), checked before it is measured: a broken plugin or config
# answers errors quickly and would otherwise look fast. Patterns are separated by "|".
declare -A EXPECT=(
  [passthrough]='"service":"backend"'
  [json]='"name":"Ada Lovelace"'
  [merge]='"name":"Ada Lovelace"|"count":3'
  [plugin-modifier]='"bench":"modifier"|"bench_req":"1"'
  [plugin-server]='X-Bench-Server: 1|"name":"Ada Lovelace"'
  [plugin-client]='"name":"Ada Lovelace"'
)

NET=inkbench
RAW=results/raw

cleanup() {
  $DOCKER rm -f bench-gateway bench-backend >/dev/null 2>&1 || true
  $DOCKER network rm $NET >/dev/null 2>&1 || true
}
trap cleanup EXIT

load() { # load <extra oha args...>
  $DOCKER run --rm --network $NET --cpus "$LOAD_CPUS" "$OHA_IMAGE" --no-tui "$@"
}

fetch() { # fetch <url>: headers and body of one request (the nginx image has a busybox wget)
  $DOCKER run --rm --network $NET --entrypoint wget "$NGINX_IMAGE" -q -S -O - "$1" 2>&1
}

wait_ready() { # wait_ready <scenario>: wait for the gateway, then check the answer
  local url="http://bench-gateway:8080/$1" out="" pattern
  for _ in $(seq 1 60); do
    if out=$(fetch "$url"); then break; fi
    out=""; sleep 0.5
  done
  if [ -z "$out" ]; then
    echo "gateway did not become ready for $1" >&2; $DOCKER logs bench-gateway >&2 || true; exit 1
  fi
  local IFS='|'
  for pattern in ${EXPECT[$1]}; do
    if ! grep -qiF -- "$pattern" <<<"$out"; then
      echo "scenario $1: the answer does not contain: $pattern" >&2
      echo "$out" >&2; $DOCKER logs bench-gateway >&2 || true; exit 1
    fi
  done
}

start_gateway() { # start_gateway <inkgate|krakend> <scenario>
  local config="config/gateway.json"
  [ -f "config/$2.json" ] && config="config/$2.json"   # a scenario may bring its own config (plugins)
  case "$1" in
    inkgate) $DOCKER run -d --name bench-gateway --network $NET --cpus "$GATEWAY_CPUS" --memory "$GATEWAY_MEMORY" \
      -v "$PWD/$config:/etc/inkgate/inkgate.json:ro,z" -v "$PWD/.plugins/inkgate:/opt/plugins:ro,z" \
      "$INKGATE_IMAGE" >/dev/null ;;
    krakend) $DOCKER run -d --name bench-gateway --network $NET --cpus "$GATEWAY_CPUS" --memory "$GATEWAY_MEMORY" \
      -v "$PWD/$config:/etc/krakend/krakend.json:ro,z" -v "$PWD/.plugins/krakend:/opt/plugins:ro,z" \
      "$KRAKEND_IMAGE" run -c /etc/krakend/krakend.json >/dev/null ;;
  esac
  wait_ready "$2"
}

# Every plugin below plugins/ is a tiny module of its own (standard library only), except client-inkgate: it imports the
# inkgate context package, so it is built inside the repo module. Server and modifier are the same source for both
# gateways; the client plugin has two sources with the same logic (client-krakend and client-inkgate).
# Go loads a plugin only into a binary built the same way: the inkgate ones use the Dockerfile build stage, the KrakenD
# ones the KrakenD builder image.
build_plugins() {
  rm -rf .plugins && mkdir -p .plugins/inkgate .plugins/krakend
  $DOCKER build --target build -t "$INKGATE_BUILD_IMAGE" --build-arg VERSION=bench -f ../Dockerfile ..
  $DOCKER run --rm -v "$PWD/plugins:/src/benchmark/plugins:z" -v "$PWD/.plugins/inkgate:/out:z" -w /src/benchmark/plugins \
    "$INKGATE_BUILD_IMAGE" sh -c 'export CGO_ENABLED=1
      for p in modifier server; do (cd $p && go build -trimpath -buildmode=plugin -o /out/$p.so .) || exit 1; done
      cd /src && go build -trimpath -buildmode=plugin -o /out/client.so ./benchmark/plugins/client-inkgate'
  $DOCKER run --rm -v "$PWD/plugins:/app:z" -v "$PWD/.plugins/krakend:/out:z" -w /app "$KRAKEND_BUILDER_IMAGE" \
    sh -c 'for p in modifier server client-krakend; do (cd $p && go build -buildmode=plugin -o /out/${p%-krakend}.so .) || exit 1; done'
}

sample_stats() { # sample_stats <file>: one "cpu%|memory" line per second until killed
  while true; do
    $DOCKER stats --no-stream --format '{{.CPUPerc}}|{{.MemUsage}}' bench-gateway >>"$1" 2>/dev/null || true
    sleep 1
  done
}

if [ -z "${SKIP_BUILD:-}" ]; then
  $DOCKER build -t "$INKGATE_IMAGE" --build-arg VERSION=bench -f ../Dockerfile ..
  build_plugins
fi
for img in "$KRAKEND_IMAGE" "$NGINX_IMAGE" "$OHA_IMAGE" "$KRAKEND_BUILDER_IMAGE"; do $DOCKER image inspect "$img" >/dev/null 2>&1 || $DOCKER pull "$img"; done

rm -rf results && mkdir -p $RAW
cleanup
$DOCKER network create $NET >/dev/null
$DOCKER run -d --name bench-backend --network $NET --cpus "$BACKEND_CPUS" \
  -v "$PWD/backend/nginx.conf:/etc/nginx/nginx.conf:ro,z" "$NGINX_IMAGE" >/dev/null

cat >results/meta.json <<META
{
  "date": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "host": {"cpu": "$(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2 | xargs)", "threads": $(nproc), "memory_gb": $(awk '/MemTotal/ {printf "%.0f", $2/1048576}' /proc/meminfo), "kernel": "$(uname -sr)"},
  "images": {"inkgate": "$INKGATE_IMAGE", "krakend": "$KRAKEND_IMAGE", "backend": "$NGINX_IMAGE", "load": "$OHA_IMAGE"},
  "limits": {"gateway_cpus": $GATEWAY_CPUS, "gateway_memory": "$GATEWAY_MEMORY", "backend_cpus": $BACKEND_CPUS, "load_cpus": $LOAD_CPUS},
  "run": {"duration": "$DURATION", "warmup": "$WARMUP", "connections": $CONNECTIONS, "rounds": $ROUNDS},
  "scenarios": [$(printf '"%s", ' "${SCENARIOS[@]}" | sed 's/, $//')]
}
META

for round in $(seq 1 "$ROUNDS"); do
  for scenario in "${SCENARIOS[@]}"; do
    # alternate who goes first so neither always runs on a cooler or hotter machine
    if [ $((round % 2)) -eq 1 ]; then order="inkgate krakend"; else order="krakend inkgate"; fi
    for gw in $order; do
      echo "== round $round/$ROUNDS  $scenario  $gw"
      start_gateway "$gw" "$scenario"
      url="http://bench-gateway:8080/$scenario"
      load -z "$WARMUP" -c "$CONNECTIONS" --output-format quiet "$url" >/dev/null
      stats="$RAW/$gw-$scenario-r$round.stats"; : >"$stats"
      sample_stats "$stats" & sampler=$!
      load -z "$DURATION" -c "$CONNECTIONS" --output-format json "$url" >"$RAW/$gw-$scenario-r$round.json"
      kill "$sampler" 2>/dev/null || true; wait "$sampler" 2>/dev/null || true
      $DOCKER rm -f bench-gateway >/dev/null
    done
  done
done

python3 -I report.py
echo "report: $PWD/results/report.md"
