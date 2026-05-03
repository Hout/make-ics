#!/usr/bin/env bash
# Create a versioned release: update CHANGELOG.md, commit, and tag.
#
# Usage: ./scripts/release.sh v1.2.3
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version>  (e.g. v1.0.0)" >&2
  exit 1
fi
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must be in semver form: v<major>.<minor>.<patch>" >&2
  exit 1
fi
if ! command -v git-cliff &>/dev/null; then
  echo "git-cliff not found — run 'nix shell nixpkgs#git-cliff' or enter the devShell" >&2
  exit 1
fi
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is dirty — commit or stash changes first" >&2
  exit 1
fi
if git rev-parse "$VERSION" &>/dev/null; then
  echo "Tag $VERSION already exists" >&2
  exit 1
fi

echo "Generating CHANGELOG.md for $VERSION..."
git cliff --tag "$VERSION" -o CHANGELOG.md

git add CHANGELOG.md
git commit -m "chore(release): $VERSION"
git tag -a "$VERSION" -m "$VERSION"

echo "Tagged $VERSION. Push with:"
echo "  git push && git push origin $VERSION"
