VERSION ?= 0.1.0
LDFLAGS := -X github.com/9edang/inkgate/engine/core.InkgateVersion=$(VERSION)

.PHONY: build test test-plugins test-certs bench-ratelimit tools lint sec vuln check-all docker-version docker-build docker-push

GOBIN := $(shell go env GOPATH)/bin
export PATH := $(GOBIN):$(PATH)

# keep the exit code of a checker when its output goes through tee
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

# resource limits for lint, sec and vuln (they can use all RAM on a big module)
#   SCAN_CPUS      parallel jobs and Go threads (GOMAXPROCS)
#   SCAN_MEM_SOFT  Go heap target (GOMEMLIMIT); the GC works harder near it. It is a target, not a hard cap
# override per run: make lint SCAN_CPUS=1 SCAN_MEM_SOFT=512MiB
SCAN_CPUS ?= 2
SCAN_MEM_SOFT ?= 1GiB

# lowest CPU and disk priority, Go limited by env, and no cgo (the scanners then need no C compiler and parse less)
SCAN_RUN := CGO_ENABLED=0 GOMAXPROCS=$(SCAN_CPUS) GOMEMLIMIT=$(SCAN_MEM_SOFT) GOGC=50 nice -n 19 ionice -c3

# every check prints to the terminal and is also saved here
REPORT_DIR := bin/reports

GOLANGCI_LINT_VERSION ?= latest
GOSEC_VERSION ?= latest
GOVULNCHECK_VERSION ?= latest

build:
	go build -ldflags="$(LDFLAGS)" -o bin/inkgate ./cmd/inkgate

# the plugins the plugin loader tests open (sources in tests/plugins, built next to them)
test-plugins:
	CGO_ENABLED=1 go build -buildmode=plugin -o tests/plugins/client/client.so ./tests/plugins/client
	CGO_ENABLED=1 go build -buildmode=plugin -o tests/plugins/server/server.so ./tests/plugins/server
	CGO_ENABLED=1 go build -buildmode=plugin -o tests/plugins/proxy/error.so ./tests/plugins/proxy/error
	CGO_ENABLED=1 go build -buildmode=plugin -o tests/plugins/proxy/logger.so ./tests/plugins/proxy/logger

# the TLS certificate (cert.pem, key.pem) the jose tests serve JWKs with, made by the go:generate line in jwk_test.go
test-certs:
	cd feature/auth/jose && go generate ./...

# integration tests (./tests) run the built binary
test: build test-plugins test-certs
	go test ./...

# rate limiter benchmarks
bench-ratelimit:
	go test -bench=. -count 5 -benchmem -run='^#' ./feature/traffic/ratelimit/

# install the checkers into $(GOBIN), only those that are not there yet; `make tools FORCE=1` reinstalls
tools:
	@test -x $(GOBIN)/golangci-lint -a -z "$(FORCE)" || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@test -x $(GOBIN)/gosec -a -z "$(FORCE)" || go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	@test -x $(GOBIN)/govulncheck -a -z "$(FORCE)" || go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# static analysis; report in $(REPORT_DIR)/lint.txt
lint:
	@mkdir -p $(REPORT_DIR)
	$(SCAN_RUN) golangci-lint run --concurrency $(SCAN_CPUS) $(if $(LINT_ALL),--new=false) ./... 2>&1 | tee $(REPORT_DIR)/lint.txt

# security issues in our own code; report in $(REPORT_DIR)/gosec.txt
sec:
	@mkdir -p $(REPORT_DIR)
	$(SCAN_RUN) gosec -quiet ./... 2>&1 | tee $(REPORT_DIR)/gosec.txt

# known CVEs in the dependencies and the standard library that the code really calls;
# report in $(REPORT_DIR)/vuln.txt
vuln:
	@mkdir -p $(REPORT_DIR)
	$(SCAN_RUN) govulncheck ./... 2>&1 | tee $(REPORT_DIR)/vuln.txt

# every check in one go; keeps going after a failure and reports at the end
check-all:
	@fail=0; \
	for t in lint sec vuln; do \
	  echo "== $$t"; $(MAKE) --no-print-directory $$t || fail=1; \
	done; \
	test $$fail -eq 0 && echo "all checks OK"

# Docker image; DOCKER_REPO and BUMP (patch|minor|major|beta|dev|X.Y.Z[-beta.N]) can be set on the command line
BUMP ?= beta

docker-version:
	@scripts/docker-release.sh version $(BUMP)

docker-build:
	scripts/docker-release.sh build $(BUMP)

# builds, pushes to Docker Hub (run `docker login` first) and creates the git tag
docker-push:
	scripts/docker-release.sh push $(BUMP)
