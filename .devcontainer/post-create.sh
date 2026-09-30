#!/usr/bin/env bash
set -euo pipefail

cd /workspaces/"$(basename "$PWD")" 2>/dev/null || true

if [[ ! -f go.mod ]]; then
  echo "Run post-create from the repository root inside the workspace." >&2
  exit 1
fi

go mod download
# Debian Bookworm marks the system interpreter as externally managed.
# The base image already installs the Python build toolchain via apt.
python3 -m build --version >/dev/null
python3 -m pip --version >/dev/null
command -v nfpm >/dev/null
command -v goreleaser >/dev/null

if ! grep -Fq 'export PATH="$HOME/go/bin:$PATH"' "$HOME/.bashrc"; then
  printf '\nexport PATH="$HOME/go/bin:$PATH"\n' >> "$HOME/.bashrc"
fi

cat <<'EOF'
Dev container ready.

Recommended checks:
  go test ./...
  go build -o ./gc ./cmd/gc
  ./gc version
  ./scripts/regression-core.sh

Packaging and release helpers:
  ./scripts/package.sh v0.3.10 release
  make release-local

Notes:
  - Real command validation still requires GC_TOKEN or GITCODE_TOKEN.
  - Host tokens are not auto-forwarded by the committed devcontainer config.
    Export them manually only after you trust the checked-out code.
  - The regression script must target infra-test/* repositories only.
EOF
