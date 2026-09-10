#!/usr/bin/env bash
#
# Rolls the deployment forward to one image tag, verifies it took over, and
# rolls back automatically if it did not.
#
# Runs on the server, from the deployment directory (a git clone that also
# holds the private .env). Called by .github/workflows/deploy.yml through the
# self-hosted runner, and safe to run by hand:
#
#   ./deploy/deploy-remote.sh <git-sha>          roll forward
#   ./deploy/deploy-remote.sh "$(cat .last-good-tag)"   roll back by hand
#
# The whole body lives in main() so bash parses the file before executing it.
# Without that, checking out a newer version of this very script mid-run would
# make bash read the rest from the changed file.

set -euo pipefail

readonly HEALTH_URL="http://127.0.0.1:8080/api/v1/healthz"
readonly HEALTH_TIMEOUT_SECONDS=120

log() {
    printf '[deploy %s] %s\n' "$(date -u +%H:%M:%S)" "$*"
}

fail() {
    printf '[deploy %s] ERROR: %s\n' "$(date -u +%H:%M:%S)" "$*" >&2
    exit 1
}

# running_commit asks the API container which build is serving traffic. Empty
# output means it is not answering yet.
running_commit() {
    docker compose exec -T api wget -q -O - "$HEALTH_URL" 2>/dev/null |
        sed -n 's/.*"commit":"\([^"]*\)".*/\1/p'
}

wait_for_commit() {
    local expected="$1" waited=0 actual=""

    while [ "$waited" -lt "$HEALTH_TIMEOUT_SECONDS" ]; do
        actual="$(running_commit || true)"
        if [ "$actual" = "$expected" ]; then
            return 0
        fi
        sleep 3
        waited=$((waited + 3))
    done

    log "health check timed out after ${HEALTH_TIMEOUT_SECONDS}s (reported commit: ${actual:-none})"
    return 1
}

roll_to() {
    local tag="$1"

    VHOME_IMAGE_TAG="$tag" docker compose pull api web
    # Migrations run before the new code, and only forward. They must be
    # backward compatible with the image currently running, which is what the
    # expand/contract rule in migrations/README.md buys us.
    VHOME_IMAGE_TAG="$tag" docker compose run --rm migrate
    VHOME_IMAGE_TAG="$tag" docker compose up -d --wait api web
}

main() {
    local tag="${1:-}"
    [ -n "$tag" ] || fail "usage: $0 <git-sha>"

    cd "$(dirname "$0")/.."
    local deploy_dir
    deploy_dir="$(pwd)"

    [ -f .env ] || fail "missing $deploy_dir/.env"
    [ ! -f compose.override.yaml ] || fail \
        "compose.override.yaml exists on the server; it would rebuild images locally. Delete it."

    log "deploying $tag to $deploy_dir"

    local previous=""
    if [ -f .last-good-tag ]; then
        previous="$(cat .last-good-tag)"
        log "current good tag: $previous"
    fi

    log "fetching sources"
    git fetch --quiet origin
    git checkout --quiet --force "$tag"

    log "rolling forward"
    if roll_to "$tag" && wait_for_commit "$tag"; then
        printf '%s\n' "$tag" > .last-good-tag
        log "deployed $tag successfully"
        docker image prune --force --filter "until=168h" >/dev/null 2>&1 || true
        docker compose ps
        return 0
    fi

    log "rollout failed"

    if [ -z "$previous" ]; then
        docker compose logs --tail 50 api >&2 || true
        fail "no previous good tag recorded; the stack needs manual attention"
    fi

    log "rolling back to $previous"
    git checkout --quiet --force "$previous"

    if roll_to "$previous" && wait_for_commit "$previous"; then
        docker compose logs --tail 50 api >&2 || true
        fail "deploy of $tag failed; rolled back to $previous"
    fi

    docker compose logs --tail 50 api >&2 || true
    fail "deploy of $tag failed AND rollback to $previous failed; manual recovery required"
}

main "$@"
