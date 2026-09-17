# gitcode-cli Makefile

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Binary name
BINARY_NAME := gc
BINARY := bin/$(BINARY_NAME)
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

# Docker
DOCKER_IMAGE := gitcode/$(BINARY_NAME)
DOCKER_TAG ?= latest

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod

.PHONY: all build build-all clean test system-test system-test-write install fmt lint help
.PHONY: docker docker-build docker-push docker-run
.PHONY: release release-local release-snapshot
.PHONY: completions validate-ai-template validate-ai-record validate-ai-templates validate-sigs
.PHONY: classify-change-risk verify-remote-facts
.PHONY: deps update-deps dev-setup dev-doctor install uninstall

all: build

# Build for current platform
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY) ./cmd/gc

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/gc
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/gc

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/gc
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/gc

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/gc

run:
	$(GOCMD) run ./cmd/gc

test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

system-test:
	$(GOTEST) -tags=system ./tests/system

system-test-write:
	GC_SYSTEM_WRITE=1 $(GOTEST) -tags=system ./tests/system -run TestWriteScripts

test-coverage: test
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out coverage.html

install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BINARY) $(DESTDIR)$(BINDIR)/gc
	ln -sf gc $(DESTDIR)$(BINDIR)/gitcode

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/gc $(DESTDIR)$(BINDIR)/gitcode

fmt:
	$(GOCMD) fmt ./...

lint:
	golangci-lint run ./...

# Generate completions
completions:
	@mkdir -p completions
	@$(GOCMD) run ./cmd/gc completion bash > completions/$(BINARY_NAME).bash
	@$(GOCMD) run ./cmd/gc completion zsh > completions/$(BINARY_NAME).zsh
	@$(GOCMD) run ./cmd/gc completion fish > completions/$(BINARY_NAME).fish
	@echo "Completions generated in completions/"

# Docker targets
docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):$(VERSION) .

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):$(VERSION)

docker-run:
	docker run --rm -it -e GC_TOKEN $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-all:
	docker buildx build --platform linux/amd64,linux/arm64 -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Release targets
release:
	goreleaser release --clean

release-local:
	goreleaser release --snapshot --clean

release-snapshot:
	goreleaser release --snapshot --rm-dist

# Development
dev:
	$(GOCMD) run ./cmd/gc

# Check
check: test lint
	@echo "All checks passed!"

validate-ai-template:
	@test -n "$(FILE)" || (echo "Usage: make validate-ai-template FILE=docs/ai-templates/pr-self-check.md [KIND=pr-self-check]" && exit 2)
	@python3 scripts/validate-ai-record.py --mode template $(if $(KIND),--kind $(KIND),) "$(FILE)"

validate-ai-templates:
	@for file in docs/ai-templates/*.md; do \
		python3 scripts/validate-ai-record.py --mode template "$$file" || exit $$?; \
	done

validate-ai-record:
	@test -n "$(FILE)" || (echo "Usage: make validate-ai-record FILE=/path/to/file.md KIND=pr-self-check" && exit 2)
	@test -n "$(KIND)" || (echo "KIND is required" && exit 2)
	@python3 scripts/validate-ai-record.py --mode record --kind "$(KIND)" "$(FILE)"

validate-sigs:
	@bash scripts/validate-sig-yamls.sh

classify-change-risk:
	@test -n "$(BASE)" || (echo "Usage: make classify-change-risk BASE=origin/main" && exit 2)
	@python3 scripts/classify-change-risk.py --base "$(BASE)"

verify-remote-facts:
	@test -n "$(REPO)" || (echo "Usage: make verify-remote-facts REPO=owner/repo [ISSUE=1] [PR=2] [HEAD_SHA=<sha>]" && exit 2)
	@python3 scripts/verify-remote-facts.py --repo "$(REPO)" \
		$(if $(ISSUE),--issue $(ISSUE),) \
		$(if $(PR),--pr $(PR),) \
		$(if $(HEAD_SHA),--head-sha $(HEAD_SHA),)

# Dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Host development environment (container-free)
dev-setup:
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-setup.ps1
else
	@bash scripts/dev-setup.sh
endif

dev-doctor:
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-setup.ps1 -Check
else
	@bash scripts/dev-setup.sh --check
endif

# Update
update-deps:
	$(GOMOD) tidy
	$(GOGET) -u ./...

# Help
help:
	@echo "GitCode CLI Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make dev-setup      Install core host verification dependencies"
	@echo "  make dev-doctor     Check core host dependencies, install nothing"
	@echo "  make deps           Download and tidy Go module dependencies"
	@echo "  make build          Build the binary for current platform"
	@echo "  make build-all      Build binaries for all platforms"
	@echo "  make run            Run the application"
	@echo "  make test           Run tests"
	@echo "  make system-test    Run read-only real-command system tests"
	@echo "  make system-test-write Run opt-in write-path system tests"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make clean          Clean build artifacts"
	@echo "  make install        Install gc/gitcode to PREFIX/bin (default /usr/local)"
	@echo "  make uninstall      Remove gc/gitcode from PREFIX/bin"
	@echo "  make fmt            Format code"
	@echo "  make lint           Run linter"
	@echo "  make completions    Generate shell completions"
	@echo ""
	@echo "Docker:"
	@echo "  make docker         Build Docker image"
	@echo "  make docker-push    Push Docker image"
	@echo "  make docker-run     Run Docker container"
	@echo ""
	@echo "Release:"
	@echo "  make release        Create a release (requires tag)"
	@echo "  make release-local  Create a local release (snapshot)"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=$(VERSION)"
	@echo "  COMMIT=$(COMMIT)"
