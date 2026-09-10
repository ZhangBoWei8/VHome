# VHome deployment

Images are built by GitHub Actions and pulled by tag on the server. Nothing is
compiled on the deployment host.

```
push/PR         → ci.yml      gofmt / vet / wire freshness / tests / layering / web build
workflow_dispatch → deploy.yml build+push api and web images tagged with the commit SHA
                              → self-hosted runner on the server runs deploy/deploy-remote.sh
                                → git checkout <sha> → migrate up → up -d --wait → verify commit
                                  → on failure, roll back to .last-good-tag
```

## Files

| File | Role |
|---|---|
| `compose.yaml` | Production topology. Images by tag, no build. |
| `compose.override.yaml` | Development only. Adds the build stanzas back. **Must not exist on the server.** |
| `deploy.sh` | First-time host setup and manual start. |
| `deploy/deploy-remote.sh` | The rollout: pin a tag, migrate, verify, roll back on failure. |
| `migrations/` | Versioned schema. See `migrations/README.md`. |

## One-time server setup

Everything below runs on the cloud server.

### 1. Check that the registry is reachable

```bash
time docker pull ghcr.io/homebrew/core/hello:latest
```

Under ~30s: use GHCR. Slow or timing out: use an Aliyun ACR namespace in the
same region instead, and set `VHOME_IMAGE_REGISTRY` accordingly.

### 2. Turn the deployment directory into a git clone

The rollout checks out the deployed commit so `compose.yaml` and the migrations
always match the image. Keep the existing `.env` — it is gitignored.

```bash
sudo mkdir -p /opt/vhome && sudo chown "$USER" /opt/vhome
cd /opt/vhome
git clone https://github.com/ZhangBoWei8/VHome.git .
cp /path/to/old/.env .env          # or run ./deploy.sh once to generate one
rm -f compose.override.yaml        # never build on the server
```

Add the image source to `.env`:

```dotenv
VHOME_IMAGE_REGISTRY=ghcr.io/zhangbowei8
VHOME_IMAGE_TAG=latest
```

### 3. Baseline the existing database

The production schema already has `000001` and `000002` applied by hand. Tell
`migrate` that, without re-running them. **Back up first.**

```bash
docker compose exec -T mysql sh -c \
  'exec mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" "$MYSQL_DATABASE"' > ~/vhome-backup-$(date +%F).sql

docker compose run --rm migrate force 2
docker compose run --rm migrate version    # must print 2
```

On a brand new database skip this: `migrate` applies both from scratch.

### 4. Log the Docker daemon into the registry

```bash
# GHCR: use a PAT with read:packages
echo "$GHCR_TOKEN" | docker login ghcr.io -u ZhangBoWei8 --password-stdin
```

### 5. Install the self-hosted runner

In GitHub: **Settings → Actions → Runners → New self-hosted runner**, pick
Linux x64 and follow the shown commands. When it asks for labels, add
`vhome-prod` — `deploy.yml` targets `[self-hosted, vhome-prod]`.

Install it as a service so it survives reboots:

```bash
sudo ./svc.sh install
sudo ./svc.sh start
```

The runner user must be able to run `docker` (`sudo usermod -aG docker $USER`)
and to write `/opt/vhome`.

### 6. Configure the repository

**Settings → Secrets and variables → Actions → Variables:**

| Variable | Value |
|---|---|
| `VHOME_IMAGE_REGISTRY` | `ghcr.io/zhangbowei8` |
| `VHOME_REGISTRY_HOST` | `ghcr.io` |
| `VHOME_DEPLOY_DIR` | `/opt/vhome` |

**Secrets** — only needed for a non-GHCR registry, which then needs
`VHOME_REGISTRY_USERNAME` and `VHOME_REGISTRY_PASSWORD`. With GHCR the workflow
falls back to `GITHUB_TOKEN`.

## Deploying

Manual, from the Actions tab: **Deploy → Run workflow**. Leave `ref` empty to
deploy the latest `main`, or paste a SHA to redeploy or roll back to it.

Once a few runs have gone through cleanly, enable automatic deploys by
uncommenting the `push: branches: [main]` trigger in
`.github/workflows/deploy.yml`.

### Rolling back

```bash
cd /opt/vhome
./deploy/deploy-remote.sh "$(cat .last-good-tag)"
```

Or re-run the Deploy workflow with the previous SHA in `ref`.

Rollback replaces the application image only. The schema stays where it is,
which is why migrations must follow the expand/contract rule in
`migrations/README.md`: the previous image has to keep working against the
current schema.

## Verifying a deploy

```bash
curl -s http://127.0.0.1:5173/api/v1/healthz
# {"status":"ok","commit":"<sha>","build_time":"...","api_version":"v1"}
```

`commit` must match the tag that was deployed. `deploy-remote.sh` checks exactly
this and rolls back if it does not match within 120s.

## Downtime

`docker compose up -d --wait` stops the old container before starting the new
one, so each deploy has roughly 5–15s of 502. That is acceptable for a family
application. For zero downtime, nginx must re-resolve the API container's
address, which needs `deploy/nginx.conf` changed to:

```nginx
resolver 127.0.0.11 valid=10s;
set $api_upstream http://api:8080;
location /api/ { proxy_pass $api_upstream; }
```

and a rolling replacement (for example the `docker rollout` compose plugin).
Without the resolver change, any rolling strategy 502s permanently because
nginx caches the old container IP.

## Ports

- `5173`: browser entry (nginx serves the built Vue app and proxies `/api/`).
- `3307`: MySQL, bound to host loopback only.
- `8080`: internal API port, not published.

## HTTPS

Put port 5173 behind an HTTPS reverse proxy, then set in `.env`:

```dotenv
VHOME_ENV=production
VHOME_AUTH_COOKIE_SECURE=true
```

and redeploy. `VHOME_ENV=production` with `COOKIE_SECURE=false` is rejected at
startup by config validation.

## Operations

```bash
docker compose ps
docker compose logs -f api
docker compose run --rm migrate version
cat .last-good-tag
```

`docker compose down` stops the stack without deleting the MySQL data or the
uploaded images. Never add `-v` unless you mean to delete both volumes.
