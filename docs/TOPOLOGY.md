# Topology

Public entry: **https://gps.unitip.global** → `openremote-nginx` service on Railway.

## Data flow

```
                     ┌──────────────────────────────────────────────┐
                     │                                              │
                     │         Railway project: ERP                 │
                     │         Environment: OpenRemoteGPS           │
                     │                                              │
                     │                                              │
 GPS devices         │   ┌──────────────┐      ┌──────────────┐     │
  (OsmAnd,      TCP  │   │              │      │              │     │
   Teltonika,  :5055 │   │   traccar    │◄─────│ gps-adapter  │     │
   etc.) ────────────┼──►│  :8082/:5055 │ REST │  :8080 (Go)  │     │
                     │   │              │      │              │     │
                     │   └──────┬───────┘      └──────┬───────┘     │
                     │          │                     │             │
                     │          │ JDBC                │ REST/MQTT   │
                     │          │                     │ OAuth2      │
                     │          ▼                     ▼             │
                     │   ┌──────────────┐      ┌──────────────┐     │
                     │   │              │◄─────│              │     │
                     │   │ timescaledb  │      │   manager    │     │
                     │   │   :5432      │      │   :8080      │     │
                     │   │  (Postgres)  │─────►│ (OpenRemote) │     │
                     │   │              │ JDBC │              │     │
                     │   └──────┬───────┘      └──────┬───────┘     │
                     │          │                     │             │
                     │          │ JDBC                │ OIDC        │
                     │          ▼                     ▼             │
                     │   ┌──────────────┐      ┌──────────────┐     │
                     │   │   keycloak   │◄─────│openremote-   │     │
                     │   │   :8080      │      │  nginx       │◄────┼──── gps.unitip.global
                     │   │              │      │   :8080      │     │    (public HTTPS)
                     │   └──────────────┘      └──────────────┘     │
                     │                                              │
                     │   ┌──────────────┐                           │
                     │   │    backup    │   pg_dump → /backups      │
                     │   │  cron 02:00  │   (Railway volume)        │
                     │   └──────────────┘                           │
                     │                                              │
                     └──────────────────────────────────────────────┘

                               ▲
                               │  OAuth2 client-credentials + REST/MQTT
                               │
                     ┌─────────┴─────────┐
                     │                   │
                     │    Core ERP       │   services/core/app/modules/fleet/
                     │  (main monorepo)  │     gps/providers/openremote_provider.py
                     │                   │
                     └───────────────────┘
```

## Ports

| Service | Internal | External | Purpose |
|---|---|---|---|
| openremote-nginx | 8080 | 443 via `gps.unitip.global` | Public HTTPS edge |
| manager | 8080 | — (internal) | OpenRemote API/UI |
| keycloak | 8080 | optional `keycloak-openremotegps.up.railway.app` | Identity provider |
| traccar | 8082 (HTTP), 5055 (TCP) | 5055 exposed via `gondola.proxy.rlwy.net:40590` | GPS device ingest |
| timescaledb | 5432 | — (internal only) | Shared Postgres |
| gps-adapter | 8080 | — (internal) | Traccar↔OpenRemote bridge |
| backup | — | — | Nightly cron |

## Private network names (Railway internal)

All services reach each other via `<name>.railway.internal`:

- `openremotegps.railway.internal` (nginx)
- `manager.railway.internal`
- `keycloak.railway.internal`
- `traccar.railway.internal`
- `timescaledb.railway.internal`
- `gps-adapter.railway.internal`
- `openremotegps-backup.railway.internal`

## Volumes (Railway-managed, persistent)

| Volume | Mounted on | Mount path |
|---|---|---|
| `timescaledb-volume-l4FW` | timescaledb | `/pgdata` |
| `traccar-volume` | traccar | `/opt/traccar/data` |
| `openremotegps-backup-volume` | backup | `/backups` |

## Shared database

A single `timescaledb` instance hosts the `openremote` database, used by **manager**, **keycloak** (in schema `public` / Keycloak's own), and **traccar**. All three connect as user `openremote`. This is convenient but means one compromised credential gives access to everything — worth splitting later.

## External dependency

- **Core ERP** (main monorepo) consumes this stack as one of its GPS providers. See `services/core/app/modules/fleet/gps/providers/openremote_provider.py`. It authenticates via Keycloak OAuth2 client-credentials and pulls vehicle/position data from `manager` (REST + MQTT).
