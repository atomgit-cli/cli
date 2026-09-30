#!/bin/bash
# Package script for gitcode-cli
# Usage: ./scripts/package.sh [version] [target]
#
# This script builds binaries and creates DEB/RPM/PyPI packages.
# Run from project root directory.
#
# Targets:
#   all      - Build all packages (default)
#   deb      - Build DEB packages only
#   rpm      - Build RPM packages only
#   linux    - Build DEB + RPM packages
#   pypi     - Build PyPI package only
#   release  - Build DEB + RPM + PyPI (for release)

set -e
set -o pipefail

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Change to project root
cd "${PROJECT_ROOT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
verify_build() {
    local display_name="$1"
    local file_path="${display_name#"  "}"  # Remove leading "  " for display
    if [[ ! -f "$file_path" ]]; then
        error "Build failed: $file_path not found"
    fi
    success "$display_name"
}

# Usage
usage() {
    echo "Usage: $0 <version> [target]"
    echo ""
    echo "Targets:"
    echo "  all      - Build all packages (default)"
    echo "  deb      - Build DEB packages only"
    echo "  rpm      - Build RPM packages only"
    echo "  linux    - Build DEB + RPM packages"
    echo "  pypi     - Build PyPI package only"
    echo "  release  - Build DEB + RPM + PyPI (for release)"
    echo ""
    echo "Example:"
    echo "  $0 v0.2.12          # Build all packages for v0.2.12"
    echo "  $0 0.2.12 deb       # Build DEB packages only"
    exit 1
}

# Check arguments
if [ -z "$1" ]; then
    usage
fi

# Parse version (remove 'v' prefix if present)
VERSION=${1#v}
TARGET=${2:-all}

# Validate version format
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "Invalid version format: $VERSION (expected X.Y.Z)"
fi

# Validate target
case "$TARGET" in
    all|deb|rpm|linux|pypi|release) ;;
    *) error "Invalid target: $TARGET";;
esac

COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +%Y-%m-%d)
# darwin keeps its symbol table (-w only): -s drops LC_UUID, which dyld
# requires (same policy as ci.yml and the goreleaser overrides).
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"
LDFLAGS_DARWIN="-w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

echo ""
echo "============================================"
echo "  GitCode CLI Package Builder"
echo "============================================"
echo "  Version:  ${VERSION}"
echo "  Commit:   ${COMMIT}"
echo "  Date:     ${DATE}"
echo "  Target:   ${TARGET}"
echo "============================================"

# ============================================
# Step 1: Sync version across all files
# ============================================
echo ""
info "Step 1: Syncing version to all config files..."

bash scripts/sync-package-version.sh "${VERSION}"
success "Aligned VERSION and all package-manager metadata"

# Sync the three user-facing docs (README, AI-GUIDE, PACKAGING) through the
# dedicated script: it detects each file's current version independently,
# replaces with boundary awareness (no corruption of longer version strings),
# and refuses to exit 0 on residual old versions. The inline sed blocks it
# replaces were unbounded, rewrote historical version mentions, and had no
# residual check.
bash scripts/sync-docs-version.sh "v${VERSION}"
success "Updated README.md, docs/AI-GUIDE.md, docs/PACKAGING.md"

# ============================================
# Step 2: Create dist directory
# ============================================
mkdir -p dist

# ============================================
# Step 2.1: Prepare package-only assets
# ============================================
prepare_package_assets() {
    info "Preparing gitcode completion aliases..."
    bash scripts/prepare-package-assets.sh
    success "Prepared build/completions/gitcode.*"
    success "Prepared build/scripts/postinstall.sh"
}

# ============================================
# Step 3: Build Linux binaries (for DEB/RPM)
# ============================================
build_linux_binaries() {
    info "Building Linux amd64 binary..."
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o dist/gc_linux_amd64 ./cmd/gc
    verify_build "dist/gc_linux_amd64"

    info "Building Linux arm64 binary..."
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o dist/gc_linux_arm64 ./cmd/gc
    verify_build "dist/gc_linux_arm64"
}

# ============================================
# Step 4: Build DEB packages
# ============================================
build_deb() {
    info "Building DEB packages..."
    prepare_package_assets

    "$NFPMPATH" pkg -f nfpm-amd64.yaml -p deb -t "dist/gc_${VERSION}_amd64.deb" 2>&1 | sed 's/^/  /'
    verify_build "dist/gc_${VERSION}_amd64.deb"
    "$NFPMPATH" pkg -f nfpm-arm64.yaml -p deb -t "dist/gc_${VERSION}_arm64.deb" 2>&1 | sed 's/^/  /'
    verify_build "dist/gc_${VERSION}_arm64.deb"
}

# ============================================
# Step 5: Build RPM packages
# ============================================
build_rpm() {
    info "Building RPM packages..."
    prepare_package_assets

    "$NFPMPATH" pkg -f nfpm-amd64.yaml -p rpm -t "dist/gc-${VERSION}-1.x86_64.rpm" 2>&1 | sed 's/^/  /'
    verify_build "dist/gc-${VERSION}-1.x86_64.rpm"
    "$NFPMPATH" pkg -f nfpm-arm64.yaml -p rpm -t "dist/gc-${VERSION}-1.aarch64.rpm" 2>&1 | sed 's/^/  /'
    verify_build "dist/gc-${VERSION}-1.aarch64.rpm"
}

# ============================================
# Step 6: Build PyPI package
# ============================================
build_pypi() {
    if ! command -v python3 &> /dev/null; then
        warn "Python3 not found, skipping PyPI build"
        return 0
    fi

    info "Building PyPI package..."

    # Build multi-platform binaries for PyPI
    mkdir -p gc_cli/bin

    info "  Building binaries for PyPI..."

    # Linux amd64
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o gc_cli/bin/gc-linux-amd64 ./cmd/gc
    verify_build "  gc_cli/bin/gc-linux-amd64"

    # Linux arm64
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o gc_cli/bin/gc-linux-arm64 ./cmd/gc
    verify_build "  gc_cli/bin/gc-linux-arm64"

    # macOS amd64
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS_DARWIN}" -o gc_cli/bin/gc-darwin-amd64 ./cmd/gc
    verify_build "  gc_cli/bin/gc-darwin-amd64"

    # macOS arm64
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS_DARWIN}" -o gc_cli/bin/gc-darwin-arm64 ./cmd/gc
    verify_build "  gc_cli/bin/gc-darwin-arm64"

    # Windows amd64
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o gc_cli/bin/gc-windows-amd64.exe ./cmd/gc
    verify_build "  gc_cli/bin/gc-windows-amd64.exe"

    # Build wheel and sdist
    info "  Building wheel and sdist..."
    python3 -m build --wheel --sdist --outdir dist/ 2>&1 | sed 's/^/  /'

    verify_build "dist/gitcode_cli-${VERSION}-py3-none-any.whl"
    verify_build "dist/gitcode_cli-${VERSION}.tar.gz"
}

# ============================================
# Check nfpm availability
# ============================================
NFPMPATH=$(command -v nfpm 2>/dev/null || echo "$HOME/go/bin/nfpm")
if [ ! -x "$NFPMPATH" ]; then
    if [[ "$TARGET" =~ ^(all|deb|rpm|linux|release)$ ]]; then
        error "nfpm not found. Install it with:\n  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3"
    fi
fi

# ============================================
# Execute builds based on target
# ============================================
echo ""
info "Step 2: Building packages (target: ${TARGET})..."

case "$TARGET" in
    deb)
        build_linux_binaries
        build_deb
        ;;
    rpm)
        build_linux_binaries
        build_rpm
        ;;
    linux)
        build_linux_binaries
        build_deb
        build_rpm
        ;;
    pypi)
        build_pypi
        ;;
    all|release)
        build_linux_binaries
        build_deb
        build_rpm
        build_pypi
        ;;
esac

# ============================================
# Summary
# ============================================
echo ""
echo "============================================"
echo "  Package Summary"
echo "============================================"

# List all packages with sizes
if ls dist/gc*${VERSION}* 2>/dev/null | head -1 > /dev/null; then
    echo ""
    echo "Linux Packages:"
    ls -lh dist/gc*${VERSION}* 2>/dev/null | awk '{printf "  %-40s %s\n", $9, $5}'
fi

if ls dist/gitcode_cli*${VERSION}* 2>/dev/null | head -1 > /dev/null; then
    echo ""
    echo "PyPI Packages:"
    ls -lh dist/gitcode_cli* 2>/dev/null | grep -E "${VERSION}|whl|tar.gz" | awk '{printf "  %-40s %s\n", $9, $5}'
fi

echo ""
echo "============================================"
echo "  Next Steps"
echo "============================================"
echo ""
echo "1. Verify the local packages only (do not publish these artifacts):"
echo "   go build -o ./gc ./cmd/gc"
echo "   ./gc version"
echo ""
echo "2. Merge the release PR into main and run .github/workflows/release.yml."
echo "   The workflow builds and publishes the authoritative release artifacts."
echo ""
