#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/gc-dev-setup-test.XXXXXX")"
trap 'rm -rf "$TMP_ROOT"' EXIT

STUB_BIN="$TMP_ROOT/stubs"
TOOLS_DIR="$TMP_ROOT/tools"
CALL_LOG="$TMP_ROOT/calls.log"
TOKEN_LEAK="$TMP_ROOT/token-leak"
CACHE_LOG="$TMP_ROOT/cache.log"
MANAGED_WRAPPER_MARKER="# Managed by gitcode-cli scripts/dev-setup.sh"
mkdir -p "$STUB_BIN" "$TMP_ROOT/outside-modcache/cache/download"

cat >"$STUB_BIN/tool-stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
name="$(basename "$0")"
if [[ -n "${GC_TOKEN:-}" || -n "${GITCODE_TOKEN:-}" ]]; then
    : >"${TOKEN_LEAK:?}"
    exit 90
fi
printf '%s %s\n' "$name" "$*" >>"${CALL_LOG:?}"

case "$name" in
    go)
        case "${1:-} ${2:-}" in
            "env GOVERSION") echo "${GO_STUB_VERSION:-go1.22.9}" ;;
            "version -m")
                module="$(sed -n 's/^# module=//p' "${3:-}" | head -n 1)"
                version="$(sed -n 's/^# version=//p' "${3:-}" | head -n 1)"
                [[ -n "$module" && -n "$version" ]] && printf 'mod\t%s\t%s\n' "$module" "$version"
                ;;
            "mod download") [[ -z "${MOD_DOWNLOAD_FAIL:-}" ]] || exit 1 ;;
            "list -mod=readonly")
                printf '%s|%s|%s|%s|%s|%s|%s|%s\n' \
                    "${GOCACHE:-}" "${GOTMPDIR:-}" "${HOME:-}" \
                    "${XDG_CONFIG_HOME:-}" "${GOENV:-}" "${GOPATH:-}" \
                    "${GOMODCACHE:-}" "${GOPROXY:-}" >>"${CACHE_LOG:?}"
                ;;
            "build -mod=readonly"|"build -o")
                printf '%s|%s|%s|%s|%s|%s|%s|%s\n' \
                    "${GOCACHE:-}" "${GOTMPDIR:-}" "${HOME:-}" \
                    "${XDG_CONFIG_HOME:-}" "${GOENV:-}" "${GOPATH:-}" \
                    "${GOMODCACHE:-}" "${GOPROXY:-}" >>"${CACHE_LOG:?}"
                out=""
                while (($#)); do
                    if [[ "$1" == "-o" ]]; then out="$2"; break; fi
                    shift
                done
                printf '#!/usr/bin/env bash\nexit 0\n' >"$out"
                chmod +x "$out"
                ;;
            "test -mod=readonly"|"test -race")
                printf '%s|%s|%s|%s|%s|%s|%s|%s\n' \
                    "${GOCACHE:-}" "${GOTMPDIR:-}" "${HOME:-}" \
                    "${XDG_CONFIG_HOME:-}" "${GOENV:-}" "${GOPATH:-}" \
                    "${GOMODCACHE:-}" "${GOPROXY:-}" >>"${CACHE_LOG:?}"
                ;;
            "install "*)
                spec="${2:-}"
                package="${spec%@*}"
                version="${spec##*@}"
                binary="${package##*/}"
                [[ "$package" == github.com/zricethezav/gitleaks/* ]] && binary="gitleaks"
                module="$package"
                [[ "$package" == github.com/golangci/golangci-lint/v2/* ]] && module="github.com/golangci/golangci-lint/v2"
                [[ "$package" == github.com/zricethezav/gitleaks/* ]] && module="github.com/zricethezav/gitleaks/v8"
                mkdir -p "${GOBIN:?}"
                printf '#!/usr/bin/env bash\n# module=%s\n# version=%s\nexit 0\n' "$module" "$version" >"$GOBIN/$binary"
                chmod +x "$GOBIN/$binary"
                ;;
            *) ;;
        esac
        ;;
    python3)
        if [[ "${1:-} ${2:-}" == "-m venv" ]]; then
            venv="$3"
            mkdir -p "$venv/bin"
            cat >"$venv/bin/python" <<'PYTHON'
#!/usr/bin/env bash
set -euo pipefail
if [[ -n "${GC_TOKEN:-}" || -n "${GITCODE_TOKEN:-}" ]]; then
    : >"${TOKEN_LEAK:?}"
    exit 90
fi
if [[ "${1:-} ${2:-}" == "-m pip" ]]; then
    [[ "${FAIL_PRECOMMIT_INSTALL:-0}" == 1 ]] && exit 42
    target="$(cd "$(dirname "$0")" && pwd)/pre-commit"
    printf '#!%s\n# pre-commit console script\n' "$0" >"$target"
    chmod +x "$target"
elif [[ "${1:-}" == */pre-commit ]]; then
    [[ "${PYTHONDONTWRITEBYTECODE:-}" == 1 ]] || exit 91
    printf 'pre-commit 4.6.1\n'
fi
PYTHON
            chmod +x "$venv/bin/python"
        fi
        ;;
    *) ;;
esac
STUB
chmod +x "$STUB_BIN/tool-stub"
for name in go git make python3 cc; do
    ln -s "$STUB_BIN/tool-stub" "$STUB_BIN/$name"
done

run_setup() {
    env \
        PATH="$STUB_BIN:/usr/bin:/bin" \
        GC_DEV_TOOLS_DIR="$TOOLS_DIR" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        GC_TOKEN="token-sentinel" GITCODE_TOKEN="token-sentinel-2" \
        HOME="$TMP_ROOT/outside-home" XDG_CONFIG_HOME="$TMP_ROOT/outside-config" \
        GOPATH="$TMP_ROOT/outside-gopath" GOMODCACHE="$TMP_ROOT/outside-modcache" \
        GOCACHE="$TMP_ROOT/outside-cache" GOTMPDIR="$TMP_ROOT/outside-tmp" \
        bash "$ROOT_DIR/scripts/dev-setup.sh" "$@"
}

set +e
first_output="$(run_setup 2>&1)"
first_status=$?
set -e
[[ $first_status -eq 0 ]]
[[ "$first_output" == *"dev environment ready."* ]]
[[ "$first_output" == *"WARNING: managed tools are not on PATH"* ]]
[[ "$first_output" == *"pre-commit / pre-push"* ]]
[[ "$first_output" == *"gitleaks: command not found"* ]]
[[ "$first_output" == *">> ~/.bashrc"* ]]
[[ ! -e "$TOKEN_LEAK" ]]
[[ -x "$TOOLS_DIR/bin/golangci-lint" ]]
[[ -x "$TOOLS_DIR/bin/gitleaks" ]]
[[ -L "$TOOLS_DIR/bin/pre-commit" ]]

: >"$CALL_LOG"
set +e
second_output="$(run_setup 2>&1)"
second_status=$?
set -e
[[ $second_status -eq 0 ]]
[[ "$second_output" != *"installing golangci-lint"* ]]
[[ "$second_output" != *"installing gitleaks"* ]]
[[ "$second_output" != *"installing pre-commit"* ]]
[[ ! -e "$TOKEN_LEAK" ]]

: >"$CACHE_LOG"
set +e
check_output="$(run_setup --check 2>&1)"
check_status=$?
set -e
[[ $check_status -eq 1 ]]
[[ "$check_output" == *"incomplete: 1 dependency gap(s)"* ]]
[[ "$check_output" == *"MISS managed tools on PATH"* ]]
[[ "$check_output" != *"dev environment ready."* ]]
[[ ! -e "$TOKEN_LEAK" ]]
[[ -s "$CACHE_LOG" ]]
! grep -F "$TMP_ROOT/outside-cache" "$CACHE_LOG"
! grep -F "$TMP_ROOT/outside-tmp" "$CACHE_LOG"
! grep -F "$TMP_ROOT/outside-home" "$CACHE_LOG"
! grep -F "$TMP_ROOT/outside-config" "$CACHE_LOG"
! grep -F "$TMP_ROOT/outside-gopath" "$CACHE_LOG"
! grep -F "$TMP_ROOT/outside-modcache|" "$CACHE_LOG"
grep -F "file://$TMP_ROOT/outside-modcache/cache/download" "$CACHE_LOG" >/dev/null

: >"$CALL_LOG"
set +e
modfail_output="$(MOD_DOWNLOAD_FAIL=1 run_setup 2>&1)"
modfail_status=$?
set -e
[[ $modfail_status -eq 1 ]]
[[ "$modfail_output" == *"MISS go mod download failed"* ]]
[[ "$modfail_output" == *"go env GOPROXY"* ]]
[[ "$modfail_output" == *"goproxy.cn,direct"* ]]
[[ "$modfail_output" == *"mirrors.aliyun.com/goproxy/"* ]]
[[ "$modfail_output" == *"examples only"* ]]

PERSISTED_GO_ENV="$TMP_ROOT/go-env"
PERSISTED_GOPATH="$TMP_ROOT/persisted go#path"
PERSISTED_MODCACHE="$TMP_ROOT/persisted mod#cache"
mkdir -p "$PERSISTED_MODCACHE/cache/download"
printf 'GOPATH=%s\nGOMODCACHE=%s\n' "$PERSISTED_GOPATH" "$PERSISTED_MODCACHE" >"$PERSISTED_GO_ENV"
persisted_go_env_before="$(cat "$PERSISTED_GO_ENV")"
: >"$CACHE_LOG"
set +e
persisted_output="$(
    env -u GOPATH -u GOMODCACHE \
        PATH="$TOOLS_DIR/bin:$STUB_BIN:/usr/bin:/bin" \
        GC_DEV_TOOLS_DIR="$TOOLS_DIR" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        HOME="$TMP_ROOT/outside-home" GOENV="$PERSISTED_GO_ENV" \
        bash "$ROOT_DIR/scripts/dev-setup.sh" --check 2>&1
)"
persisted_status=$?
set -e
[[ $persisted_status -eq 0 ]]
[[ "$persisted_output" == *"dev environment ready."* ]]
[[ "$persisted_output" != *"MISS managed tools on PATH"* ]]
grep -F "file://$TMP_ROOT/persisted%20mod%23cache/cache/download" "$CACHE_LOG" >/dev/null
[[ "$(cat "$PERSISTED_GO_ENV")" == "$persisted_go_env_before" ]]

MANAGED_HIJACK="$TMP_ROOT/managed-go-was-run"
cat >"$TOOLS_DIR/bin/go" <<EOF
#!/usr/bin/env bash
: >"$MANAGED_HIJACK"
exit 91
EOF
chmod +x "$TOOLS_DIR/bin/go"
MANAGED_BIN_ALIAS="$TMP_ROOT/managed-bin-alias"
ln -s "$TOOLS_DIR/bin" "$MANAGED_BIN_ALIAS"
set +e
hijack_output="$(
    env \
        PATH="$MANAGED_BIN_ALIAS:$STUB_BIN:/usr/bin:/bin" \
        GC_DEV_TOOLS_DIR="$TOOLS_DIR" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        HOME="$TMP_ROOT/outside-home" XDG_CONFIG_HOME="$TMP_ROOT/outside-config" \
        bash "$ROOT_DIR/scripts/dev-setup.sh" --check 2>&1
)"
hijack_status=$?
set -e
[[ $hijack_status -eq 0 ]]
[[ "$hijack_output" == *"dev environment ready."* ]]
[[ "$hijack_output" != *"MISS managed tools on PATH"* ]]
[[ ! -e "$MANAGED_HIJACK" ]]
rm "$TOOLS_DIR/bin/go" "$MANAGED_BIN_ALIAS"

NO_MAKE_BIN="$TMP_ROOT/no-make"
mkdir -p "$NO_MAKE_BIN"
for name in go git python3 cc; do
    ln -s "$STUB_BIN/tool-stub" "$NO_MAKE_BIN/$name"
done
for name in awk bash chmod env grep head ln mkdir mktemp mv readlink rm sed; do
    ln -s "$(command -v "$name")" "$NO_MAKE_BIN/$name"
done
FORGED_WRAPPER_MARKER="$TMP_ROOT/forged-wrapper-was-run"
cat >"$TOOLS_DIR/bin/make" <<EOF
$MANAGED_WRAPPER_MARKER
: >"$FORGED_WRAPPER_MARKER"
exit 0
EOF
chmod +x "$TOOLS_DIR/bin/make"
set +e
forged_output="$(
    env \
        PATH="$STUB_BIN:/usr/bin:/bin" GC_DEV_SYSTEM_PATH="$NO_MAKE_BIN" \
        GC_DEV_TOOLS_DIR="$TOOLS_DIR" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        HOME="$TMP_ROOT/outside-home" XDG_CONFIG_HOME="$TMP_ROOT/outside-config" \
        bash "$ROOT_DIR/scripts/dev-setup.sh" --check 2>&1
)"
forged_status=$?
set -e
[[ $forged_status -eq 1 ]]
[[ "$forged_output" == *"MISS make"* ]]
[[ ! -e "$FORGED_WRAPPER_MARKER" ]]
rm "$TOOLS_DIR/bin/make"

if [[ "$(/usr/bin/uname -s)" != "Darwin" &&
    ! -x /opt/homebrew/bin/brew &&
    ! -x /usr/local/bin/brew &&
    ! -x /home/linuxbrew/.linuxbrew/bin/brew ]]; then
    FAKE_BREW_MARKER="$TMP_ROOT/fake-brew-was-run"
    cat >"$NO_MAKE_BIN/brew" <<EOF
#!/usr/bin/env bash
: >"$FAKE_BREW_MARKER"
exit 0
EOF
    chmod +x "$NO_MAKE_BIN/brew"
    set +e
    brew_output="$(
        env \
            PATH="$NO_MAKE_BIN" GC_DEV_SYSTEM_PATH="$NO_MAKE_BIN" \
            GC_DEV_HOST_OS=Darwin GC_DEV_TOOLS_DIR="$TOOLS_DIR" \
            CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
            HOME="$TMP_ROOT/outside-home" XDG_CONFIG_HOME="$TMP_ROOT/outside-config" \
            bash "$ROOT_DIR/scripts/dev-setup.sh" 2>&1
    )"
    brew_status=$?
    set -e
    [[ $brew_status -eq 1 ]]
    [[ "$brew_output" == *"MISS Homebrew"* ]]
    [[ ! -e "$FAKE_BREW_MARKER" ]]
fi

set +e
old_go_output="$(
    GO_STUB_VERSION=go1.21.0 run_setup --check 2>&1
)"
old_go_status=$?
set -e
[[ $old_go_status -eq 1 ]]
[[ "$old_go_output" == *"older than 1.22"* ]]

UNMANAGED_PRE_TOOLS="$TMP_ROOT/unmanaged-pre-tools"
mkdir -p "$UNMANAGED_PRE_TOOLS/pre-commit-4.6.1"
printf 'keep\n' >"$UNMANAGED_PRE_TOOLS/pre-commit-4.6.1/sentinel"
set +e
unmanaged_pre_output="$(
    env \
        PATH="$STUB_BIN:/usr/bin:/bin" GC_DEV_TOOLS_DIR="$UNMANAGED_PRE_TOOLS" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        bash "$ROOT_DIR/scripts/dev-setup.sh" 2>&1
)"
unmanaged_pre_status=$?
set -e
[[ $unmanaged_pre_status -eq 1 ]]
[[ "$unmanaged_pre_output" == *"refusing to overwrite unmanaged pre-commit environment"* ]]
[[ "$(cat "$UNMANAGED_PRE_TOOLS/pre-commit-4.6.1/sentinel")" == "keep" ]]

FAILED_PRE_TOOLS="$TMP_ROOT/failed-pre-tools"
set +e
failed_pre_output="$(
    env \
        PATH="$STUB_BIN:/usr/bin:/bin" GC_DEV_TOOLS_DIR="$FAILED_PRE_TOOLS" \
        CALL_LOG="$CALL_LOG" TOKEN_LEAK="$TOKEN_LEAK" CACHE_LOG="$CACHE_LOG" \
        FAIL_PRECOMMIT_INSTALL=1 \
        bash "$ROOT_DIR/scripts/dev-setup.sh" 2>&1
)"
failed_pre_status=$?
set -e
[[ $failed_pre_status -eq 1 ]]
[[ "$failed_pre_output" == *"verified venv install failed"* ]]
[[ ! -e "$FAILED_PRE_TOOLS/pre-commit-4.6.1" ]]
[[ ! -e "$FAILED_PRE_TOOLS/bin/pre-commit" ]]

rm "$TOOLS_DIR/bin/gitleaks"
set +e
missing_output="$(run_setup --check 2>&1)"
missing_status=$?
set -e
[[ $missing_status -eq 1 ]]
[[ "$missing_output" == *"MISS gitleaks v8.30.1"* ]]

printf '#!/usr/bin/env bash\nexit 0\n' >"$TOOLS_DIR/bin/gitleaks"
chmod +x "$TOOLS_DIR/bin/gitleaks"
before_hash="$(cksum "$TOOLS_DIR/bin/gitleaks")"
set +e
unmanaged_output="$(run_setup 2>&1)"
unmanaged_status=$?
set -e
after_hash="$(cksum "$TOOLS_DIR/bin/gitleaks")"
[[ $unmanaged_status -eq 1 ]]
[[ "$unmanaged_output" == *"refusing to overwrite unmanaged tool"* ]]
[[ "$before_hash" == "$after_hash" ]]

printf 'Unix dev setup install, idempotency, isolation, and guard tests passed.\n'
