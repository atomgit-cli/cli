#!/usr/bin/env bash

log() {
  printf '\n[%s]\n' "$1"
}

fail() {
  printf 'SYSTEM-TEST: %s\n' "$*" >&2
  exit 1
}

skip() {
  printf 'SKIP: %s\n' "$*" >&2
}

require_infra_repo() {
  local name="$1"
  local repo="$2"

  if [[ -z "$repo" ]]; then
    fail "$name is required"
  fi
  if [[ "$repo" != infra-test/* ]]; then
    fail "$name must be infra-test/*, got '$repo'"
  fi
  if [[ "$repo" == "infra-test/" || "$repo" == */*/* ]]; then
    fail "$name must be an owner/repo path under infra-test, got '$repo'"
  fi
}

run_capture() {
  local __var_name="$1"
  shift

  local output
  if ! output="$("$@" 2>&1)"; then
    printf '%s\n' "$output" >&2
    return 1
  fi
  printf -v "$__var_name" '%s' "$output"
  if [[ "${SYSTEM_VERBOSE:-0}" == "1" ]]; then
    printf '%s\n' "$output"
  fi
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
    fail "expected exit status $expected_status, got $status: $*"
  fi
  printf -v "$__var_name" '%s' "$output"
  if [[ "${SYSTEM_VERBOSE:-0}" == "1" ]]; then
    printf '%s\n' "$output"
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "expected output to contain: $needle"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" == *"$needle"* ]]; then
    fail "expected output to NOT contain: $needle"
  fi
}

assert_json() {
  python3 -c 'import json, sys; json.load(sys.stdin)' >/dev/null
}

probe_or_skip() {
  local description="$1"
  shift

  if ! "$@" >/dev/null 2>&1; then
    skip "$description fixture unavailable"
    return 1
  fi
}
