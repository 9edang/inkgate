# inkgate vs KrakenD CE

Run on 2026-11-09T19:22:30Z. The median of 3 rounds is shown, picked by requests per second.

## Setup

| | |
|---|---|
| Host | AMD Ryzen 5 5600 6-Core Processor, 12 threads, 8 GB RAM, Linux 7.2.15-300.fc45.x86_64 |
| inkgate | `localhost/inkgate-bench:dev` (built from this repository) |
| KrakenD CE | `docker.io/library/krakend:2.13.11` |
| Backend | `docker.io/library/nginx:1.31.8-alpine`, fixed JSON answers, 2 CPUs |
| Load generator | `ghcr.io/hatoo/oha:1.16.0`, 128 keep-alive connections, 4 CPUs |
| Each gateway | 2 CPUs, 512m memory, one at a time |
| Per run | 10s warm-up, then 30s measured |
| Config | the same file for both: [config/gateway.json](config/gateway.json) |

## Results

### passthrough: one backend, no-op encoding (the body is streamed, never parsed)

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 30,928 | 4.14 ms | 3.70 ms | 7.34 ms | 10.68 ms | 100.00% | 168% | 38 MB |
| KrakenD CE | 22,918 | 5.58 ms | 5.30 ms | 9.65 ms | 12.91 ms | 100.00% | 173% | 39 MB |

inkgate serves **1.35x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [26030, 30928, 31367], KrakenD [19121, 22918, 23136].

### json: one backend, JSON is decoded and encoded again

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 29,390 | 4.35 ms | 3.91 ms | 7.71 ms | 11.04 ms | 100.00% | 169% | 38 MB |
| KrakenD CE | 26,725 | 4.79 ms | 4.36 ms | 8.44 ms | 12.01 ms | 100.00% | 171% | 42 MB |

inkgate serves **1.10x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [25544, 29535, 29390], KrakenD [23081, 26725, 26894].

### merge: two backends called in parallel, both JSON bodies merged into one

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 16,811 | 7.61 ms | 7.20 ms | 11.88 ms | 15.63 ms | 100.00% | 173% | 44 MB |
| KrakenD CE | 15,570 | 8.22 ms | 7.85 ms | 12.77 ms | 16.79 ms | 100.00% | 174% | 44 MB |

inkgate serves **1.08x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [14105, 16925, 16811], KrakenD [15547, 15570, 15655].

### plugin-modifier: one backend, a request modifier plugin adds a header and a response modifier plugin adds a field

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 29,155 | 4.39 ms | 3.91 ms | 7.83 ms | 11.47 ms | 100.00% | 170% | 48 MB |
| KrakenD CE | 27,045 | 4.73 ms | 4.27 ms | 8.26 ms | 11.90 ms | 100.00% | 171% | 44 MB |

inkgate serves **1.08x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [28943, 29155, 29290], KrakenD [27045, 27412, 26089].

### plugin-server: one backend, a server plugin wraps the HTTP handler and adds a response header

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 29,291 | 4.37 ms | 3.91 ms | 7.77 ms | 11.32 ms | 100.00% | 170% | 43 MB |
| KrakenD CE | 26,309 | 4.86 ms | 4.42 ms | 8.57 ms | 12.22 ms | 100.00% | 172% | 42 MB |

inkgate serves **1.11x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [29332, 29291, 28286], KrakenD [26412, 26309, 25652].

### plugin-client: one backend, a client plugin replaces the backend client and calls the API itself

| Gateway | Requests/s | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 20,992 | 6.10 ms | 5.71 ms | 10.72 ms | 14.16 ms | 100.00% | 175% | 46 MB |
| KrakenD CE | 19,612 | 6.52 ms | 6.17 ms | 11.30 ms | 15.36 ms | 100.00% | 175% | 44 MB |

inkgate serves **1.07x** the requests per second of KrakenD CE here. Runs (req/s): inkgate [21026, 20992, 20799], KrakenD [19612, 19619, 19427].

## Plugin cost

Requests per second of each plugin scenario as a share of the same gateway on `json` (one backend, no plugin). The plugin source is the same for both gateways; it is built once with each toolchain.

| Scenario | inkgate | KrakenD CE |
|---|--:|--:|
| plugin-modifier | 99% | 101% |
| plugin-server | 100% | 98% |
| plugin-client | 71% | 73% |

![requests per second](rps.svg)

## Reproduce

```sh
cd benchmark
./run.sh                       # builds the inkgate image, runs everything, rewrites results/
DURATION=60s ROUNDS=5 ./run.sh  # longer, more rounds
```

Numbers depend on the machine; compare the two gateways within one run, not across machines.
