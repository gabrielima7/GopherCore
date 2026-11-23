#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="$SCRIPT_DIR/next-version.sh"

assert_eq() {
  local expected="$1"
  local actual="$2"
  local desc="$3"
  if [ "$expected" != "$actual" ]; then
    echo "FAIL: $desc (expected: $expected, got: $actual)"
    exit 1
  fi
  echo "PASS: $desc"
}

# Test 1: explicit tag passed
assert_eq "v0.9.5" "$("$TARGET" tag v0.9.5 "v0.5.1")" "Tag passthrough"

# Test 2: baseline upgrade when latest tag is older than v0.6.0
assert_eq "v0.6.0" "$("$TARGET" branch main "v0.5.1")" "Baseline upgrade from v0.5.1"

# Test 3: increment patch < 10
assert_eq "v0.6.1" "$("$TARGET" branch main "v0.6.0")" "Increment from v0.6.0 to v0.6.1"
assert_eq "v0.6.10" "$("$TARGET" branch main "v0.6.9")" "Increment from v0.6.9 to v0.6.10"

# Test 4: rollover at .10 to next minor .0
assert_eq "v0.7.0" "$("$TARGET" branch main "v0.6.10")" "Rollover from v0.6.10 to v0.7.0"
assert_eq "v1.0.0" "$("$TARGET" branch main "v0.9.10")" "Rollover from v0.9.10 to v1.0.0"

echo "All version calculation tests passed!"
