#!/usr/bin/env bash
# Version, build and (optionally) push the inkgate Docker image.
#
# Usage: scripts/docker-release.sh <command> [options]
#
# Commands:
#   version [bump]    print the version that would be released (nothing is changed)
#   build   [bump]    build the image locally with all its tags
#   push    [bump]    build, push every tag to the registry, and create the git tag
#
# bump (default: patch):
#   patch | minor | major   increase the latest git tag vX.Y.Z (0.0.0 if there is none)
#   X.Y.Z                   use exactly this version
#   beta                    X.Y.Z-beta.N, where X.Y.Z is VERSION in the Makefile and N is one more than the
#                           highest existing git tag vX.Y.Z-beta.N (1 for the first one)
#   X.Y.Z-beta.N | X.Y.Z-rc.N   exactly this pre-release
#   dev                     X.Y.Z-dev-<sha> from the current commit; never tagged in git, never "latest"
#
# Environment:
#   DOCKER_REPO   image name on Docker Hub        (default: 120200/inkgate)
#   PLATFORMS     comma list for buildx            (default: the local platform only)
#   GO_VERSION    Go version of the builder image  (default: the Dockerfile's)
#   NO_GIT_TAG=1  push without creating/pushing the git tag
#
# Tags pushed for a release 1.4.2: 1.4.2, 1.4, 1, latest.
# A pre-release 1.5.0-beta.2 gets 1.5.0-beta.2 and "beta" (or "rc"); never latest, 1.5 or 1.
# A dev build 1.4.3-dev-abc1234 gets only that tag.
set -euo pipefail

cd "$(dirname "$0")/.."

DOCKER_REPO="${DOCKER_REPO:-120200/inkgate}"
PLATFORMS="${PLATFORMS:-}"

die() { echo "error: $*" >&2; exit 1; }

# latest stable release tag, pre-releases (anything with a '-') are ignored
latest_version() {
  git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -v -- '-' | head -n1 | sed 's/^v//' || true
}

# VERSION from the Makefile: the version being developed
dev_version() {
  sed -n 's/^VERSION ?= *//p' Makefile | head -n1
}

next_beta() {
  local base last
  base="$(dev_version)"; [[ "$base" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "VERSION in the Makefile is not X.Y.Z"
  last="$(git tag --list "v${base}-beta.*" | sed "s/^v${base}-beta\.//" | sort -n | tail -n1)"
  echo "${base}-beta.$(( ${last:-0} + 1 ))"
}

next_version() {
  local bump="${1:-patch}" cur major minor patch
  cur="$(latest_version)"; cur="${cur:-0.0.0}"
  IFS=. read -r major minor patch <<<"$cur"

  case "$bump" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
    beta)  next_beta ;;
    [0-9]*.[0-9]*.[0-9]*-beta.[0-9]*|[0-9]*.[0-9]*.[0-9]*-rc.[0-9]*)
      [[ "$bump" =~ ^[0-9]+\.[0-9]+\.[0-9]+-(beta|rc)\.[0-9]+$ ]] || die "invalid pre-release: $bump"
      echo "$bump" ;;
    dev)   echo "${major}.${minor}.$((patch + 1))-dev-$(git rev-parse --short HEAD)" ;;
    [0-9]*.[0-9]*.[0-9]*)
      [[ "$bump" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "invalid version: $bump"
      echo "$bump" ;;
    *) die "unknown bump '$bump' (patch|minor|major|dev|X.Y.Z)" ;;
  esac
}

# prints the image tags (one per line) for a version
tags_for() {
  local v="$1" major minor
  if [[ "$v" == *-dev-* ]]; then echo "$v"; return; fi
  if [[ "$v" =~ -(beta|rc)\. ]]; then printf '%s\n' "$v" "${BASH_REMATCH[1]}"; return; fi
  IFS=. read -r major minor _ <<<"$v"
  printf '%s\n' "$v" "${major}.${minor}" "${major}" latest
}

cmd="${1:-}"; [[ -n "$cmd" ]] || { sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }
bump="${2:-patch}"
version="$(next_version "$bump")"

case "$cmd" in
  version)
    echo "$version"
    tags_for "$version" | sed "s#^#${DOCKER_REPO}:#" >&2
    exit 0 ;;
  build|push) ;;
  *) die "unknown command '$cmd' (version|build|push)" ;;
esac

if [[ "$cmd" == push && "$version" != *-dev-* ]]; then
  [[ -z "$(git status --porcelain)" ]] || die "working tree is not clean; commit first (the image must match a commit)"
  git rev-parse -q --verify "refs/tags/v${version}" >/dev/null && die "git tag v${version} already exists"
fi

revision="$(git rev-parse HEAD)"
created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

args=(--build-arg "VERSION=${version}" --build-arg "REVISION=${revision}" --build-arg "CREATED=${created}")
[[ -z "${GO_VERSION:-}" ]] || args+=(--build-arg "GO_VERSION=${GO_VERSION}")
while read -r t; do args+=(-t "${DOCKER_REPO}:${t}"); done < <(tags_for "$version")

echo ">> version ${version}"
tags_for "$version" | sed "s#^#   ${DOCKER_REPO}:#"

if [[ "$cmd" == build ]]; then
  docker build "${args[@]}" .
  exit 0
fi

# push: login is expected beforehand (docker login)
if [[ -n "$PLATFORMS" ]]; then
  docker buildx build --platform "$PLATFORMS" "${args[@]}" --push .
else
  docker build "${args[@]}" .
  while read -r t; do docker push "${DOCKER_REPO}:${t}"; done < <(tags_for "$version")
fi

if [[ "$version" != *-dev-* && "${NO_GIT_TAG:-}" != 1 ]]; then
  git tag -a "v${version}" -m "inkgate ${version}"
  git push origin "v${version}"
fi
echo ">> pushed ${DOCKER_REPO}:${version}"
