#!/usr/bin/env bash
# Test scripts/sync-docs-version.sh: version detection, boundary-aware
# replacement, per-file self-heal, the --check drift gate, and argument
# validation. The script resolves the repo root from its own location, so
# each case runs a throwaway copy inside a temp dir against fixture docs.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
sync_script="${script_dir}/sync-docs-version.sh"
failures=0
case_dir=""

fail() {
    echo "FAIL: $*" >&2
    failures=$((failures + 1))
}

cleanup() {
    if [[ -n "${case_dir}" ]]; then
        rm -rf "${case_dir}"
    fi
}
trap cleanup EXIT

write_doc() {
    local path="$1" version="$2"
    mkdir -p "$(dirname "${path}")"
    cat > "${path}" <<EOF
# Fixture doc
Download: https://gitcode.com/atomgit-cli/cli/releases/download/v${version}/gc_${version}_amd64.deb
Checksums: gc_${version}_checksums.txt
historical mention of v0.6.10 must not change
template: releases/download/v{VERSION}/{FILENAME}
EOF
}

new_case() {
    cleanup
    case_dir="$(mktemp -d)"
    mkdir -p "${case_dir}/scripts" "${case_dir}/docs"
    cp "${sync_script}" "${case_dir}/scripts/sync-docs-version.sh"
}

write_all_docs() {
    local version="$1"
    write_doc "${case_dir}/README.md" "${version}"
    write_doc "${case_dir}/docs/PACKAGING.md" "${version}"
    write_doc "${case_dir}/docs/AI-GUIDE.md" "${version}"
}

run_sync() {
    sync_output="$(bash "${case_dir}/scripts/sync-docs-version.sh" "$@" 2>&1)" && sync_status=0 || sync_status=$?
}

expect_status() {
    local want="$1" label="$2"
    if [[ ${sync_status} -ne ${want} ]]; then
        fail "${label}: exit ${sync_status}, want ${want} (output: ${sync_output})"
    fi
}

expect_output_contains() {
    local want="$1" label="$2"
    if ! grep -qF -- "${want}" <<<"${sync_output}"; then
        fail "${label}: output missing '${want}' (got: ${sync_output})"
    fi
}

expect_file_contains() {
    local path="$1" want="$2" label="$3"
    if ! grep -qF -- "${want}" "${path}"; then
        fail "${label}: ${path} missing '${want}'"
    fi
}

expect_file_not_contains() {
    local path="$1" want="$2" label="$3"
    if grep -qF -- "${want}" "${path}"; then
        fail "${label}: ${path} unexpectedly contains '${want}'"
    fi
}

# 1. Normal upgrade: v-prefixed URLs and bare filenames move, the longer
#    historical version survives, no prefix-corruption, template untouched.
new_case
write_all_docs "0.6.1"
run_sync v0.7.0
expect_status 0 "normal upgrade"
for f in README.md docs/PACKAGING.md docs/AI-GUIDE.md; do
    expect_file_contains "${case_dir}/${f}" "releases/download/v0.7.0/gc_0.7.0_amd64.deb" "normal upgrade"
    expect_file_contains "${case_dir}/${f}" "gc_0.7.0_checksums.txt" "normal upgrade"
    expect_file_contains "${case_dir}/${f}" "historical mention of v0.6.10 must not change" "normal upgrade"
    expect_file_not_contains "${case_dir}/${f}" "v0.7.00" "normal upgrade"
    expect_file_contains "${case_dir}/${f}" "template: releases/download/v{VERSION}/{FILENAME}" "normal upgrade"
done

# 2. Old is a prefix of new (0.6.1 -> 0.6.10): the two-pass placeholders
#    must not turn the new version into 0.6.100.
new_case
write_all_docs "0.6.1"
run_sync v0.6.10
expect_status 0 "prefix collision"
for f in README.md docs/PACKAGING.md docs/AI-GUIDE.md; do
    expect_file_contains "${case_dir}/${f}" "releases/download/v0.6.10/gc_0.6.10_amd64.deb" "prefix collision"
    expect_file_not_contains "${case_dir}/${f}" "v0.6.100" "prefix collision"
done

# 3. Partial drift self-heals: two docs already at 0.7.0, one lagging at
#    0.6.1 — a single run brings all three to the target.
new_case
write_doc "${case_dir}/README.md" "0.7.0"
write_doc "${case_dir}/docs/PACKAGING.md" "0.7.0"
write_doc "${case_dir}/docs/AI-GUIDE.md" "0.6.1"
run_sync v0.8.0
expect_status 0 "drift self-heal"
for f in README.md docs/PACKAGING.md docs/AI-GUIDE.md; do
    expect_file_contains "${case_dir}/${f}" "releases/download/v0.8.0/gc_0.8.0_amd64.deb" "drift self-heal"
done

# 4. No-op: everything already at the target version.
new_case
write_all_docs "0.7.0"
before="$(cat "${case_dir}/README.md")"
run_sync v0.7.0
expect_status 0 "no-op"
expect_output_contains "nothing to do" "no-op"
[[ "${before}" == "$(cat "${case_dir}/README.md")" ]] || fail "no-op: README.md changed"

# 5. Dry-run writes nothing.
new_case
write_all_docs "0.6.1"
before="$(cat "${case_dir}/docs/PACKAGING.md")"
run_sync v0.7.0 --dry-run
expect_status 0 "dry-run"
[[ "${before}" == "$(cat "${case_dir}/docs/PACKAGING.md")" ]] || fail "dry-run: docs/PACKAGING.md changed"

# 6. --check passes when every doc references the target.
new_case
write_all_docs "0.7.0"
run_sync v0.7.0 --check
expect_status 0 "check green"

# 7. --check fails on a lagging doc and names it plus the fix command.
new_case
write_doc "${case_dir}/README.md" "0.7.0"
write_doc "${case_dir}/docs/PACKAGING.md" "0.7.0"
write_doc "${case_dir}/docs/AI-GUIDE.md" "0.6.1"
run_sync v0.7.0 --check
expect_status 1 "check red"
expect_output_contains "docs/AI-GUIDE.md does not reference v0.7.0" "check red"
expect_output_contains "bash scripts/sync-docs-version.sh v0.7.0" "check red"

# 8. A doc without any releases/download/vX.Y.Z/ URL is a detection error.
new_case
write_all_docs "0.6.1"
printf '# no download URLs here\n' > "${case_dir}/docs/AI-GUIDE.md"
run_sync v0.7.0
expect_status 1 "detection failure"
expect_output_contains "Could not detect current doc version" "detection failure"

# 9. Argument validation: invalid version exits 1, usage errors exit 2.
new_case
write_all_docs "0.6.1"
run_sync v1.2
expect_status 1 "invalid version"
run_sync
expect_status 2 "missing version"
run_sync v0.7.0 --bogus
expect_status 2 "unknown option"
run_sync v0.7.0 --check --dry-run
expect_status 2 "check plus dry-run"

# 10. A missing doc file is an error.
new_case
write_all_docs "0.6.1"
rm "${case_dir}/docs/AI-GUIDE.md"
run_sync v0.7.0
expect_status 1 "missing file"
expect_output_contains "Missing file" "missing file"

if ((failures > 0)); then
    echo "sync-docs-version tests: ${failures} failure(s)" >&2
    exit 1
fi

echo "sync-docs-version tests passed"
