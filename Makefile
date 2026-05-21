SHELL := /bin/sh

APP_NAME ?= llm-monitor
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP_NAME)
CHART_DIR ?= charts/llm-monitor
GO_PACKAGES ?= ./...
WEB_DIR ?= web

DOCKER_COMPOSE ?= docker compose
HELM ?= helm
NPM ?= npm --prefix $(WEB_DIR)

GO_FILES := $(shell find . -path './.git' -prune -o -path './$(WEB_DIR)/node_modules' -prune -o -name '*.go' -print)
NODE_MODULES_STAMP := $(WEB_DIR)/node_modules/.package-lock.json

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: deps
deps: go-deps web-deps ## Install Go and frontend dependencies.

.PHONY: go-deps
go-deps: ## Download Go modules.
	go mod download

.PHONY: web-deps
web-deps: $(NODE_MODULES_STAMP) ## Install frontend dependencies from package-lock.json.

$(NODE_MODULES_STAMP): $(WEB_DIR)/package.json $(WEB_DIR)/package-lock.json
	$(NPM) ci

.PHONY: fmt
fmt: ## Format Go sources with gofmt.
	gofmt -w $(GO_FILES)

.PHONY: format
format: fmt ## Alias for fmt.

.PHONY: fmt-check
fmt-check: ## Check Go sources are gofmt-formatted.
	@files="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$files" ]; then \
		printf 'Go files need gofmt:\n%s\n' "$$files"; \
		exit 1; \
	fi

.PHONY: format-check
format-check: fmt-check ## Alias for fmt-check.

.PHONY: vet
vet: ## Run go vet.
	go vet $(GO_PACKAGES)

.PHONY: lint
lint: fmt-check vet compose-config ## Run non-mutating lint and config checks.

.PHONY: test-go
test-go: ## Run Go tests.
	go test $(GO_PACKAGES)

.PHONY: test
test: test-go ## Alias for test-go.

.PHONY: web-typecheck
web-typecheck: $(NODE_MODULES_STAMP) ## Run Vue TypeScript checks.
	cd $(WEB_DIR) && npm exec -- vue-tsc --noEmit

.PHONY: typecheck
typecheck: web-typecheck ## Alias for web-typecheck.

.PHONY: web-build
web-build: $(NODE_MODULES_STAMP) ## Build the Vue frontend.
	$(NPM) run build

.PHONY: build
build: web-build ## Build frontend assets and the Go server, worker, and scheduler binaries.
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN) ./cmd/server
	go build -trimpath -o $(BIN)-worker ./cmd/worker
	go build -trimpath -o $(BIN)-scheduler ./cmd/scheduler

.PHONY: run
run: ## Run the Go server locally.
	go run ./cmd/server

.PHONY: dev
dev: $(NODE_MODULES_STAMP) ## Start the Vite dev server.
	$(NPM) run dev

.PHONY: compose-config
compose-config: ## Validate docker-compose.yml.
	$(DOCKER_COMPOSE) config >/dev/null

.PHONY: docker-config
docker-config: compose-config ## Alias for compose-config.

.PHONY: docker-build
docker-build: ## Build the production Docker image locally.
	docker build -t $(APP_NAME):local .

.PHONY: chart-lint
chart-lint: ## Lint the Helm chart.
	$(HELM) lint $(CHART_DIR)

.PHONY: helm-lint
helm-lint: chart-lint ## Alias for chart-lint.

.PHONY: static-check
static-check: web-build ## Compare freshly built frontend assets with embedded static assets.
	diff -qr $(WEB_DIR)/dist cmd/server/static

.PHONY: check
check: lint test web-build ## Run the main pre-ship verification suite.
