#!/usr/bin/env bash
# Install and verify the core host toolchain without a dev container.
#
# Usage:
#   bash scripts/dev-setup.sh          # install missing tools, then verify
#   bash scripts/dev-setup.sh --check  # offline verification; no persistent changes
#
# Packaging and release tools remain in .devcontainer/ and the documented
# release workflow. This script never needs GitCode credentials.

set -euo pipefail

unset GC_TOKEN GITCODE_TOKEN

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

GO_MIN_MAJOR=1
GO_MIN_MINOR=22
GOLANGCI_VERSION="v2.12.2"
GITLEAKS_VERSION="v8.30.1"
PRE_COMMIT_VERSION="4.6.1"
GOLANGCI_MODULE="github.com/golangci/golangci-lint/v2"
GOLANGCI_PACKAGE="$GOLANGCI_MODULE/cmd/golangci-lint"
GITLEAKS_MODULE="github.com/zricethezav/gitleaks/v8"
TOOLS_ROOT="${GC_DEV_TOOLS_DIR:-$HOME/.local/share/gitcode-cli/dev-tools}"
MANAGED_BIN="$TOOLS_ROOT/bin"
PRE_COMMIT_VENV="$TOOLS_ROOT/pre-commit-$PRE_COMMIT_VERSION"
MANAGED_WRAPPER_MARKER="# Managed by gitcode-cli scripts/dev-setup.sh"
RAW_SYSTEM_PATH="${GC_DEV_SYSTEM_PATH:-$PATH}"
SYSTEM_PATH=""
HOST_OS="${GC_DEV_HOST_OS:-$(/usr/bin/uname -s)}"
CHECK_ONLY=0
MISSING=()
SMOKE_DIR=""

case "${1:-}" in
    "") ;;
    --check) CHECK_ONLY=1 ;;
    -h|--help) sed -n '2,9p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
esac
[[ $# -le 1 ]] || { echo "too many arguments" >&2; exit 2; }

filter_system_path() {
    local raw="$1" entry filtered=""
    local old_ifs="$IFS"
    IFS=:
    for entry in $raw; do
        [[ "${entry%/}" == "${MANAGED_BIN%/}" ]] && continue
        if [[ -d "$entry" && -d "$MANAGED_BIN" && "$entry" -ef "$MANAGED_BIN" ]]; then
            continue
        fi
        filtered="${filtered:+$filtered:}$entry"
    done
    IFS="$old_ifs"
    printf '%s\n' "$filtered"
}

SYSTEM_PATH="$(filter_system_path "$RAW_SYSTEM_PATH")"
export PATH="$SYSTEM_PATH"

# managed_bin_on_path reports whether the managed bin directory is reachable
# through the original PATH (directly or via an equivalent directory entry).
managed_bin_on_path() {
    local raw="$RAW_SYSTEM_PATH" entry
    local old_ifs="$IFS"
    IFS=:
    for entry in $raw; do
        if [[ "${entry%/}" == "${MANAGED_BIN%/}" ]]; then
            IFS="$old_ifs"
            return 0
        fi
        if [[ -d "$entry" && -d "$MANAGED_BIN" && "$entry" -ef "$MANAGED_BIN" ]]; then
            IFS="$old_ifs"
            return 0
        fi
    done
    IFS="$old_ifs"
    return 1
}

cleanup_smoke_dir() {
    [[ -n "$SMOKE_DIR" ]] || return 0
    chmod -R u+w "$SMOKE_DIR" 2>/dev/null || true
    rm -rf "$SMOKE_DIR"
}

go_env_file() {
    if [[ "${GOENV:-}" == "off" ]]; then
        return
    fi
    if [[ -n "${GOENV:-}" ]]; then
        printf '%s\n' "$GOENV"
        return
    fi
    if [[ "$HOST_OS" == "Darwin" ]]; then
        printf '%s\n' "${HOME:-}/Library/Application Support/go/env"
        return
    fi
    printf '%s\n' "${XDG_CONFIG_HOME:-${HOME:-}/.config}/go/env"
}

persisted_go_env_value() {
    local key="$1" file line value=""
    file="$(go_env_file)"
    [[ -n "$file" && -f "$file" ]] || return 0
    while IFS= read -r line || [[ -n "$line" ]]; do
        [[ "$line" == "$key="* ]] && value="${line#*=}"
    done <"$file"
    printf '%s\n' "$value"
}

file_uri() {
    local path="$1" uri="" char escaped
    local LC_ALL=C
    local i
    for ((i = 0; i < ${#path}; i++)); do
        char="${path:i:1}"
        case "$char" in
            [a-zA-Z0-9._~/-]) uri+="$char" ;;
            *)
                printf -v escaped '%%%02X' "'$char"
                uri+="$escaped"
                ;;
        esac
    done
    printf 'file://%s\n' "$uri"
}

if (( CHECK_ONLY )); then
    SMOKE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gc-dev-setup.XXXXXX")"
    trap cleanup_smoke_dir EXIT
    original_home="${HOME:-}"
    persisted_gopath="$(persisted_go_env_value GOPATH)"
    persisted_modcache="$(persisted_go_env_value GOMODCACHE)"
    original_gopath="${GOPATH:-${persisted_gopath:-$original_home/go}}"
    original_modcache="${GOMODCACHE:-${persisted_modcache:-${original_gopath%%:*}/pkg/mod}}"
    export GOPATH="$SMOKE_DIR/gopath"
    export GOMODCACHE="$SMOKE_DIR/modcache"
    export GOPROXY="$(file_uri "$original_modcache/cache/download")"
    export GOENV=off
    export HOME="$SMOKE_DIR/home"
    export XDG_CONFIG_HOME="$SMOKE_DIR/config"
    export GOCACHE="$SMOKE_DIR/gocache"
    export GOTMPDIR="$SMOKE_DIR/gotmp"
    mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$GOCACHE" "$GOTMPDIR" "$GOPATH" "$GOMODCACHE"
fi

info() { printf '  %s\n' "$1"; }
ok() { printf 'ok   %s\n' "$1"; }
gap() { printf 'MISS %s\n' "$1"; MISSING+=("$1"); }
section() { printf '\n[%s]\n' "$1"; }

system_command() {
    PATH="$SYSTEM_PATH" command -v "$1" 2>/dev/null || true
}

have_system() {
    [[ -n "$(system_command "$1")" ]]
}

run_system() {
    local binary
    binary="$(system_command "$1")"
    [[ -n "$binary" ]] || return 127
    shift
    "$binary" "$@"
}

trusted_installer_command() {
    local name="$1" candidate
    case "$name" in
        brew)
            for candidate in /opt/homebrew/bin/brew /usr/local/bin/brew /home/linuxbrew/.linuxbrew/bin/brew; do
                [[ -x "$candidate" ]] && { printf '%s\n' "$candidate"; return 0; }
            done
            ;;
        sudo|apt-get|dnf|yum|pacman|zypper)
            for candidate in "/usr/bin/$name" "/usr/sbin/$name" "/bin/$name" "/sbin/$name"; do
                [[ -x "$candidate" ]] && { printf '%s\n' "$candidate"; return 0; }
            done
            ;;
    esac
    return 1
}

safe_managed_dir() {
    local path
    for path in "$TOOLS_ROOT" "$MANAGED_BIN"; do
        if [[ -L "$path" ]]; then
            gap "refusing managed symlink directory: $path"
            return 1
        fi
    done
    mkdir -p "$TOOLS_ROOT" "$MANAGED_BIN"
    SYSTEM_PATH="$(filter_system_path "$SYSTEM_PATH")"
    export PATH="$SYSTEM_PATH"
}

run_as_root() {
    local sudo_path
    if [[ "$(id -u)" -eq 0 ]]; then
        "$@"
        return
    fi
    sudo_path="$(trusted_installer_command sudo || true)"
    if [[ -n "$sudo_path" ]]; then
        "$sudo_path" -- "$@"
    else
        gap "sudo (required to install system packages)"
        return 1
    fi
}

go_version_ok() {
    local go_path raw major minor
    go_path="$(system_command go)"
    [[ -n "$go_path" ]] || return 1
    raw="$("$go_path" env GOVERSION 2>/dev/null || true)"
    raw="${raw#go}"
    major="${raw%%.*}"
    minor="${raw#*.}"; minor="${minor%%.*}"
    [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]] || return 1
    (( major > GO_MIN_MAJOR || (major == GO_MIN_MAJOR && minor >= GO_MIN_MINOR) ))
}

compiler_available() {
    have_system cc || have_system gcc || have_system clang
}

managed_wrapper_valid() {
    local path="$1" target="$2" expected line=""
    local -a lines=()
    [[ -f "$path" && ! -L "$path" ]] || return 1
    while IFS= read -r line || [[ -n "$line" ]]; do
        lines+=("$line")
    done <"$path"
    printf -v expected 'exec %q "$@"' "$target"
    [[ ${#lines[@]} -eq 2 &&
        "${lines[0]}" == "$MANAGED_WRAPPER_MARKER" &&
        "${lines[1]}" == "$expected" ]]
}

write_managed_wrapper() {
    local path="$1" target="$2" tmp
    if [[ -e "$path" || -L "$path" ]]; then
        managed_wrapper_valid "$path" "$target" || {
            gap "refusing to overwrite unmanaged wrapper: $path"
            return 1
        }
    fi
    safe_managed_dir || return 1
    tmp="$(mktemp "$MANAGED_BIN/.wrapper.XXXXXX")"
    {
        printf '%s\n' "$MANAGED_WRAPPER_MARKER"
        printf 'exec %q "$@"\n' "$target"
    } >"$tmp"
    chmod 0755 "$tmp"
    mv -f "$tmp" "$path"
}

install_system_dependencies() {
    (( CHECK_ONLY )) && return 0

    local need=0 tool package_manager package_manager_path brew_path
    for tool in go git make bash python3; do
        have_system "$tool" || need=1
    done
    compiler_available || need=1
    (( need )) || return 0

    section "System dependencies"
    case "$HOST_OS" in
        Darwin)
            brew_path="$(trusted_installer_command brew || true)"
            if [[ -z "$brew_path" ]]; then
                gap "Homebrew (install from https://brew.sh)"
                return
            fi
            info "installing Go, Git, GNU Make, Bash, and Python with Homebrew"
            "$brew_path" install go git make bash python
            if ! have_system make && have_system gmake; then
                write_managed_wrapper "$MANAGED_BIN/make" "$(system_command gmake)" || true
            fi
            compiler_available || gap "C compiler (run: xcode-select --install)"
            ;;
        *)
            for package_manager in apt-get dnf yum pacman zypper; do
                package_manager_path="$(trusted_installer_command "$package_manager" || true)"
                [[ -n "$package_manager_path" ]] && break
                package_manager=""
            done
            case "$package_manager" in
                apt-get)
                    run_as_root "$package_manager_path" update
                    run_as_root "$package_manager_path" install -y git make bash python3 python3-venv build-essential ca-certificates
                    ;;
                dnf)
                    run_as_root "$package_manager_path" install -y git make bash python3 gcc gcc-c++ ca-certificates
                    ;;
                yum)
                    run_as_root "$package_manager_path" install -y git make bash python3 gcc gcc-c++ ca-certificates
                    ;;
                pacman)
                    run_as_root "$package_manager_path" -S --needed --noconfirm git make bash python base-devel ca-certificates
                    ;;
                zypper)
                    run_as_root "$package_manager_path" --non-interactive install git make bash python3 gcc gcc-c++ ca-certificates
                    ;;
                *)
                    gap "supported package manager (apt, dnf, yum, pacman, zypper, or brew)"
                    ;;
            esac
            ;;
    esac
}

go_tool_version() {
    local binary="$1" module="$2" go_path
    go_path="$(system_command go)"
    [[ -n "$go_path" && -f "$binary" && ! -L "$binary" ]] || return 0
    "$go_path" version -m "$binary" 2>/dev/null |
        awk -v module="$module" '$1 == "mod" && $2 == module { print $3; exit }'
}

go_tool_candidate() {
    local name="$1" module="$2" candidate
    candidate="$MANAGED_BIN/$name"
    if [[ -e "$candidate" || -L "$candidate" ]]; then
        [[ "$(go_tool_version "$candidate" "$module")" != "" ]] && {
            printf '%s\n' "$candidate"
            return 0
        }
        return 0
    fi
    candidate="$(system_command "$name")"
    [[ -n "$candidate" && "$(go_tool_version "$candidate" "$module")" != "" ]] &&
        printf '%s\n' "$candidate"
    return 0
}

ensure_go_tool() {
    local name="$1" module="$2" package="$3" version="$4"
    local binary current target temp_dir temp_binary go_path
    target="$MANAGED_BIN/$name"
    binary="$(go_tool_candidate "$name" "$module")"
    current=""
    [[ -n "$binary" ]] && current="$(go_tool_version "$binary" "$module")"
    if [[ "$current" == "$version" ]]; then
        ok "$name $version ($binary)"
        return
    fi
    if (( CHECK_ONLY )); then
        gap "$name $version${current:+ (found $current)}"
        return
    fi
    if [[ -e "$target" || -L "$target" ]]; then
        [[ -f "$target" && ! -L "$target" && -n "$(go_tool_version "$target" "$module")" ]] || {
            gap "refusing to overwrite unmanaged tool: $target"
            return
        }
    fi
    safe_managed_dir || return
    temp_dir="$(mktemp -d "$TOOLS_ROOT/.install-$name.XXXXXX")"
    temp_binary="$temp_dir/$name"
    go_path="$(system_command go)"
    info "installing $name $version into $MANAGED_BIN"
    if GOBIN="$temp_dir" "$go_path" install "$package@$version" &&
        [[ "$(go_tool_version "$temp_binary" "$module")" == "$version" ]]; then
        chmod 0755 "$temp_binary"
        mv -f "$temp_binary" "$target"
        ok "$name $version ($target)"
    else
        gap "$name $version (verified go install failed)"
    fi
    rm -rf "$temp_dir"
}

system_pre_commit() {
    system_command pre-commit
}

managed_pre_commit() {
    local wrapper="$MANAGED_BIN/pre-commit" expected="$PRE_COMMIT_VENV/bin/pre-commit"
    [[ -L "$wrapper" && -x "$expected" ]] || return 0
    [[ "$(readlink "$wrapper" 2>/dev/null || true)" == "$expected" ]] || return 0
    printf '%s\n' "$expected"
    return 0
}

pre_commit_version() {
    local binary="$1"
    PYTHONDONTWRITEBYTECODE=1 "$binary" --version 2>/dev/null | awk '{print $2}'
}

ensure_pre_commit() {
    local python_path binary current wrapper expected temp_link
    python_path="$(system_command python3)"
    wrapper="$MANAGED_BIN/pre-commit"
    expected="$PRE_COMMIT_VENV/bin/pre-commit"
    binary="$(managed_pre_commit)"
    [[ -n "$binary" ]] || binary="$(system_pre_commit)"
    current=""
    [[ -n "$binary" ]] && current="$(pre_commit_version "$binary")"
    if [[ "$current" == "$PRE_COMMIT_VERSION" ]]; then
        ok "pre-commit $PRE_COMMIT_VERSION ($binary)"
        return
    fi
    if (( CHECK_ONLY )); then
        gap "pre-commit $PRE_COMMIT_VERSION${current:+ (found $current)}"
        return
    fi
    if [[ -e "$wrapper" || -L "$wrapper" ]]; then
        [[ -n "$(managed_pre_commit)" ]] || {
            gap "refusing to overwrite unmanaged wrapper: $wrapper"
            return
        }
    fi
    if [[ -e "$PRE_COMMIT_VENV" || -L "$PRE_COMMIT_VENV" ]]; then
        gap "refusing to overwrite unmanaged pre-commit environment: $PRE_COMMIT_VENV"
        return
    fi
    safe_managed_dir || return
    info "installing pre-commit $PRE_COMMIT_VERSION into $PRE_COMMIT_VENV"
    if "$python_path" -m venv "$PRE_COMMIT_VENV" &&
        "$PRE_COMMIT_VENV/bin/python" -m pip install --disable-pip-version-check "pre-commit==$PRE_COMMIT_VERSION" >/dev/null &&
        [[ "$(pre_commit_version "$expected")" == "$PRE_COMMIT_VERSION" ]]; then
        temp_link="$(mktemp "$MANAGED_BIN/.pre-commit-link.XXXXXX")"
        rm -f "$temp_link"
        ln -s "$expected" "$temp_link"
        [[ -L "$wrapper" ]] && rm -f "$wrapper"
        mv "$temp_link" "$wrapper"
        ok "pre-commit $PRE_COMMIT_VERSION ($expected)"
    else
        gap "pre-commit $PRE_COMMIT_VERSION (verified venv install failed)"
        rm -rf "$PRE_COMMIT_VENV"
    fi
}

install_system_dependencies

section "Go toolchain"
if ! have_system go; then
    gap "go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ (install from https://go.dev/dl/)"
elif ! go_version_ok; then
    if (( ! CHECK_ONLY )) && [[ "$HOST_OS" == "Darwin" ]]; then
        brew_path="$(trusted_installer_command brew || true)"
        [[ -n "$brew_path" ]] && "$brew_path" upgrade go || true
    fi
    go_version_ok ||
        gap "go $("$(system_command go)" env GOVERSION 2>/dev/null || true) is older than ${GO_MIN_MAJOR}.${GO_MIN_MINOR}; upgrade from https://go.dev/dl/"
else
    ok "go $("$(system_command go)" env GOVERSION) ($(system_command go))"
fi

GO_READY=0
if have_system go && go_version_ok; then
    GO_READY=1
    if (( ! CHECK_ONLY )); then
        SMOKE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gc-dev-setup.XXXXXX")"
        trap cleanup_smoke_dir EXIT
    fi
fi

if (( GO_READY )); then
    go_path="$(system_command go)"
    section "Module dependencies"
    if (( CHECK_ONLY )); then
        if "$go_path" list -mod=readonly -deps ./... >/dev/null 2>&1; then
            ok "module dependencies resolved"
        else
            gap "module dependencies (run without --check to download)"
        fi
    elif "$go_path" mod download; then
        ok "module dependencies resolved"
    else
        gap "go mod download failed"
        printf '  hint: check network reachability and go env GOPROXY.\n'
        printf '  If proxy.golang.org is unreachable, a public module mirror may help, for example:\n'
        printf '    go env -w GOPROXY=https://goproxy.cn,direct\n'
        printf '    go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct\n'
        printf '  (examples only; pick one per your own network environment.)\n'
    fi

    section "Verification tools"
    ensure_go_tool golangci-lint "$GOLANGCI_MODULE" "$GOLANGCI_PACKAGE" "$GOLANGCI_VERSION"
    ensure_go_tool gitleaks "$GITLEAKS_MODULE" "$GITLEAKS_MODULE" "$GITLEAKS_VERSION"
fi

section "System tools"
for tool in git bash python3; do
    have_system "$tool" && ok "$tool ($(system_command "$tool"))" || gap "$tool"
done
if have_system make; then
    ok "make ($(system_command make))"
elif have_system gmake &&
    managed_wrapper_valid "$MANAGED_BIN/make" "$(system_command gmake)" &&
    "$MANAGED_BIN/make" --version >/dev/null 2>&1; then
    ok "managed make wrapper ($MANAGED_BIN/make)"
else
    gap "make"
fi
compiler_available && ok "C compiler" || gap "C compiler (required by go test -race)"

if have_system python3; then
    ensure_pre_commit
else
    gap "pre-commit (Python 3 unavailable)"
fi

section "Verification"
if (( GO_READY )); then
    go_path="$(system_command go)"
    if (( CHECK_ONLY )); then
        build_cmd=("$go_path" build -mod=readonly -o "$SMOKE_DIR/gc" ./cmd/gc)
        race_cmd=("$go_path" test -mod=readonly -race ./pkg/config)
    else
        build_cmd=("$go_path" build -o "$SMOKE_DIR/gc" ./cmd/gc)
        race_cmd=("$go_path" test -race ./pkg/config)
    fi
    "${build_cmd[@]}" && "$SMOKE_DIR/gc" version >/dev/null &&
        ok "build and gc version" || gap "build or gc version"
    compiler_available && "${race_cmd[@]}" >/dev/null &&
        ok "race-enabled test" || gap "race-enabled test"
fi

cleanup_smoke_dir
SMOKE_DIR=""
trap - EXIT

# In --check mode an absent managed bin on PATH is a gap: the pre-commit and
# pre-push hooks resolve gitleaks through PATH and would fail on commit.
if (( CHECK_ONLY )) && ! managed_bin_on_path; then
    gap "managed tools on PATH (git hooks depend on it)"
fi

printf '\n'
if (( ${#MISSING[@]} )); then
    printf 'incomplete: %d dependency gap(s)\n' "${#MISSING[@]}"
    printf '  - %s\n' "${MISSING[@]}"
    printf '\nManaged tools directory: %s\n' "$MANAGED_BIN"
    printf 'After resolving gaps, add it to the current shell PATH:\n'
    printf '  export PATH=%q:"$PATH"\n' "$MANAGED_BIN"
    exit 1
fi

printf 'dev environment ready.\nManaged tools: %s\n' "$MANAGED_BIN"
if managed_bin_on_path; then
    printf 'No shell profile was changed.\n'
else
    printf 'WARNING: managed tools are not on PATH. Git hooks (pre-commit / pre-push)\n'
    printf 'will fail with "gitleaks: command not found" until it is on PATH.\n'
    printf 'Fix the current shell:\n'
    printf '  export PATH=%q:"$PATH"\n' "$MANAGED_BIN"
    printf 'Persist it (bash/zsh example; pick your own profile file):\n'
    printf "  echo 'export PATH=%q:\"\$PATH\"' >> ~/.bashrc\n" "$MANAGED_BIN"
    printf 'No shell profile was changed by this script.\n'
fi
