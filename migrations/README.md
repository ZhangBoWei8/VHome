# Database migrations

Every schema change goes here. The `migrate` service in `compose.yaml` applies
them before the API container starts, so a deploy can never run new code
against an old schema.

## Rules

1. **Never edit an applied migration.** Add a new one instead. `migrate` records
   a checksum-free version number, but a changed file still means two
   deployments disagree about what the schema is.
2. **Expand, then contract.** Add a column in one release, start writing to it
   in the next, drop the old one in a third. A deploy that adds a column and
   removes another in the same step cannot be rolled back by redeploying the
   previous image.
3. **`*.down.sql` is for throwaway databases.** Production rollback means
   redeploying the previous image against the *current* schema, which only
   works if rule 2 was followed.

## Adding a migration

```bash
# pick the next number, keep the six digit padding
migrations/000003_add_health_records.up.sql
migrations/000003_add_health_records.down.sql
```

Test it locally against a scratch database before committing:

```bash
docker compose up -d mysql
docker compose run --rm migrate up
```

## Baselining an existing database

`000001` and `000002` were already applied by hand on the deployment that
predates this directory (`init.sql` through the MySQL entrypoint, then the memos
SQL piped in by `deploy.sh`). Mark them as applied without re-running:

```bash
docker compose run --rm migrate force 2
docker compose run --rm migrate version   # must print: 2
```

Take a `mysqldump` backup first. On a brand new database, skip this — `migrate`
applies both from scratch.

## Why init.sql is no longer mounted

`internal/DB/mysql/init.sql` used to be mounted into
`/docker-entrypoint-initdb.d`, which only runs on an empty data volume. Fresh
and upgraded databases therefore took different paths and could drift. Both now
go through this directory. The file is kept as the historical source of
`000001` and is no longer read at runtime.
