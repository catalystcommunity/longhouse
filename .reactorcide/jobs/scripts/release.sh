#!/bin/sh
set -e

SEMVER_TAGS_VERSION="v0.4.0"
GHCLI_VERSION="2.63.2"

# Script is invoked from the repo root (by the release job).
#
# Run-local parity: SKIP_GITHUB=true skips the version-bump push and the
# `gh release create` step; on-disk file edits + binary build still run, so
# the release flow is exercisable end-to-end against a working tree without
# publishing anything. The release job auto-enables this when invoked under
# `reactorcide run-local` (REACTORCIDE_WORKER_MODE=local).
if [ "${REACTORCIDE_WORKER_MODE:-remote}" = "local" ] && [ -z "${SKIP_GITHUB:-}" ]; then
  echo "=== run-local detected: SKIP_GITHUB=true ==="
  SKIP_GITHUB=true
fi

# -------------------------------------------------------------------
# 0. Configure git auth + land on main. semver-tags pushes the new
#    tag itself, so the credential needs to be on `origin` *before*
#    we call it. runnerlib checks the source out at a specific SHA
#    (detached HEAD), so we also fetch + checkout origin/main so the
#    eventual `git push` has an upstream branch to push to.
#    SKIP_GITHUB=true leaves both alone — run-local uses whatever the
#    working tree already has.
# -------------------------------------------------------------------
if [ "${SKIP_GITHUB:-false}" = "true" ]; then
  echo "=== SKIP_GITHUB=true: leaving git auth + branch state alone ==="
else
  git config user.name "Catalyst Community (automation)"
  git config user.email "automation@catalystcommunity.dev"
  git remote set-url origin "https://x-access-token:${GITHUB_PAT}@github.com/${REACTORCIDE_REPO}.git"
  echo "=== Aligning to origin/main ==="
  git fetch origin main
  git checkout -B main origin/main
fi

# -------------------------------------------------------------------
# 1. Install semver-tags
# -------------------------------------------------------------------
echo "=== Installing semver-tags ${SEMVER_TAGS_VERSION} ==="
curl -fsSL "https://github.com/catalystcommunity/semver-tags/releases/download/${SEMVER_TAGS_VERSION}/semver-tags.tar.gz" \
  -o /tmp/semver-tags.tar.gz
tar -xzf /tmp/semver-tags.tar.gz -C /tmp
chmod +x /tmp/semver-tags
export PATH="/tmp:$PATH"

# -------------------------------------------------------------------
# 2. Determine version bump from conventional commits
# -------------------------------------------------------------------
echo "=== Running semver-tags ==="
# Echo the full semver-tags output (including any push errors) before we
# try to parse the JSON tail — silent failures here have masked release
# breakage in the past.
semver-tags run --output_json > /tmp/semver-output.txt 2>&1 || true
cat /tmp/semver-output.txt
OUTPUT=$(tail -1 /tmp/semver-output.txt)

NEW_TAG=$(echo "${OUTPUT}" | grep -o '"New_release_git_tag":"[^"]*"' | cut -d'"' -f4)
PUBLISHED=$(echo "${OUTPUT}" | grep -o '"New_release_published":"[^"]*"' | cut -d'"' -f4)

if [ "${PUBLISHED}" != "true" ]; then
  echo "No new release needed."
  exit 0
fi

echo "=== New release: ${NEW_TAG} ==="
VERSION="${NEW_TAG#v}"

# -------------------------------------------------------------------
# 3. Update versioned files
# -------------------------------------------------------------------
echo "=== Updating versioned files to ${VERSION} ==="
sed -i "s/^version: .*/version: ${VERSION}/" helm_chart/Chart.yaml
sed -i "s/^appVersion: .*/appVersion: \"${VERSION}\"/" helm_chart/Chart.yaml
echo "${VERSION}" > version/VERSION.txt

# Commit the version bump (push requires GITHUB_PAT — CI only).
# Auth was already configured at the top of the script.
if [ "${SKIP_GITHUB:-false}" = "true" ]; then
  echo "=== SKIP_GITHUB=true: skipping version-bump commit and push ==="
else
  git add helm_chart/Chart.yaml version/VERSION.txt
  if git diff --cached --quiet; then
    echo "No version changes to commit"
  else
    git commit -m "ci: bump version to ${VERSION}"
    # Be explicit about the refspec: `git checkout -B main origin/main`
    # didn't set upstream tracking, so a bare `git push` fails with "no
    # upstream". Letting set -e propagate this is intentional — if the
    # bump can't land on main, the path-triggered build/deploy never
    # fires, and a half-baked release (tag without main bump) is worse
    # than a loud failure here.
    git push origin HEAD:main
  fi
fi

# -------------------------------------------------------------------
# 4. Build the release binaries
# -------------------------------------------------------------------
# Only the API ships as a binary. The web tier is the SolidJS SPA (webapp/),
# released as a container image by the web-build-and-deploy job, not a tarball.
echo "=== Building longhouse binaries ==="

cd api
go build -o /tmp/longhouse .
cd ..

RELEASE_DIR="/tmp/release"
mkdir -p "${RELEASE_DIR}"

tar -czf "${RELEASE_DIR}/longhouse-api-${VERSION}-linux-amd64.tar.gz" -C /tmp longhouse

# -------------------------------------------------------------------
# 5. Install gh CLI and create GitHub release
# -------------------------------------------------------------------
if [ "${SKIP_GITHUB:-false}" = "true" ]; then
  echo "=== SKIP_GITHUB=true: skipping GitHub release create ==="
  echo "=== Built artifacts left in ${RELEASE_DIR} for inspection ==="
else
  echo "=== Creating GitHub release ==="
  curl -fsSL "https://github.com/cli/cli/releases/download/v${GHCLI_VERSION}/gh_${GHCLI_VERSION}_linux_amd64.tar.gz" -o /tmp/gh.tar.gz
  tar -xzf /tmp/gh.tar.gz -C /tmp
  export PATH="/tmp/gh_${GHCLI_VERSION}_linux_amd64/bin:$PATH"

  GH_TOKEN="${GITHUB_PAT}" gh release create "${NEW_TAG}" \
    --repo "${REACTORCIDE_REPO}" \
    --title "${NEW_TAG}" \
    --generate-notes \
    ${RELEASE_DIR}/*

  echo "=== Released ${NEW_TAG} ==="
fi
