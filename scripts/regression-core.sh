#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GC_BIN="${GC_BIN:-$ROOT_DIR/gc}"
READONLY_REPO="${GC_REGRESSION_REPO:-infra-test/gctest1}"
RELEASE_TAG="${GC_REGRESSION_RELEASE_TAG:-v0.0.1-test}"
RUN_WRITE_PATHS="${GC_REGRESSION_WRITE:-0}"

# Guard: write paths must only target infra-test/* repositories.
WRITE_REPOS=()
if [[ "${RUN_WRITE_PATHS}" != "0" ]]; then
  WRITE_PR_REPO="${GC_REGRESSION_PR_REPO:-infra-test/gctest1}"
  WRITE_REPOS+=("$WRITE_PR_REPO")
fi
for repo in "${WRITE_REPOS[@]}"; do
  if [[ "$repo" != infra-test/* ]]; then
    echo "REGRESSION: write-path repository must be infra-test/*, got '$repo'" >&2
    echo "Set GC_REGRESSION_PR_REPO to an infra-test/* repository." >&2
    exit 1
  fi
done
if [[ "$READONLY_REPO" != infra-test/* ]]; then
  echo "REGRESSION: READONLY_REPO must be infra-test/*, got '$READONLY_REPO'" >&2
  exit 1
fi

log() {
  printf '\n[%s]\n' "$1"
}

# Build the CLI binary before fixture probing so a clean-worktree first run
# can probe fixtures with a valid binary (#478).
log "Build"
(cd "$ROOT_DIR" && go build -o "$GC_BIN" ./cmd/gc)
if [[ ! -x "$GC_BIN" ]]; then
  echo "REGRESSION: build failed, $GC_BIN not executable" >&2
  exit 1
fi

# Pre-flight fixture probe: verify required remote objects exist.
# Skip dependent tests if fixtures are missing (drift, not CLI regression).
# Probes run after Build so a clean-worktree first run can detect fixtures.
# Invariant: Build must precede probe (#478). If this guard fires on a clean
# worktree, someone moved Build after probe — first run would silently skip
# coverage again (#503).
if [[ ! -x "$GC_BIN" ]]; then
  echo "REGRESSION: $GC_BIN not built before fixture probe - Build must precede probe (#478/#503)" >&2
  exit 1
fi
SKIP_ISSUE=0; SKIP_PR=0; SKIP_RELEASE=0

if ! "$GC_BIN" issue list -R "$READONLY_REPO" --limit 1 --json >/dev/null 2>&1; then
  SKIP_ISSUE=1
  echo "REGRESSION: issue fixture unavailable for $READONLY_REPO, issue tests will skip" >&2
fi
if ! "$GC_BIN" pr list -R "$READONLY_REPO" --limit 1 --json >/dev/null 2>&1; then
  SKIP_PR=1
  echo "REGRESSION: PR fixture unavailable for $READONLY_REPO, PR tests will skip" >&2
fi
if ! "$GC_BIN" release list -R "$READONLY_REPO" --json >/dev/null 2>&1; then
  SKIP_RELEASE=1
  echo "REGRESSION: release fixture unavailable for $READONLY_REPO, release tests will skip" >&2
fi

run_capture() {
  local __var_name="$1"
  shift

  local output
  if ! output="$("$@" 2>&1)"; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  printf -v "$__var_name" '%s' "$output"
  printf '%s\n' "$output"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "expected output to contain: $needle" >&2
    exit 1
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" == *"$needle"* ]]; then
    echo "expected output to NOT contain: $needle" >&2
    exit 1
  fi
}

run_expect_fail() {
  local __var_name="$1"
  shift

  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e

  if [[ $status -eq 0 ]]; then
    echo "expected command to fail: $*" >&2
    exit 1
  fi
  printf -v "$__var_name" '%s' "$output"
  printf '%s\n' "$output"
}

run_expect_status() {
  local __var_name="$1"
  local expected_status="$2"
  shift 2

  local output
  set +e
  output="$("$@" 2>&1)"
  local status=$?
  set -e

  if [[ $status -ne $expected_status ]]; then
    printf '%s\n' "$output" >&2
    echo "expected exit status $expected_status, got $status: $*" >&2
    exit 1
  fi
  printf -v "$__var_name" '%s' "$output"
  printf '%s\n' "$output"
}

TMP_EMPTY_CONFIG_DIR="$(mktemp -d)"
TMP_NON_GIT_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_EMPTY_CONFIG_DIR" "$TMP_NON_GIT_DIR"
}
trap cleanup EXIT

log "Auth Status"
run_capture status_in_out "$GC_BIN" auth status
assert_contains "$status_in_out" "Logged in as"

log "Auth Token Non-Interactive Guard"
if token_out="$(GC_TOKEN="fake-token-for-non-interactive-guard" GITCODE_TOKEN= "$GC_BIN" auth token 2>&1)"; then
  echo "auth token unexpectedly succeeded in non-interactive mode" >&2
  exit 1
fi
assert_contains "$token_out" "interactive confirmation"

log "Auth Exit Code"
run_expect_status auth_exit_out 4 env GC_CONFIG_DIR="$TMP_EMPTY_CONFIG_DIR" GC_TOKEN= GITCODE_TOKEN= "$GC_BIN" pr review 1 -R "$READONLY_REPO" --approve
assert_contains "$auth_exit_out" "not authenticated"

# Env-token override guidance (issue 613): a failing token from GC_TOKEN must
# not suggest a plain re-login (the env var would keep overriding it) but the
# copy-paste remediation plans. Assertions are platform-neutral on purpose:
# the renderer emits sed/unset on Unix and PowerShell on Windows.
# The empty config dir and cleared GC_HOST pin the default-host resolution
# to gitcode.com regardless of the runner's real config, so the stub env
# token is always the one in effect (mirrors the Auth Exit Code isolation).
log "Auth Env Override Guidance"
run_expect_status env_guidance_out 4 env GC_CONFIG_DIR="$TMP_EMPTY_CONFIG_DIR" GC_HOST= GC_TOKEN="regression-stub-not-a-real-token" GITCODE_TOKEN= "$GC_BIN" auth status
assert_contains "$env_guidance_out" "仅重新登录不会生效"
assert_contains "$env_guidance_out" "gc auth login"
assert_contains "$env_guidance_out" "gc auth status"
assert_not_contains "$env_guidance_out" "To re-authenticate, run: gc auth login"

log "Repo View"
run_capture repo_view_out "$GC_BIN" repo view "$READONLY_REPO"
assert_contains "$repo_view_out" "$READONLY_REPO"

log "Repo View JSON"
run_capture repo_view_json "$GC_BIN" repo view "$READONLY_REPO" --json
assert_contains "$repo_view_json" "\"full_name\""

if [[ "$SKIP_ISSUE" -eq 0 ]]; then
log "Issue List"
run_capture issue_list_out "$GC_BIN" issue list -R "$READONLY_REPO" --limit 1
assert_contains "$issue_list_out" "#"
ISSUE_NUMBER="$(printf '%s\n' "$issue_list_out" | awk '/^#/ {gsub(/^#/, "", $1); print $1; exit}')"
if [[ -z "$ISSUE_NUMBER" ]]; then
  echo "failed to determine issue number from issue list output" >&2
  exit 1
fi

log "Issue View"
run_capture issue_view_out "$GC_BIN" issue view "$ISSUE_NUMBER" -R "$READONLY_REPO"
assert_contains "$issue_view_out" "#$ISSUE_NUMBER"

log "Issue List JSON"
run_capture issue_list_json "$GC_BIN" issue list -R "$READONLY_REPO" --limit 1 --json
assert_contains "$issue_list_json" "\"number\""

log "Issue View JSON"
run_capture issue_view_json "$GC_BIN" issue view "$ISSUE_NUMBER" -R "$READONLY_REPO" --json
assert_contains "$issue_view_json" "\"number\""
else
  echo "SKIP: issue fixture unavailable" >&2
fi

if [[ "$SKIP_PR" -eq 0 ]]; then
log "PR List JSON"
run_capture pr_list_json "$GC_BIN" pr list -R "$READONLY_REPO" --limit 1 --json
assert_contains "$pr_list_json" "["

log "PR List Paginate JSON"
run_capture pr_list_paginate_json "$GC_BIN" pr list -R "$READONLY_REPO" --paginate --per-page 1 --limit 1 --json
assert_contains "$pr_list_paginate_json" "["

log "Repo Log JSON"
run_capture repo_log_json "$GC_BIN" repo log -R "$READONLY_REPO" --limit 1 --json
assert_contains "$repo_log_json" "["

log "API Repo"
run_capture api_repo_json "$GC_BIN" api "repos/$READONLY_REPO"
assert_contains "$api_repo_json" "\"full_name\""

else
  echo "SKIP: PR fixture unavailable" >&2
fi

if [[ "$SKIP_RELEASE" -eq 0 ]]; then
log "Release List JSON"
run_capture release_list_json "$GC_BIN" release list -R "$READONLY_REPO" --json
assert_contains "$release_list_json" "\"tag_name\""
else
  echo "SKIP: release fixture unavailable" >&2
fi

log "Release Delete Dry Run"
run_capture release_delete_dry_run "$GC_BIN" release delete "$RELEASE_TAG" -R "$READONLY_REPO" --dry-run
assert_contains "$release_delete_dry_run" "Dry run: would delete release"

log "Non-Git Error Path"
run_expect_status nongit_out 2 bash -lc "cd '$TMP_NON_GIT_DIR' && '$GC_BIN' repo view"
assert_contains "$nongit_out" "not in a git repository"

log "Commit Not Found Exit Code"
run_expect_status commit_not_found_out 3 "$GC_BIN" commit view definitely-missing-sha-for-exit-code-probe -R "$READONLY_REPO"
assert_contains "$commit_not_found_out" "Not Found Commit"

if [[ "$RUN_WRITE_PATHS" == "1" ]]; then
  PR_REPO="${GC_REGRESSION_PR_REPO:-}"
  PR_HEAD="${GC_REGRESSION_PR_HEAD:-}"
  PR_BASE="${GC_REGRESSION_PR_BASE:-main}"
  PR_TITLE="${GC_REGRESSION_PR_TITLE:-Regression Probe}"
  PR_BODY="${GC_REGRESSION_PR_BODY:-Created by scripts/regression-core.sh}"

  if [[ -z "$PR_REPO" || -z "$PR_HEAD" ]]; then
    echo "GC_REGRESSION_PR_REPO and GC_REGRESSION_PR_HEAD are required when GC_REGRESSION_WRITE=1" >&2
    exit 1
  fi

  log "PR Create"
  run_capture pr_create_out "$GC_BIN" pr create -R "$PR_REPO" --head "$PR_HEAD" --base "$PR_BASE" --title "$PR_TITLE" --body "$PR_BODY"
  assert_contains "$pr_create_out" "Created PR #"
else
  log "PR Create"
  echo "skipped: set GC_REGRESSION_WRITE=1 plus GC_REGRESSION_PR_REPO and GC_REGRESSION_PR_HEAD to run write-path regression"
fi

log "Done"
echo "Core regression checks completed."
