.DEFAULT_GOAL := help

WAILS_TAGS            := webkit2_41
FRONTEND_DIR          := frontend
TOOLS_DIR             := $(CURDIR)/bin
GOLANGCI_LINT         := $(TOOLS_DIR)/golangci-lint
GOLANGCI_LINT_VERSION := v2.11.4
NFPM                  := $(TOOLS_DIR)/nfpm
NFPM_VERSION          := v2.41.3

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: install
install: install-tools ## Install backend, frontend and dev tool dependencies
	go mod download
	cd $(FRONTEND_DIR) && npm install

.PHONY: install-tools
install-tools: $(GOLANGCI_LINT) $(NFPM) ## Install pinned dev tools into ./bin

$(GOLANGCI_LINT):
	@mkdir -p $(TOOLS_DIR)
	GOBIN=$(TOOLS_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(NFPM):
	@mkdir -p $(TOOLS_DIR)
	GOBIN=$(TOOLS_DIR) go install github.com/goreleaser/nfpm/v2/cmd/nfpm@$(NFPM_VERSION)

.PHONY: dev
dev: ## Run the app in dev mode (hot reload)
	wails dev -tags $(WAILS_TAGS)

.PHONY: build
build: ## Build the production desktop binary
	wails build -tags $(WAILS_TAGS)

.PHONY: package-deb
package-deb: build $(NFPM) ## Build a .deb package for Ubuntu/Debian (output: build/bin/*.deb)
	$(NFPM) package --config build/linux/nfpm.yaml --packager deb --target build/bin/

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf build/bin $(FRONTEND_DIR)/dist/*
	touch $(FRONTEND_DIR)/dist/gitkeep

.PHONY: fmt
fmt: fmt-go fmt-front ## Format Go and frontend sources

.PHONY: fmt-go
fmt-go: ## gofmt -s -w .
	gofmt -s -w .

.PHONY: fmt-front
fmt-front: ## Prettier write
	cd $(FRONTEND_DIR) && npm run format

.PHONY: lint
lint: lint-go lint-front ## Run all linters

.PHONY: lint-go
lint-go: $(GOLANGCI_LINT) ## golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: lint-front
lint-front: ## eslint + svelte-check + prettier check
	cd $(FRONTEND_DIR) && npm run lint
	cd $(FRONTEND_DIR) && npm run check
	cd $(FRONTEND_DIR) && npm run format:check

.PHONY: test
test: test-go test-front ## Run all tests

.PHONY: test-go
test-go: ## go test -race -cover
	go test -race -cover $$(go list ./... | grep -v '/frontend/')

.PHONY: test-front
test-front: ## vitest
	cd $(FRONTEND_DIR) && npm run test

.PHONY: check
check: fmt lint test ## Full pipeline: fmt + lint + test (CLAUDE.md workflow)
