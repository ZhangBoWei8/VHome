# VHome Docker deployment

## One-command deployment

Requirements: Docker Engine, Docker Compose, and OpenSSL.

```bash
chmod +x deploy.sh
./deploy.sh
```

Open `http://SERVER_IP:5173` after all containers become healthy.

The first run creates a private `.env` containing random MySQL passwords. Keep
this file and the named Docker volumes when upgrading.

## Ports

- `5173`: VHome browser entry (Nginx serves the built Vue application and proxies API/upload requests).
- `3307`: the dedicated VHome MySQL instance, bound only to host loopback.
- `3306`: MySQL's internal container port; it is not used by the API through the host.
- `8080`: internal API port; it is not published on the host.

Change `VHOME_WEB_PORT` or `VHOME_MYSQL_HOST_PORT` in `.env` if either host port
is occupied.

## Database initialization

`internal/DB/mysql/init.sql` is mounted into
`/docker-entrypoint-initdb.d/001-init.sql`. The official MySQL image executes it
automatically only when `vhome_mysql_data` is empty. It creates all current
tables and seeds the built-in locations and material templates. The bind mount
uses a private SELinux label so it also works on enforcing CentOS hosts.

Changing `init.sql` does not migrate an existing database volume. Future schema
changes must be applied through versioned migrations.

## HTTPS

The generated configuration supports immediate access over HTTP. For an
internet-facing deployment, place port 5173 behind an HTTPS reverse proxy, then
set the following values in `.env` and recreate the API container:

```dotenv
VHOME_ENV=production
VHOME_AUTH_COOKIE_SECURE=true
```

```bash
docker compose up -d --build
```

## Operations

```bash
docker compose ps
docker compose logs -f
docker compose pull
docker compose up -d --build
```

`docker compose down` stops the stack without deleting MySQL data or uploaded
images. Do not add `-v` unless you intentionally want to delete both volumes.
