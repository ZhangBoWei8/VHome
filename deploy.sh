#!/bin/sh
set -eu

cd "$(dirname "$0")"

if [ ! -f .env ]; then
    if ! command -v openssl >/dev/null 2>&1; then
        echo "openssl is required to generate deployment passwords" >&2
        exit 1
    fi

    umask 077
    db_password="$(openssl rand -hex 24)"
    root_password="$(openssl rand -hex 24)"

    {
        printf '%s\n' 'VHOME_WEB_PORT=5173'
        printf '%s\n' 'VHOME_MYSQL_HOST_PORT=3307'
        printf '%s\n' 'VHOME_MYSQL_DATABASE=vhome'
        printf '%s\n' 'VHOME_MYSQL_USER=vhome'
        printf 'VHOME_MYSQL_PASSWORD=%s\n' "$db_password"
        printf 'VHOME_MYSQL_ROOT_PASSWORD=%s\n' "$root_password"
        printf '%s\n' 'VHOME_ENV=development'
        printf '%s\n' 'VHOME_AUTH_COOKIE_SECURE=false'
    } > .env

    echo "Generated private deployment configuration: .env"
fi

docker compose up -d --build
docker compose ps

echo "VHome is starting at http://SERVER_IP:5173"
echo "MySQL is mapped to host port 3307. Application containers use mysql:3306 internally."
