# OpenRemoteSetup

Infrastructure-as-Code snapshot of the Railway **OpenRemoteGPS** environment (project `ERP`, env id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`).

This repo is the source-of-truth for the OpenRemote GPS stack topology, service configuration, and environment variables. Secrets are **not** stored here — placeholders only.

Public entry point: **https://gps.unitip.global**

## Stack

| Service | Image / Build | Purpose | Public |
|---|---|---|---|
| `openremote-nginx` | `insiderfyr/openremote-nginx:latest` | Edge / TLS termination for `gps.unitip.global` | ✅ |
| `manager` | `openremote/manager:latest` | OpenRemote core — assets, rules, MQTT | internal |
| `keycloak` | `openremote/keycloak:23.0.7.6` | Identity provider (OAuth2 / OIDC) | internal |
| `traccar` | Traccar server | GPS device ingest (TCP `:5055`, HTTP `:8082`) | TCP proxy |
| `timescaledb` | TimescaleDB (Postgres) | Database for OpenRemote + Keycloak + Traccar | internal |
| `gps-adapter` | Go · Dockerfile | Bridge: Traccar ↔ OpenRemote asset model | internal |
| `backup` | Dockerfile cron `0 2 * * *` | Nightly `pg_dump` of `openremote` DB to volume | cron only |

## Layout

```
OpenRemoteSetup/
├── README.md                 # this file
├── docker-compose.yml        # full stack, local dev
├── .env.example              # all env vars (secrets as placeholders)
├── services/
│   ├── openremote-nginx/     # railway.toml + vars.env
│   ├── manager/
│   ├── keycloak/
│   ├── traccar/
│   ├── timescaledb/
│   ├── gps-adapter/          # + SOURCE.md (where the Go code lives)
│   └── backup/               # + SOURCE.md
└── docs/
    ├── TOPOLOGY.md           # data flow, ports, network
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

**Updating Railway:** change the relevant `services/<name>/railway.toml` or `vars.env` here, commit, then apply on Railway (manually via dashboard/CLI, or via CI/CD once wired up). This repo should always match prod — treat drift as a bug.

**Rotating a secret:** update it on Railway first (env var), then update the placeholder value in `docs/SECRETS.md` tracking table. Never paste the real secret in this repo.

## Source code

Two services in this stack are built from a `Dockerfile`, not from a public image:

- `gps-adapter` — Go service; see `services/gps-adapter/SOURCE.md`
- `backup` — cron + `pg_dump`; see `services/backup/SOURCE.md`

The actual source for these lives elsewhere (see those `SOURCE.md` files). This repo captures the deployment configuration only.

## Related

- ERP-side integration: `services/core/app/modules/fleet/gps/providers/openremote_provider.py` in the main `ERP` monorepo — the Core ERP consumes this stack as one of its GPS providers (OAuth2 client-credentials via Keycloak, REST for assets/datapoints, MQTT for real-time).

## Railway references

- Project: `ERP` (id `be3b01c9-51b2-4ff8-83ac-60cdca93c5a6`)
- Environment: `OpenRemoteGPS` (id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`)
- Region: `us-east4-eqdc4a`
- Timezone: `Europe/Bucharest`
