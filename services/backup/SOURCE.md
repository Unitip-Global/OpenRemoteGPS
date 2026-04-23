# Backup — source

Dockerfile-built cron service. Source not in this repo.

**TODO (denis):** fill in actual repo URL / Dockerfile contents, e.g.:

```
Repo:   github.com/<org>/<repo>
Branch: main
```

## Deployment signature

- Build: `DOCKERFILE`, path `Dockerfile`
- Cron: `0 2 * * *` (02:00 Europe/Bucharest daily)
- Restart policy: `ON_FAILURE` x3
- Volume: `/backups` (persistent, Railway volume `openremotegps-backup-volume`)
- Last SUCCESS build: `417e5954-9a12-4c65-929a-740d26729477` at 2026-03-30T08:42:00Z

## Role

Nightly `pg_dump` of the `openremote` database from `timescaledb.railway.internal`, written to `/backups`.

## Things to verify

- Is the cron actually firing post-2026-03-30? The last recorded build was on that date; cron builds don't always show as new deploys.
- Is there rotation / retention on `/backups`? (Without it, the volume fills up eventually.)
- Is there off-site copy (S3, B2) or is `/backups` the only copy? If only copy — single point of failure.
