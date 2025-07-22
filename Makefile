# ─────────────────────────────────────────────────────────────
# ghsearch – build / test / init automation
#
# Targets:
#   make init      – one-shot: go mod tidy, download deps, install dev tools
#   make build     – build binaries for linux/amd64 & darwin/arm64
#   make lint      – run vet + fmt + (optional) golangci-lint
#   make test      – run unit tests with the race detector
#   make clean     – remove bin/ artifacts
#
# Variables you might override on the CLI, eg:
#   make build VERSION=$(git rev-parse --short HEAD)
# ─────────────────────────────────────────────────────────────

########## configurable bits #################################
GO        ?= go
BIN_DIR   ?= bin
MAIN_PKG  ?= ./cmd/ghsearch
WEB_PKG   ?= ./cmd/ghweb
CGO       ?= 0                    # keep everything static
LDFLAGS   ?= -s -w
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null)
BUILD_DATE?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# dev-tool versions (HEAD is fine for most teams)
GOLANGCI_LINT_VER ?= latest
########## end configurable ##################################

# default target
.DEFAULT_GOAL := build

# one-shot: bootstrap the repo for GoLand / VS Code / whatever
init: tidy deps tools ## run go mod tidy, download deps, install toolchain

tidy: ## ensure go.mod matches sources
	$(GO) mod tidy

deps: ## download module cache (for offline builds)
	$(GO) mod download

tools: ## install developer CLI tools under GOPATH/bin
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VER)
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install honnef.co/go/tools/cmd/staticcheck@latest

########## CI-friendly helpers ################################
lint: ## go vet, fmt check, and optional golangci-lint if installed
	$(GO) vet ./...
	@diff=$$($(GO) fmt ./... | tee /dev/stderr); \
	 test -z "$$diff" || { echo "gofmt needed"; exit 1; }
	@command -v golangci-lint >/dev/null 2>&1 && \
		golangci-lint run ./... || true

test: ## unit tests with the race detector
	$(GO) test -race ./...

########## cross-compile ######################################
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/ghsearch-linux-amd64: | $(BIN_DIR)
	@echo "→ building $@"
	GOOS=linux  GOARCH=amd64 CGO_ENABLED=$(CGO) \
	$(GO) build -trimpath \
		-ldflags="$(LDFLAGS) -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)" \
		-o $@ $(MAIN_PKG)

$(BIN_DIR)/ghsearch-darwin-arm64: | $(BIN_DIR)
       @echo "→ building $@"
       GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO) \
       $(GO) build -trimpath \
               -ldflags="$(LDFLAGS) -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)" \
               -o $@ $(MAIN_PKG)

$(BIN_DIR)/ghweb-linux-amd64: | $(BIN_DIR)
	@echo "→ building $@"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO) \
	$(GO) build -trimpath \
	        -ldflags="$(LDFLAGS) -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)" \
	        -o $@ $(WEB_PKG)

$(BIN_DIR)/ghweb-darwin-arm64: | $(BIN_DIR)
	@echo "→ building $@"
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO) \
	$(GO) build -trimpath \
	-ldflags="$(LDFLAGS) -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)" \
	-o $@ $(WEB_PKG)

build: ## build both binaries
       $(MAKE) $(BIN_DIR)/ghsearch-linux-amd64
       $(MAKE) $(BIN_DIR)/ghsearch-darwin-arm64

build-web: $(BIN_DIR)/ghweb-linux-amd64 $(BIN_DIR)/ghweb-darwin-arm64 ## build web server

run-web: ## start the web server locally
	$(GO) run $(WEB_PKG)

clean: ## remove compiled binaries
	rm -rf $(BIN_DIR)

.PHONY: init tidy deps tools lint test build build-web run-web clean
