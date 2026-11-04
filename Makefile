VERSION ?= 0.1.0
LDFLAGS := -X github.com/9edang/inkgate/engine/core.InkgateVersion=$(VERSION)

.PHONY: build test check-layout tools lint sec vuln check-all docker-version docker-build docker-push

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

# integration tests (./tests) run the built binary
test: build
	go test ./...

# net/http routing code may only live in the mux router, the mux adapters (feature/**/mux),
# the HTTP stages (flow/entry, flow/request) and cmd; gin must not be imported at all
check-layout:
	@go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}}' ./... | awk '\
	  { for (i = 2; i <= NF; i++) if ($$i ~ /gin-gonic/) { print "gin imported: " $$1 " -> " $$i; bad = 1 } } \
	  $$1 !~ /\/mux(\/|$$)|^github\.com\/9edang\/inkgate\/flow\/(entry|request)$$|^github\.com\/9edang\/inkgate\/cmd\// { \
	    for (i = 2; i <= NF; i++) if ($$i ~ /gorilla\/mux|^github\.com\/9edang\/inkgate\/.*\/mux(\/|$$)|^github\.com\/9edang\/inkgate\/flow\/(entry|request)$$/) { print "router code imported by shared package: " $$1 " -> " $$i; bad = 1 } } \
	  END { exit bad }' && echo "layout OK"

# install the checkers into $(GOBIN) (once, or to upgrade)
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# static analysis; report in $(REPORT_DIR)/lint.txt
lint:
	@mkdir -p $(REPORT_DIR)
	$(SCAN_RUN) golangci-lint run --concurrency $(SCAN_CPUS) ./... 2>&1 | tee $(REPORT_DIR)/lint.txt

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
	for t in check-layout lint sec vuln; do \
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
