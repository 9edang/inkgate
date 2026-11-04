VERSION ?= 0.1.0
LDFLAGS := -X github.com/9edang/inkgate/engine/core.InkgateVersion=$(VERSION)

.PHONY: build test check-layout
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
