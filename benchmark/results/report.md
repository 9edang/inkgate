# inkgate vs KrakenD CE

Run on 2026-11-13T05:59:25Z. 7 rounds of each scenario: the table shows the median round (by requests per second) and the range, lowest to highest round. When the ranges of the two gateways overlap the lead is marked "within noise": it is not a result.

Go: inkgate 1.27.4, KrakenD CE 1.26.8. They are different, so the comparison includes the Go toolchain and runtime, not only the gateway code.

## Setup

| | |
|---|---|
| Host | AMD Ryzen 5 5600 6-Core Processor, 12 threads, 8 GB RAM, Linux 7.3.3-300.fc45.x86_64 |
| inkgate | `localhost/inkgate-bench:dev` (built from this repository), revision `1c37dfc`, Go 1.27.4, glibc 2.41 |
| KrakenD CE | `docker.io/library/krakend:2.13.11`, built with Go 1.26.8 (MUSL-1.2.5_(alpine-3.23.5)) |
| Backend | `docker.io/library/nginx:1.31.8-alpine`, fixed JSON answers, 2 CPUs |
| Load generator | `ghcr.io/hatoo/oha:1.16.0`, 128 keep-alive connections, 4 CPUs |
| Each gateway | 2 CPUs, 512m memory, one at a time |
| Per run | 30s warm-up, then 30s measured |
| Config | the same file for both: [config/gateway.json](../config/gateway.json) |

## Results

### passthrough: one backend, no-op encoding (the body is streamed, never parsed)

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 32,469 (32,310 to 32,536) | 3.94 ms | 3.52 ms | 7.03 ms | 10.19 ms | 100.00% | 171% | 39 MB |
| KrakenD CE | 23,402 (23,300 to 23,551) | 5.47 ms | 5.18 ms | 9.47 ms | 12.70 ms | 100.00% | 175% | 40 MB |

inkgate against KrakenD CE: **1.39x**. Runs (req/s): inkgate [32374, 32354, 32514, 32536, 32498, 32469, 32310], KrakenD [23373, 23304, 23551, 23445, 23441, 23300, 23402].

### json: one backend, JSON is decoded and encoded again

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 30,390 (30,132 to 30,668) | 4.21 ms | 3.78 ms | 7.48 ms | 10.78 ms | 100.00% | 172% | 38 MB |
| KrakenD CE | 27,496 (27,454 to 27,611) | 4.65 ms | 4.23 ms | 8.19 ms | 11.76 ms | 100.00% | 173% | 43 MB |

inkgate against KrakenD CE: **1.11x**. Runs (req/s): inkgate [30132, 30491, 30256, 30252, 30390, 30424, 30668], KrakenD [27611, 27588, 27454, 27469, 27557, 27486, 27496].

### merge: two backends called in parallel, both JSON bodies merged into one

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 17,242 (17,219 to 17,349) | 7.42 ms | 7.02 ms | 11.63 ms | 15.27 ms | 100.00% | 176% | 45 MB |
| KrakenD CE | 15,897 (15,867 to 16,125) | 8.05 ms | 7.66 ms | 12.47 ms | 16.62 ms | 100.00% | 176% | 45 MB |

inkgate against KrakenD CE: **1.08x**. Runs (req/s): inkgate [17245, 17223, 17293, 17238, 17349, 17219, 17242], KrakenD [15998, 15896, 15897, 15938, 15877, 15867, 16125].

### routes: 500 endpoints (250 static, 150 with one parameter, 100 with two), one backend each, requests spread over all of them

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 24,712 (24,474 to 24,923) | 5.18 ms | 4.63 ms | 9.19 ms | 13.49 ms | 100.00% | 176% | 48 MB |
| KrakenD CE | 26,897 (26,664 to 27,124) | 4.76 ms | 4.30 ms | 8.44 ms | 12.14 ms | 100.00% | 173% | 43 MB |

inkgate against KrakenD CE: **0.92x**. Runs (req/s): inkgate [24762, 24474, 24888, 24604, 24712, 24582, 24923], KrakenD [26664, 26962, 26794, 26858, 26897, 26984, 27124].

### plugin-modifier: one backend, a request modifier plugin adds a header and a response modifier plugin adds a field

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 30,151 (29,861 to 30,193) | 4.24 ms | 3.78 ms | 7.59 ms | 11.09 ms | 100.00% | 172% | 44 MB |
| KrakenD CE | 28,074 (27,766 to 28,324) | 4.56 ms | 4.14 ms | 8.02 ms | 11.49 ms | 100.00% | 172% | 42 MB |

inkgate against KrakenD CE: **1.07x**. Runs (req/s): inkgate [30179, 30031, 29861, 30193, 30156, 30101, 30151], KrakenD [28122, 28074, 27766, 27835, 28222, 28062, 28324].

### plugin-server: one backend, a server plugin wraps the HTTP handler and adds a response header

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 29,880 (29,832 to 30,079) | 4.28 ms | 3.81 ms | 7.67 ms | 11.15 ms | 100.00% | 172% | 44 MB |
| KrakenD CE | 27,117 (26,288 to 27,304) | 4.72 ms | 4.29 ms | 8.30 ms | 11.90 ms | 100.00% | 173% | 42 MB |

inkgate against KrakenD CE: **1.10x**. Runs (req/s): inkgate [29888, 29852, 29927, 29871, 29880, 30079, 29832], KrakenD [27117, 27177, 27263, 27030, 26288, 27032, 27304].

### plugin-client: one backend, a client plugin replaces the backend client and calls the API itself

| Gateway | Requests/s (range) | Avg | p50 | p90 | p99 | Success | CPU (of 100% per CPU) | Peak memory |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| inkgate | 21,025 (20,998 to 21,226) | 6.09 ms | 5.70 ms | 10.71 ms | 14.19 ms | 100.00% | 177% | 45 MB |
| KrakenD CE | 19,998 (19,943 to 20,165) | 6.40 ms | 6.07 ms | 11.05 ms | 14.95 ms | 100.00% | 177% | 42 MB |

inkgate against KrakenD CE: **1.05x**. Runs (req/s): inkgate [21226, 21006, 21078, 21007, 21025, 20998, 21186], KrakenD [20165, 19953, 20016, 19943, 19970, 19998, 20023].

## Plugin cost

Requests per second of each plugin scenario as a share of the same gateway on `json` (one backend, no plugin). The plugin source is the same for both gateways; it is built once with each toolchain.

| Scenario | inkgate | KrakenD CE |
|---|--:|--:|
| plugin-modifier | 99% | 102% |
| plugin-server | 98% | 99% |
| plugin-client | 69% | 73% |

![requests per second](rps.svg)

## Reproduce

```sh
cd benchmark
./run.sh                       # builds the inkgate image, runs everything, rewrites results/
DURATION=60s ROUNDS=5 ./run.sh  # longer, more rounds
```

Numbers depend on the machine; compare the two gateways within one run, not across machines.
