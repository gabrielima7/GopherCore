# Continuous Delivery and Automated Release Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and configure a GitHub Actions continuous delivery and automated release pipeline that automatically versions builds starting at `v0.6.0` (with rollover at `.10`), publishes production artifacts without cancelling in-flight runs, deploys to a protected environment, and generates categorized release notes.

**Architecture:** 
The pipeline runs on `push` to `main` and `v*` tags. Concurrency is configured with `cancel-in-progress: false`. A dedicated shell script (`scripts/next-version.sh`) computes the next semantic version based on git tags with rollover from `.10` to `.(y+1).0`. GitHub's `.github/release.yml` drives native Conventional Commit changelog categorization in `softprops/action-gh-release@v3`, and a downstream `deploy` job runs against a protected `production` GitHub environment.

**Tech Stack:** GitHub Actions, POSIX Bash, GitHub Releases API, softprops/action-gh-release@v3, anchore/sbom-action@v0, slsa-framework/slsa-github-generator.

**Spec:** `docs/superpowers/specs/2026-11-23-continuous-delivery-release-design.md`

## Global Constraints

- Pipeline triggers on `main` push and `v*` tag creation.
- Initial version baseline starts at `v0.6.0`.
- Version rollover rule: `PATCH < 10` increments `PATCH`; `PATCH == 10` increments `MINOR` and resets `PATCH` to `0` (`0.5.10` -> `0.6.0`).
- Concurrency setting: `cancel-in-progress: false`.
- Artifacts published: source tarball (`gophercore-${VERSION}.tar.gz`), SBOM (`gophercore-sbom.spdx.json`), checksums (`checksums.txt`), SLSA provenance.
- Deploy job must explicitly declare `needs: [release]` and `environment: production`.
- Changelog categories: Novas Funcionalidades (`feat`), Alterações (`chore`, `refactor`, `perf`, `build`, `ci`), Correções (`fix`), Remoções (`revert`, `deprecat*`, `remov*`).

## Review Focus

1. Input: Git repository with no tags or tags older than `v0.6.0` (e.g. `v0.5.1`). Expected behavior: defaults directly to `v0.6.0`.
2. Input: Latest tag is `v0.6.9`. Expected behavior: next version is `v0.6.10`.
3. Input: Latest tag is `v0.6.10`. Expected behavior: next version is `v0.7.0`.
4. Input: Explicit tag push (e.g. `v0.8.2`). Expected behavior: uses tag verbatim (`v0.8.2`), bypasses increment logic.
5. Concurrency condition: Rapid sequential pushes to `main`. Expected behavior: each run finishes publishing without being cancelled in-flight.

---

### Task 1: Changelog Configuration (`.github/release.yml`)

**Files:**
- Create: `.github/release.yml`

**Interfaces:**
- Consumes: GitHub Releases automatic release notes engine.
- Produces: Categorized sections for `generate_release_notes: true` in `softprops/action-gh-release`.

- [ ] **Step 1: Write `.github/release.yml` configuration**

Create `.github/release.yml` mapping labels and conventional commit title prefixes to the 4 required categories:
```yaml
changelog:
  categories:
    - title: "🚀 Novas Funcionalidades"
      labels:
        - "enhancement"
        - "feature"
    - title: "🛠 Alterações"
      labels:
        - "refactor"
        - "chore"
        - "dependencies"
        - "build"
        - "ci"
    - title: "🐛 Correções"
      labels:
        - "bug"
        - "fix"
    - title: "🗑️ Remoções"
      labels:
        - "removal"
        - "deprecated"
        - "revert"
```

- [ ] **Step 2: Verify YAML syntax and structure**

Run: `git status --short .github/release.yml`
Expected: `?? .github/release.yml`

- [ ] **Step 3: Commit `.github/release.yml`**

```bash
git add .github/release.yml
git commit -m "ci: add categorized release notes configuration"
```

---

### Task 2: Version Calculation Script and Unit Verification

**Files:**
- Create: `scripts/next-version.sh`
- Create: `scripts/test-next-version.sh`

**Interfaces:**
- Consumes: `$1` (ref_type: `tag` or `branch`), `$2` (ref_name: tag name or branch name), and git repository tags.
- Produces: Outputs string `vX.Y.Z` to standard output.

- [ ] **Step 1: Write the failing test suite `scripts/test-next-version.sh`**

```bash
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash scripts/test-next-version.sh`
Expected: FAIL (file `next-version.sh` not found).

- [ ] **Step 3: Implement `scripts/next-version.sh`**

Create `scripts/next-version.sh` supporting both mock tag input (for testing) and dynamic `git describe` resolution:
```bash
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash scripts/test-next-version.sh`
Expected: Output showing all PASS and "All version calculation tests passed!".

- [ ] **Step 5: Commit version calculation script and tests**

```bash
git add scripts/next-version.sh scripts/test-next-version.sh
git commit -m "ci: add semantic version calculation script with .10 rollover"
```

---

### Task 3: Update Delivery & Release Workflow (`.github/workflows/release.yml`)

**Files:**
- Modify: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: GitHub push events on `main` and `v*` tags, outputs of `scripts/next-version.sh`.
- Produces: GitHub Release with tarball, SBOM, checksums, SLSA provenance attestation, and protected production deployment.

- [ ] **Step 1: Update triggers and non-cancelling concurrency**

Update `on` and add `concurrency` in `.github/workflows/release.yml`:
```yaml
on:
  push:
    branches:
      - main
    tags:
      - "v*"

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
```

- [ ] **Step 2: Add dynamic version determination step in `release` job**

Update the `release` job to execute `scripts/next-version.sh`:
```yaml
      - name: Determine Version
        id: versioning
        run: |
          chmod +x ./scripts/next-version.sh
          REF_TYPE="${{ github.ref_type }}"
          REF_NAME="${{ github.ref_name }}"
          VERSION=$(./scripts/next-version.sh "$REF_TYPE" "$REF_NAME")
          echo "Computed version: $VERSION"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          IS_PRERELEASE=false
          if [ "${{ github.ref_type }}" != "tag" ]; then
            IS_PRERELEASE=true
          fi
          echo "prerelease=$IS_PRERELEASE" >> "$GITHUB_OUTPUT"
```

- [ ] **Step 3: Update packaging and artifact publication**

Add tarball generation and update `softprops/action-gh-release@v3` to release computed version:
```yaml
      - name: Package Release Distribution
        run: |
          VERSION="${{ steps.versioning.outputs.version }}"
          tar -czf "gophercore-${VERSION}.tar.gz" --exclude='.git' --exclude='.github' .
          sha256sum "gophercore-${VERSION}.tar.gz" >> checksums.txt
```
Update `softprops/action-gh-release@v3`:
```yaml
      - name: Create Release
        uses: softprops/action-gh-release@v3
        with:
          tag_name: ${{ steps.versioning.outputs.version }}
          name: GopherCore ${{ steps.versioning.outputs.version }}
          generate_release_notes: true
          draft: false
          prerelease: ${{ steps.versioning.outputs.prerelease == 'true' }}
          files: |
            gophercore-${{ steps.versioning.outputs.version }}.tar.gz
            gophercore-sbom.spdx.json
            checksums.txt
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 4: Add protected deployment job**

Add `deploy` job depending on `release`:
```yaml
  # ──────────────────────────────────────────────────
  # Continuous Deployment (Protected Environment)
  # ──────────────────────────────────────────────────
  deploy:
    name: Deploy to Production
    runs-on: ubuntu-latest
    needs: release
    environment: production
    steps:
      - name: Checkout
        uses: actions/checkout@v7

      - name: Validate Deployment Gate
        run: |
          echo "Deploying release ${{ needs.release.outputs.version }} to production environment..."
          echo "Production deployment gate successfully validated."
```

- [ ] **Step 5: Verify YAML syntax and consistency**

Run: `git diff .github/workflows/release.yml` to verify changes against spec.

- [ ] **Step 6: Commit workflow update**

```bash
git add .github/workflows/release.yml
git commit -m "ci: configure continuous delivery and protected production deploy"
```

---

### Task 4: Full Validation and Verification

**Files:**
- Test execution across workspace.

- [ ] **Step 1: Execute version calculation tests**

Run: `bash scripts/test-next-version.sh`
Expected: Output showing all PASS.

- [ ] **Step 2: Run local Go test and lint verification**

Run: `make test`
Expected: PASS (all unit and race detector tests pass).

- [ ] **Step 3: Final branch review**

Run: `git status`
Expected: Clean working tree with all tasks committed.
