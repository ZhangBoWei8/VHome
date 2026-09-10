#!/bin/sh
#
# First-time setup for a VHome server, and the manual fallback when you want to
# bring the stack up without going through GitHub Actions.
#
# Routine deploys do NOT use this script: they go through
# deploy/deploy-remote.sh, which pins an exact image tag, runs migrations and
# rolls back on failure. This one only prepares the host and starts whatever
# tag you point it at.
#
#   ./deploy.sh              start the :latest images
#   VHOME_IMAGE_TAG=<sha> ./deploy.sh   start a specific build

set -eu

cd "$(dirname "$0")"

if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate deployment secrets" >&2
    exit 1
fi

if [ -f compose.override.yaml ]; then
    echo "compose.override.yaml is present: this machine would build images locally." >&2
    echo "That file is for development only. Delete it on a deployment server." >&2
    exit 1
fi

if [ ! -f .env ]; then
    umask 077
    db_password="$(openssl rand -hex 24)"
    root_password="$(openssl rand -hex 24)"
    secret_encryption_key="$(openssl rand -base64 32)"

    {
        printf '%s\n' '# Image source. VHOME_IMAGE_REGISTRY must match the registry CI pushes to.'
        printf '%s\n' 'VHOME_IMAGE_REGISTRY=ghcr.io/zhangbowei8'
        printf '%s\n' 'VHOME_IMAGE_TAG=latest'
        printf '%s\n' ''
        printf '%s\n' 'VHOME_WEB_PORT=5173'
        printf '%s\n' 'VHOME_MYSQL_HOST_PORT=3307'
        printf '%s\n' 'VHOME_MYSQL_DATABASE=vhome'
        printf '%s\n' 'VHOME_MYSQL_USER=vhome'
        printf 'VHOME_MYSQL_PASSWORD=%s\n' "$db_password"
        printf 'VHOME_MYSQL_ROOT_PASSWORD=%s\n' "$root_password"
        printf 'VHOME_SECRET_ENCRYPTION_KEY=%s\n' "$secret_encryption_key"
        printf '%s\n' 'VHOME_ENV=development'
        printf '%s\n' 'VHOME_AUTH_COOKIE_SECURE=false'
        printf '%s\n' 'DEEPSEEK_APIKEY='
        printf '%s\n' 'DEEPSEEK_MODEL=deepseek-chat'
        printf '%s\n' 'DEEPSEEK_BASEURL=https://api.deepseek.com/v1'
        printf '%s\n' 'GLM_APIKEY='
        printf '%s\n' 'GLM_MODEL='
        printf '%s\n' 'GLM_BASEURL='
    } > .env

    echo "Generated private deployment configuration: .env"
    echo "Fill in DEEPSEEK_APIKEY and check VHOME_IMAGE_REGISTRY, then run this script again."
    exit 0
fi

echo "Starting MySQL..."
docker compose up -d --wait mysql

echo "Applying migrations..."
docker compose run --rm migrate

echo "Starting application containers..."
docker compose up -d --wait api web
docker compose ps

echo
echo "VHome is available at http://SERVER_IP:${VHOME_WEB_PORT:-5173}"
echo "Health: curl http://127.0.0.1:${VHOME_WEB_PORT:-5173}/api/v1/healthz"
