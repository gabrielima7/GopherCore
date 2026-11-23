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
  latest=$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname 2>/dev/null | head -n1 || true)
  if [ -z "$latest" ]; then
    latest="v0.5.1"
  fi
fi

# Strip leading 'v'
clean_ver="${latest#v}"
IFS='.' read -r raw_major raw_minor raw_patch <<< "$clean_ver"
raw_major=${raw_major:-0}
raw_minor=${raw_minor:-5}
raw_patch=${raw_patch:-1}

# Sanitize numbers to base 10 to avoid octal issues
major=$((10#$raw_major))
minor=$((10#$raw_minor))
patch=$((10#$raw_patch))

# Baseline check: start at v0.6.0 if current is below v0.6.0
if [ "$major" -eq 0 ] && [ "$minor" -lt 6 ]; then
  echo "v0.6.0"
  exit 0
fi

# Rollover logic: patch goes up to 10, then bumps minor. If minor also reaches 10 and major==0, bumps major.
if [ "$patch" -lt 10 ]; then
  patch=$((patch + 1))
else
  if [ "$minor" -ge 10 ] && [ "$major" -eq 0 ]; then
    major=$((major + 1))
    minor=0
    patch=0
  else
    minor=$((minor + 1))
    patch=0
  fi
fi

echo "v${major}.${minor}.${patch}"
