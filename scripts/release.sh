#!/usr/bin/env bash
# Releases master as the given version. Pushing the tag is the release: its Build, test and deploy run
# pushes dhis2/im-manager:<version>, and Deploy to im-vm then deploys that image to prod. The change log
# is generated after tagging so it carries the new version, and is pushed to master with the tag.

set -euo pipefail

fail() {
  echo "$1" >&2
  exit 1
}

git fetch --quiet --tags origin master

version="${1:-}"
latest=$(git tag --list '[0-9]*' --sort=-version:refname | head --lines=1)
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "usage: make release version=<major.minor.patch>, the latest release is $latest"

[ "$(git branch --show-current)" = master ] || fail "check out master first"
[ -z "$(git status --porcelain)" ] || fail "the working tree has uncommitted changes"
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/master)" ] || fail "master is not at origin/master, pull or push first"
! git rev-parse --quiet --verify "refs/tags/$version" > /dev/null || fail "$version already exists"

commit=$(git rev-parse HEAD)
conclusion=$(gh run list --workflow "Build, test and deploy" --branch master --commit "$commit" --limit 1 --json conclusion --jq '.[0].conclusion // empty')
[ "$conclusion" = success ] || fail "Build, test and deploy has not succeeded on ${commit:0:8} (${conclusion:-no finished run})"

echo "Releasing ${commit:0:8} as $version, after $latest:"
git log --oneline --first-parent "$latest..HEAD" 2> /dev/null || git log --oneline --first-parent -20
read -r -p "Tag $version and deploy it to prod? [y/N] " answer
[ "$answer" = y ] || fail "aborted, nothing was changed"

git tag "$version"
make change-log
git commit CHANGELOG.md --message "chore: generate change log for v$version"
git push --atomic origin master "$version"

echo "Pushed $version. Follow the build and the prod deploy at https://github.com/dhis2-sre/im-manager/actions"
