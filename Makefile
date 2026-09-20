DEBUG ?= false

COV="./coverage.txt"

PKG_VERSION ?= $(shell git describe --tags 2>/dev/null || git rev-parse --short HEAD)
DATE_FMT = +%Y-%m-%d
BUILD_DATE ?= $(shell date "$(DATE_FMT)")
GO_LDFLAGS := -X main.buildDate=$(BUILD_DATE) $(GO_LDFLAGS)
GO_LDFLAGS := $(GO_LDFLAGS) -X main.version=$(PKG_VERSION)

.PHONY: test
test: ## Run tests
	go test -coverprofile $(COV) ./...

.PHONY: coverage
coverage: ## Run coverage report
	go tool cover -func $(COV)

.PHONY: vet
vet: ## Run code issues detection
	go vet ./...

.PHONY: build
.DEFAULT_GOAL := build
build: ## Build the cmd applications
	go build -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" -o ./bin/a32001 ./cmd/a32001
	go build -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" -o ./bin/a32002 ./cmd/a32002
	go build -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" -o ./bin/a32003 ./cmd/a32003
	go build -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" -o ./bin/a32004 ./cmd/a32004

.PHONY: install
install: ## Install cmd applications in the equivalent to $GOPATH/bin
	GO111MODULE=on go install -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" ./cmd/a32001
	GO111MODULE=on go install -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" ./cmd/a32002
	GO111MODULE=on go install -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" ./cmd/a32003
	GO111MODULE=on go install -trimpath -ldflags "$(GO_LDFLAGS) -X main.debugMode=$(DEBUG)" ./cmd/a32004

.PHONY: clean
clean: ## Remove the bin folder and test files leaving the download data files intact
	rm -rf ./bin $(COV)

.PHONY: check
check: ## Validate URL links
	bash ./scripts/bash/b32001 ./configs/input/url.txt

.PHONY: download
download: check ## Downloads, renames the non-persistent ones, and moves all to $ENV_ALT_DATA_PATH
	bash ./scripts/bash/b32002 ./configs/input/url.txt

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
