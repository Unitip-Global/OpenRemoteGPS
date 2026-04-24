# OpenRemoteSetup

Infrastructure-as-Code snapshot of the Railway **OpenRemoteGPS** environment (project `ERP`, env id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`).

This repo is the source-of-truth for the OpenRemote GPS stack topology, service configuration, and environment variables. Secrets are **not** stored here — placeholders only.

Public entry point: **https://gps.unitip.global**

## Stack

| Service | Image / Build | Purpose | Public |
|---|---|---|---|
| `openremote-nginx` | Dockerfile at repo root (`nginx:alpine` + `nginx.conf`) | Edge for `gps.unitip.global`; TLS terminated at Railway edge | ✅ |
| `manager` | `openremote/manager:latest` | OpenRemote core — assets, rules, MQTT | internal |
| `keycloak` | `openremote/keycloak:23.0.7.6` | Identity provider (OAuth2 / OIDC) | internal |
| `traccar` | Traccar server | GPS device ingest (TCP `:5027` Teltonika, HTTP `:8082`) | TCP proxy |
| `timescaledb` | TimescaleDB (Postgres) | Database for OpenRemote + Keycloak | internal |
| `traccar-transformer` | Go · Dockerfile (repo `services/traccar-transformer/src/`) | Reshapes Traccar's nested JSON forward payload → flat payload that `gps-adapter` parses | internal |
| `gps-adapter` | Go · Dockerfile (external) | Creates/updates OpenRemote `TrackerAsset` in realm `unitip` from positions pushed by `traccar-transformer` | internal |
| `backup` | Dockerfile cron `0 2 * * *` | Nightly `pg_dump` of `openremote` DB to volume | cron only |

## Layout

```
OpenRemoteSetup/
├── README.md                 # this file
├── Dockerfile                # builds the openremote-nginx edge image
├── nginx.conf                # routes / → manager, /auth → keycloak
├── railway.toml              # Railway build/deploy config for the nginx service
├── docker-compose.yml        # full stack, local dev
├── .env.example              # all env vars (secrets as placeholders)
├── services/
│   ├── openremote-nginx/     # vars.env only (env var reference)
│   ├── manager/              # vars.env
│   ├── keycloak/              # vars.env
│   ├── traccar/              # vars.env
│   ├── timescaledb/          # vars.env
│   ├── traccar-transformer/  # vars.env + SOURCE.md + src/ (Go source in repo)
│   ├── gps-adapter/          # vars.env + SOURCE.md (Go source external)
│   └── backup/               # vars.env + SOURCE.md
├── scripts/
│   ├── README.md                        # how to run the setup scripts
│   └── openremote_platform_setup.py     # idempotent: Fleet + TrackerAssets + rules
└── docs/
    ├── TOPOLOGY.md           # data flow, ports, network, fleet platform layout
    ├── VARIABLES.md          # full variable reference
    └── SECRETS.md            # what to rotate, where they live
```

## How to use this repo

**Local dev / testing:**
```bash
cp .env.example .env          # fill in real secrets
docker compose up -d
# nginx → http://localhost:8443
# keycloak admin → http://localhost:8081/auth
# traccar UI → http://localhost:8082
```

**Updating Railway:**

- **nginx edge (the only service built from this repo):** edit `nginx.conf`, `Dockerfile`, or root `railway.toml`, then `railway up` (links to project `ERP` / env `OpenRemoteGPS` / service `OpenRemoteGPS`). Commit the change after it's verified in prod.
- **All other services:** pull public Docker images directly in Railway. Change image tags, env vars, or scaling in the Railway dashboard. Mirror the new values in `services/<name>/vars.env` here so this repo keeps matching prod — treat drift as a bug.

**Rotating a secret:** update it on Railway first (env var), then update the placeholder value in `docs/SECRETS.md` tracking table. Never paste the real secret in this repo.

## Source code

Services built from a `Dockerfile` rather than a public image:

- `openremote-nginx` — built from the root `Dockerfile` + `nginx.conf` in this repo; deployed via `railway up`.
- `traccar-transformer` — Go source in this repo at `services/traccar-transformer/src/`; deployed via `railway up services/traccar-transformer/src` (linked to that service).
- `gps-adapter` — Go service; source lives elsewhere (see `services/gps-adapter/SOURCE.md`).
- `backup` — cron + `pg_dump`; source lives elsewhere (see `services/backup/SOURCE.md`).

For `gps-adapter` and `backup`, this repo captures deployment configuration only — not the source.

## Related

- ERP-side integration: `services/core/app/modules/fleet/gps/providers/openremote_provider.py` in the main `ERP` monorepo — the Core ERP consumes this stack as one of its GPS providers (OAuth2 client-credentials via Keycloak, REST for assets/datapoints, MQTT for real-time).

## Railway references

- Project: `ERP` (id `be3b01c9-51b2-4ff8-83ac-60cdca93c5a6`)
- Environment: `OpenRemoteGPS` (id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`)
- Region: `us-east4-eqdc4a`
- Timezone: `Europe/Bucharest`
