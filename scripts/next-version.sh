#!/usr/bin/env bash
set -euo pipefail

ref_type="${1:-branch}"
ref_name="${2:-main}"
mock_latest="${3:-}"

# If triggered by a git tag, use the tag name directly
if [ "$ref_type" = "tag" ]; then
  echo "$ref_name"
  exit 0
fi

# Retrieve latest tag
if [ -n "$mock_latest" ]; then
  latest="$mock_latest"
else
  latest=$(git describe --tags --match "v*" --abbrev=0 2>/dev/null || echo "v0.5.1")
fi

# Strip leading 'v'
clean_ver="${latest#v}"
IFS='.' read -r major minor patch <<< "$clean_ver"
major=${major:-0}
minor=${minor:-5}
patch=${patch:-1}

# Baseline check: start at v0.6.0 if current is below v0.6.0
if [ "$major" -eq 0 ] && [ "$minor" -lt 6 ]; then
  echo "v0.6.0"
  exit 0
fi

# Rollover logic: patch goes up to 10, then bumps minor
if [ "$patch" -lt 10 ]; then
  patch=$((patch + 1))
else
  if [ "$minor" -eq 9 ] && [ "$major" -eq 0 ]; then
    major=1
    minor=0
    patch=0
  else
    minor=$((minor + 1))
    patch=0
  fi
fi

echo "v${major}.${minor}.${patch}"
