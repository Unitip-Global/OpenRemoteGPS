# Topology

Public entry: **https://gps.unitip.global** → `openremote-nginx` service on Railway.

Default realm for business data: **`unitip`** ("Unitip Global"). Realm `master` reserved for superadmin only.

## Data flow

```
                     ┌─────────────────────────────────────────────────────────┐
                     │                                                         │
                     │         Railway project: ERP                            │
                     │         Environment: OpenRemoteGPS                      │
                     │                                                         │
 GPS devices         │   ┌──────────────┐       ┌─────────────────┐            │
  (Teltonika,  TCP   │   │              │ JSON  │   traccar-      │            │
   OsmAnd,    :5027  │   │   traccar    │──────►│   transformer   │            │
   etc.) ────────────┼──►│  :8082/:5027 │ POST  │   :8080 (Go)    │            │
                     │   │              │nested │ nested→flat     │            │
                     │   └──────┬───────┘       └────────┬────────┘            │
                     │          │                        │ flat                │
                     │          │ JDBC                   │ POST                │
                     │          │                        ▼                     │
                     │          │             ┌──────────────────┐             │
                     │          │             │   gps-adapter    │             │
                     │          │             │   :8080 (Go)     │             │
                     │          ▼             │   OAuth2 + REST  │             │
                     │   ┌──────────────┐     └──────────┬───────┘             │
                     │   │              │                │                     │
                     │   │ timescaledb  │                │ REST / MQTT         │
                     │   │   :5432      │                │ (realm=unitip)      │
                     │   │  (Postgres)  │─────►          ▼                     │
                     │   │              │ JDBC  ┌──────────────┐               │
                     │   └──────┬───────┘       │   manager    │               │
                     │          │               │   :8080      │               │
                     │          │ JDBC          │ (OpenRemote) │               │
                     │          ▼               └──────┬───────┘               │
                     │   ┌──────────────┐              │ OIDC                  │
                     │   │   keycloak   │◄─────────────┘                       │
                     │   │   :8080      │      ┌──────────────┐                │
                     │   │  (realms:    │      │openremote-   │                │
                     │   │   master,    │      │  nginx       │◄───────────────┼─── gps.unitip.global
                     │   │   unitip)    │      │   :8080      │                │    (public HTTPS, Fastly edge)
                     │   └──────────────┘      └──────────────┘                │
                     │                                                         │
                     │   ┌──────────────┐                                      │
                     │   │    backup    │   pg_dump → /backups                 │
                     │   │  cron 02:00  │   (Railway volume)                   │
                     │   └──────────────┘                                      │
                     │                                                         │
                     └─────────────────────────────────────────────────────────┘

                               ▲
                               │  OAuth2 client-credentials + REST/MQTT, realm=unitip
                               │
                     ┌─────────┴─────────┐
                     │                   │
                     │    Core ERP       │   services/core/app/modules/fleet/
                     │  (main monorepo)  │     gps/providers/openremote_provider.py
                     │                   │
                     └───────────────────┘
```

## Position flow (detail)

1. Teltonika device opens TCP to `shortline.proxy.rlwy.net:57840` (Railway TCP proxy → traccar :5027 Teltonika binary).
2. Traccar parses Codec 8/8E binary, stores position in H2 DB, and forwards via its built-in position forwarder (configured via startCommand override; see service settings in Railway).
3. Forwarder sends Traccar-style **nested** JSON (`{device:{...},position:{...},event:{...}}`) to `http://traccar-transformer.railway.internal:8080/webhook`.
4. `traccar-transformer` reshapes to **flat** JSON (`{deviceId,uniqueId,latitude,longitude,speed,valid,batteryLevel,...}`) and POSTs to `http://gps-adapter.railway.internal:8080/gps/position`. It drops `valid=false` positions by default (no-GPS-fix noise).
5. `gps-adapter` looks up the device in Traccar by `deviceId`, creates or updates a `TrackerAsset` in OpenRemote realm `unitip` via the Manager REST API. OAuth2 password grant against Keycloak (client `openremote` in realm `unitip`).
6. OpenRemote Manager persists to `timescaledb`, pushes updates to any MQTT subscribers, and exposes `/api/unitip/...` for the frontend and Core ERP.

## Ports

| Service | Internal | External | Purpose |
|---|---|---|---|
| openremote-nginx (Railway service `OpenRemoteGPS`) | 8080 | 443 via `gps.unitip.global` | Public HTTPS edge |
| manager | 8080 | — (internal) | OpenRemote API/UI |
| keycloak | 8080 | optional `keycloak-openremotegps.up.railway.app` | Identity provider |
| traccar | 8082 (HTTP), 5027 (TCP Teltonika) | 5027 via `shortline.proxy.rlwy.net:57840` | GPS device ingest |
| timescaledb | 5432 | — (internal only) | Shared Postgres |
| traccar-transformer | 8080 | — (internal) | Traccar nested JSON → adapter flat JSON |
| gps-adapter | 8080 | — (internal) | Traccar device lookup + OpenRemote TrackerAsset create/update |
| backup | — | — | Nightly cron |

## Private network names (Railway internal)

All services reach each other via `<name>.railway.internal`:

- `openremotegps.railway.internal` (nginx)
- `manager.railway.internal`
- `keycloak.railway.internal`
- `traccar.railway.internal`
- `traccar-transformer.railway.internal`
- `timescaledb.railway.internal`
- `gps-adapter.railway.internal`
- `openremotegps-backup.railway.internal`

## Volumes (Railway-managed, persistent)

| Volume | Mounted on | Mount path |
|---|---|---|
| `timescaledb-volume-l4FW` | timescaledb | `/pgdata` |
| `traccar-volume` | traccar | `/opt/traccar/data` |
| `openremotegps-backup-volume` | backup | `/backups` |

Note: Traccar currently uses local H2 at `/opt/traccar/data/database.*` (baked into traccar.xml via startCommand override), NOT the shared `timescaledb`. The `openremote` Postgres DB is used by `manager` and `keycloak` only.

## Realms

- **`master`** — Keycloak/OpenRemote superadmin. Used only for administering other realms. Contains `admin` user (password `secret`, rotate soon).
- **`unitip`** — Unitip Global's tenant. Default locale **ro**, display name "Unitip Global". All GPS fleet data (TrackerAsset instances, users, rules) lives here. Contains realm-admin user `admin` (password rotated; see SECRETS.md).

Browser URL per realm:
- Master admin: `https://gps.unitip.global/manager/` (default, or `?realm=master`)
- Unitip: `https://gps.unitip.global/manager/?realm=unitip`

## Fleet platform layout (inside the `unitip` realm)

```
Fleet Unitip Global (GroupAsset)
├── B 154 UIP (TrackerAsset)              ← traccarDeviceId=257 (the live Teltonika)
├── 01 Autobetoniera Fiori BF99H          ← traccarDeviceId=8
├── 02 Autogreder Volvo G 946B B 11954    ← traccarDeviceId=10
├── ... 254 more vehicles ...             ← 1:1 mirror of devices in Traccar
└── (any new device added in Traccar)     ← adapter creates the asset on first position push
```

Seeding + rules come from [`scripts/openremote_platform_setup.py`](../scripts/openremote_platform_setup.py)
(idempotent; safe to re-run after adding devices in Traccar).

### Baseline realm rulesets

Ten JSON rulesets are pre-seeded, all **disabled** by default (enable from the Rules UI once thresholds
fit your fleet):

| # | Rule | Trigger |
|---|---|---|
| 01 | Supraviteza critica >130kmh | `speed > 130` |
| 02 | Supraviteza >90kmh | `speed > 90` (DN limit) |
| 03 | Viteza urbana >50kmh | `speed > 50` (urban limit) |
| 04 | Baterie critica <10% | `batteryLevel < 10` |
| 05 | Baterie scazuta <20% | `batteryLevel < 20` |
| 06 | Combustibil scazut <15% | `fuelLevel < 15` |
| 07 | Altitudine extrema >2500m | `altitude > 2500` |
| 08 | Contact pornit | `ignition == true` |
| 09 | Contact oprit | `ignition == false` |
| 10 | Raport saptamanal kilometraj | cron `0 0 8 ? * MON` |

Each rule writes a status note to the matching asset's `notes` attribute. Swap to push
notifications / webhooks / alarms from the Rules UI when you wire up notification channels.

### Ongoing mirror (Traccar → OpenRemote)

- `traccar-transformer` + `gps-adapter` keep every TrackerAsset's `location`, `speed`,
  `altitude`, `batteryLevel`, etc. live from Traccar positions.
- `gps-adapter` currently does **not** parent newly-created assets to Fleet automatically
  (it creates them at realm root). After a new device shows up in Traccar and the adapter
  creates its asset, re-run `scripts/openremote_platform_setup.py` to move it under Fleet.

## Shared database

A single `timescaledb` instance hosts the `openremote` database, used by **manager** and **keycloak** (its own schemas). All connect as user `openremote`. Convenient but one compromised credential gives access to both — worth splitting later.

## External dependency

- **Core ERP** (main monorepo) consumes this stack as one of its GPS providers. See `services/core/app/modules/fleet/gps/providers/openremote_provider.py`. It authenticates via Keycloak OAuth2 (realm `unitip`, client `openremote`) and pulls vehicle/position data from `manager` (REST + MQTT).
