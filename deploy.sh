#!/bin/sh
set -eu

cd "$(dirname "$0")"

if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate deployment secrets" >&2
    exit 1
fi

if [ ! -f .env ]; then
    umask 077
    db_password="$(openssl rand -hex 24)"
    root_password="$(openssl rand -hex 24)"
    secret_encryption_key="$(openssl rand -base64 32)"

    {
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
        printf '%s\n' 'DEEPSEEK_MODEL=deepseek-v4'
        printf '%s\n' 'DEEPSEEK_BASEURL=https://api.deepseek.com'
        printf '%s\n' 'GLM_APIKEY='
        printf '%s\n' 'GLM_MODEL='
        printf '%s\n' 'GLM_BASEURL='
    } > .env

    echo "Generated private deployment configuration: .env"
fi

if ! grep -q '^VHOME_SECRET_ENCRYPTION_KEY=' .env; then
    umask 077
    secret_encryption_key="$(openssl rand -base64 32)"
    printf 'VHOME_SECRET_ENCRYPTION_KEY=%s\n' "$secret_encryption_key" >> .env
    echo "Added the notification encryption key to the existing private .env file."
fi

if ! grep -q '^DEEPSEEK_APIKEY=' .env; then
    umask 077
    {
        printf '%s\n' 'DEEPSEEK_APIKEY='
        printf '%s\n' 'DEEPSEEK_MODEL=deepseek-v4'
        printf '%s\n' 'DEEPSEEK_BASEURL=https://api.deepseek.com'
        printf '%s\n' 'GLM_APIKEY='
        printf '%s\n' 'GLM_MODEL='
        printf '%s\n' 'GLM_BASEURL='
    } >> .env
    echo "Added empty LLM provider settings to the existing private .env file."
fi

docker compose up -d mysql

attempt=0
until docker compose exec -T mysql sh -c 'mysqladmin ping -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD" --silent' >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
        echo "MySQL did not become ready in time." >&2
        exit 1
    fi
    sleep 2
done

docker compose exec -T mysql sh -c \
    'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' \
    < internal/DB/mysql/migrations/20260814_memos_notifications.sql

docker compose up -d --build api web
docker compose ps

echo "VHome is starting at http://SERVER_IP:5173"
echo "MySQL is mapped to host port 3307. Application containers use mysql:3306 internally."
