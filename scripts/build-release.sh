#!/bin/bash
# Build release binaries for all platforms
# Usage: ./scripts/build-release.sh [version]

set -e

# Get version from argument or git tag
VERSION=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%d)

# darwin keeps its symbol table (-w only): -s drops LC_UUID, which dyld
# requires (same policy as ci.yml and the goreleaser overrides).
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"
LDFLAGS_DARWIN="-w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

echo "Building gc ${VERSION} (commit: ${COMMIT}, date: ${DATE})"

mkdir -p dist

# Build for Linux AMD64
echo "Building for linux/amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/gc_linux_amd64 ./cmd/gc

# Build for Linux ARM64
echo "Building for linux/arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o dist/gc_linux_arm64 ./cmd/gc

# Build for Darwin AMD64
echo "Building for darwin/amd64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS_DARWIN}" -o dist/gc_darwin_amd64 ./cmd/gc

# Build for Darwin ARM64
echo "Building for darwin/arm64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS_DARWIN}" -o dist/gc_darwin_arm64 ./cmd/gc

# Build for Windows AMD64
echo "Building for windows/amd64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o dist/gc_windows_amd64.exe ./cmd/gc

echo ""
echo "Build complete!"
echo "Binaries:"
ls -la dist/
