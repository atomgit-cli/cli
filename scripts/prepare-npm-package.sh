#!/usr/bin/env bash
# Assemble the npm packages exclusively from the verified GoReleaser binaries.
# Produces one tarball per supported npm coordinate: @gitcode-cli/cli,
# @atomgit-cli/cli, and the bare atomgit-cli name. The three coordinates are
# a long-term parallel distribution commitment (docs/PACKAGING.md, RFC-0001):
# identical content and version; only package.json name differs. Each staged
# copy runs the npm test suite under its own coordinate name, which exercises
# the dynamic package-name code paths (updater, wrapper) per coordinate.

set -euo pipefail

VERSION="${1:?usage: prepare-npm-package.sh VERSION DIST_DIR OUTPUT_DIR [ASSET_VERSION]}"
DIST_DIR="${2:?usage: prepare-npm-package.sh VERSION DIST_DIR OUTPUT_DIR [ASSET_VERSION]}"
OUTPUT_DIR="${3:?usage: prepare-npm-package.sh VERSION DIST_DIR OUTPUT_DIR [ASSET_VERSION]}"
ASSET_VERSION="${4:-${VERSION}}"
if [[ ! "${ASSET_VERSION}" =~ ^[0-9A-Za-z][0-9A-Za-z.+-]*$ ]]; then
    printf 'invalid GoReleaser asset version: %s\n' "${ASSET_VERSION}" >&2
    exit 2
fi

# Allowlist of npm coordinates this script may produce. Extending the list is
# a reviewed release-engineering decision (PACKAGING.md release invariant);
# a name outside this list can never be staged or packed.
readonly NPM_COORDINATES=("@gitcode-cli/cli" "@atomgit-cli/cli" "atomgit-cli")

PLATFORMS_DIR="npm/bin/platforms"
mkdir -p "${OUTPUT_DIR}"
OUTPUT_DIR="$(cd "${OUTPUT_DIR}" && pwd)"

extract_tar_binary() {
    local archive="$1" target="$2" temp_dir
    temp_dir="$(mktemp -d)"
    tar -xzf "${archive}" -C "${temp_dir}"
    install -m 0755 "${temp_dir}/gc" "${target}"
    rm -rf "${temp_dir}"
}

(
    cd npm
    npm version "${VERSION}" --no-git-tag-version --allow-same-version --ignore-scripts
)
mkdir -p "${PLATFORMS_DIR}"
install -m 0755 "${DIST_DIR}/gc_linux_amd64" "${PLATFORMS_DIR}/gc-linux-amd64"
install -m 0755 "${DIST_DIR}/gc_linux_arm64" "${PLATFORMS_DIR}/gc-linux-arm64"
extract_tar_binary "${DIST_DIR}/gc_${ASSET_VERSION}_darwin_amd64.tar.gz" "${PLATFORMS_DIR}/gc-darwin-amd64"
extract_tar_binary "${DIST_DIR}/gc_${ASSET_VERSION}_darwin_arm64.tar.gz" "${PLATFORMS_DIR}/gc-darwin-arm64"
unzip -p "${DIST_DIR}/gc_${ASSET_VERSION}_windows_amd64.zip" gc.exe > "${PLATFORMS_DIR}/gc-windows-amd64.exe"

stage_coordinate() {
    local coordinate="$1" stage
    case "${coordinate}" in
        "@gitcode-cli/cli"|"@atomgit-cli/cli"|"atomgit-cli") ;;
        *)
            printf 'refusing npm coordinate outside the reviewed allowlist: %s\n' "${coordinate}" >&2
            exit 2
            ;;
    esac
    stage="$(mktemp -d)"
    cp -a npm/. "${stage}/"
    node - "${stage}/package.json" "${coordinate}" <<'NODE'
const fs = require("fs");
const [file, name] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(file, "utf8"));
pkg.name = name;
fs.writeFileSync(file, JSON.stringify(pkg, null, 2) + "\n");
NODE
    (
        cd "${stage}"
        npm test
        npm pack --pack-destination "${OUTPUT_DIR}"
    )
    rm -rf "${stage}"
    printf 'assembled npm package %s@%s\n' "${coordinate}" "${VERSION}"
}

for coordinate in "${NPM_COORDINATES[@]}"; do
    stage_coordinate "${coordinate}"
done
