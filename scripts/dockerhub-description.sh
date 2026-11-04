#!/usr/bin/env bash
# Update the Docker Hub short description and overview from docker/.
#
#   DOCKERHUB_USERNAME=120200 DOCKERHUB_TOKEN=<access token> scripts/dockerhub-description.sh
#
# The token needs Read, Write and Delete permission (Account settings -> Personal access tokens);
# a Read & Write token can push images but not edit the description.
# Categories cannot be set here: choose them in the repository Settings (see docker/categories.txt).
set -euo pipefail
cd "$(dirname "$0")/.."

: "${DOCKERHUB_USERNAME:?set DOCKERHUB_USERNAME}"
: "${DOCKERHUB_TOKEN:?set DOCKERHUB_TOKEN}"
DOCKER_REPO="${DOCKER_REPO:-120200/inkgate}"

short="$(tr -d '\n' < docker/short-description.txt)"
(( ${#short} <= 100 )) || { echo "short description is ${#short} characters, the limit is 100" >&2; exit 1; }

jwt="$(jq -n --arg u "$DOCKERHUB_USERNAME" --arg p "$DOCKERHUB_TOKEN" '{username:$u,password:$p}' \
  | curl -fsS -H 'Content-Type: application/json' -d @- https://hub.docker.com/v2/users/login/ | jq -r .token)"

jq -n --arg d "$short" --rawfile f docker/README.md '{description:$d, full_description:$f}' \
  | curl -fsS -X PATCH -H "Authorization: JWT ${jwt}" -H 'Content-Type: application/json' -d @- \
      "https://hub.docker.com/v2/repositories/${DOCKER_REPO}/" >/dev/null

echo "updated https://hub.docker.com/r/${DOCKER_REPO}"
