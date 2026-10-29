#!/usr/bin/env bash
# Sync release version strings in README.md, docs/PACKAGING.md and
# docs/AI-GUIDE.md.
#
# These files are committed, user-facing docs whose download URLs and
# examples carry a pinned release version. Unlike the build-time version
# sync in scripts/package.sh and .github/workflows/release.yml (which cover
# nfpm-*.yaml / pyproject.toml / gc_cli/__init__.py), these docs are not
# synced automatically, so they lag behind every release (see #314).
#
# This script derives the target version from a single input, auto-detects
# the version each file currently references (per file, so a partially
# synced docs tree self-heals in a single run), and replaces it. Run it as
# a release step and commit the result.
#
# Usage:
#   ./scripts/sync-docs-version.sh <version> [--dry-run|--check]
#
#   <version>   Target release version, e.g. v0.7.0 or 0.7.0
#   --dry-run   Print the planned changes without writing files
#   --check     Verify every doc references the target version's download
#               URL; exit 1 with a fix hint otherwise. Writes nothing.
#
# Exit codes:
#   0  success (or every doc already at target version / check passed)
#   1  invalid input / detection failure / residual old version after
#      replace / --check failed
#   2  usage error

set -euo pipefail

dry_run=0
check_mode=0
target=""
for arg in "$@"; do
    case "${arg}" in
        --dry-run) dry_run=1 ;;
        --check) check_mode=1 ;;
        "") ;;
        -*) echo "Unknown option: ${arg}" >&2; exit 2 ;;
        *) target="${arg}" ;;
    esac
done

if [[ -z "${target}" ]]; then
    echo "Usage: $0 <version> [--dry-run|--check]" >&2
    echo "Example: $0 v0.7.0" >&2
    exit 2
fi

if [[ ${check_mode} -eq 1 && ${dry_run} -eq 1 ]]; then
    echo "--check cannot be combined with --dry-run" >&2
    exit 2
fi

if [[ ! "${target}" =~ ^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    echo "Invalid version: ${target}" >&2
    echo "Expected vMAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH (no prerelease)" >&2
    exit 1
fi

new_num="${target#v}"
new_tag="v${new_num}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
readme="${repo_root}/README.md"
packaging="${repo_root}/docs/PACKAGING.md"
ai_guide="${repo_root}/docs/AI-GUIDE.md"

for f in "${readme}" "${packaging}" "${ai_guide}"; do
    if [[ ! -f "${f}" ]]; then
        echo "Missing file: ${f}" >&2
        exit 1
    fi
done

# Check mode: every doc must reference the target version's download URL
# directory. Used by the release preflight and CI as a drift gate.
if [[ ${check_mode} -eq 1 ]]; then
    failed=0
    for f in "${readme}" "${packaging}" "${ai_guide}"; do
        if ! grep -qF "releases/download/v${new_num}/" "${f}"; then
            echo "ERROR: ${f} does not reference ${new_tag}" >&2
            echo "  expected a 'releases/download/v${new_num}/' download URL" >&2
            failed=1
        fi
    done
    if [[ ${failed} -ne 0 ]]; then
        echo "Fix: bash scripts/sync-docs-version.sh ${new_tag} && commit the updated docs" >&2
        exit 1
    fi
    echo "Docs version check passed: README.md, docs/PACKAGING.md, docs/AI-GUIDE.md all reference ${new_tag}"
    exit 0
fi

# Detect the version a doc currently references via its first
# releases/download/vX.Y.Z/ occurrence. Detection is per file: a partially
# synced docs tree (one file bumped, another lagging) still self-heals in a
# single run instead of no-op'ing on the first file's version.
detect_doc_version() {
    grep -oE 'releases/download/v[0-9]+\.[0-9]+\.[0-9]+/' "$1" | head -1 | sed -E 's#releases/download/v([0-9]+\.[0-9]+\.[0-9]+)/#\1#'
}

# Escape regex metachars (dots) for LHS; new is RHS literal.
new_re="${new_num//./\\.}"
# Boundary-aware on both sides: the version must not be preceded or
# followed by a digit, so 0.6.1 is not falsely matched inside 0.6.10,
# 10.6.1 or v10.6.1. The same boundary guards the replace below: an
# unbounded prefix replace would corrupt a longer version string
# (0.6.1 -> 0.7.0 would turn a v10.6.1 mention into v10.7.0) and the
# residual check would count the embedded old version inside the new
# one (0.6.1 -> 10.6.1 stays "residual" forever).
new_pat="(^|[^0-9])v?${new_re}([^0-9]|$)"
# Two-pass placeholder replace: move old (v-prefixed + bare) to distinct
# placeholders, then placeholders to new. Prevents corrupting the new
# version when old_num is a string prefix of new_num (e.g. 0.6.1 -> 0.6.10
# would otherwise turn v0.6.10 into v0.6.100).
#
# Known limitation of the consumed-boundary form: two occurrences of the
# same version separated by a single boundary character (e.g. "0.6.1
# 0.6.1") leave the second unmatched, because the first match consumes the
# separating character. The residual check below then fails loudly (exit 1)
# instead of silently missing it; docs never carry back-to-back same-
# version tokens in practice (URLs and filenames keep other characters
# between them).
ptag="__GCDOC_VER_TAG__"
pnum="__GCDOC_VER_NUM__"

changed=0
for f in "${readme}" "${packaging}" "${ai_guide}"; do
    old_num="$(detect_doc_version "${f}" || true)"
    if [[ -z "${old_num}" ]]; then
        echo "Could not detect current doc version in ${f}" >&2
        echo "Expected a 'releases/download/vX.Y.Z/' URL" >&2
        exit 1
    fi
    old_tag="v${old_num}"
    if [[ "${old_num}" == "${new_num}" ]]; then
        echo "  $(basename "${f}"): already references ${new_tag}"
        continue
    fi
    old_re="${old_num//./\\.}"
    old_pat="(^|[^0-9])v?${old_re}([^0-9]|$)"
    before=$(grep -c -E "${old_pat}" "${f}" || true)
    echo "  $(basename "${f}"): ${old_tag} -> ${new_tag} (${before} line(s))"
    if [[ ${dry_run} -eq 1 ]]; then
        echo "  [dry-run] would touch ${before} line(s) in $(basename "${f}")"
        continue
    fi
    sed -E -i "s/(^|[^0-9])v${old_re}([^0-9]|$)/\1${ptag}\2/g; s/(^|[^0-9])${old_re}([^0-9]|$)/\1${pnum}\2/g" "${f}"
    sed -i "s/${ptag}/v${new_num}/g; s/${pnum}/${new_num}/g" "${f}"
    after_old=$(grep -c -E "${old_pat}" "${f}" || true)
    after_new=$(grep -c -E "${new_pat}" "${f}" || true)
    echo "  $(basename "${f}"): ${before} -> ${after_new} line(s) with ${new_tag}; residual ${old_tag}: ${after_old}"
    if [[ ${after_old} -gt 0 ]]; then
        echo "ERROR: residual ${old_tag} in $(basename "${f}") after replace" >&2
        exit 1
    fi
    changed=1
done

if [[ ${dry_run} -eq 1 ]]; then
    echo "[dry-run] no files written."
elif [[ ${changed} -eq 0 ]]; then
    echo "Docs already reference ${new_tag}; nothing to do."
else
    echo "Done. Verify with: git diff --stat README.md docs/PACKAGING.md docs/AI-GUIDE.md"
fi
