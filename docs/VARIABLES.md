# Variables reference

Every env var used by the stack, grouped by service. Values marked `__CHANGE_ME__` in `*.vars.env` and `.env.example` are **secrets** — see [SECRETS.md](SECRETS.md) for which ones need rotation.

Railway auto-injects its own `RAILWAY_*` variables (service ID, env name, private/public domain, volume IDs, TCP proxy domain). Those are intentionally **not** duplicated in `vars.env` because Railway owns them.

## Shared cross-service

| Variable | Purpose | Where |
|---|---|---|
| `TZ=Europe/Bucharest` | Timezone for all containers | nginx, manager, keycloak |
| `POSTGRES_DB=openremote` | Single DB name, shared by manager/keycloak/traccar | timescaledb, backup |
| `POSTGRES_USER=openremote` | Single DB user | timescaledb, backup |
| `POSTGRES_PASSWORD` | Single DB password (shared) | timescaledb, backup; mirrored as `OR_DB_PASSWORD`, `KC_DB_PASSWORD`, `TRACCAR_DB_PASSWORD` |
| `ADAPTER_SECRET_KEY` | Shared secret between `openremote-nginx` and `gps-adapter` for authenticated push | nginx, gps-adapter, traccar |

## openremote-nginx (edge)

| Variable | Example | Notes |
|---|---|---|
| `OR_HOSTNAME` | `gps.unitip.global` | Public hostname |
| `OR_MANAGER_URL` | `http://manager.railway.internal:8080` | Upstream for `/` |
| `OR_GPS_ADAPTER_URL` | `http://gps-adapter.railway.internal:8080` | Upstream for adapter routes |
| `OR_TRACCAR_URL` | `http://traccar.railway.internal:8082` | Upstream for traccar routes |
| `OR_KEYCLOAK_HOST` / `OR_KEYCLOAK_PORT` / `OR_KEYCLOAK_SECURE` | `keycloak.railway.internal` / `8080` / `false` | Keycloak upstream |
| `OR_DB_*` | postgres creds | Manager setup bootstraps against DB even on nginx |
| `OR_ADMIN_PASSWORD` | `__CHANGE_ME__` | Manager admin |
| `OR_DEV_MODE` | `false` | Prod |
| `OR_SETUP_TYPE` | `production` | |
| `OR_SETUP_RUN_ON_RESTART` | `true` | ⚠ Flip to `false` once stable — see [SECRETS.md](SECRETS.md) for rationale |
| `OR_MAP_SETTINGS_PATH` | `/deployment/map/mapsettings.json` | Baked into nginx image |
| `OR_MAP_TILES_PATH` | `/opt/map/mapdata.mbtiles` | Baked into nginx image |
| `OR_LOGGING_CONFIG_FILE` | `/opt/map/logging.properties` | |

## manager (OpenRemote core)

Same `OR_*` block as nginx except no `OR_MANAGER_URL/OR_GPS_ADAPTER_URL/OR_TRACCAR_URL` (it is the manager).

| Variable | Example |
|---|---|
| `OR_HTTP_PORT` | `8080` |
| `OR_SSL_PORT` | `-1` (TLS terminated upstream) |
| `OR_DB_HOST` | `timescaledb.railway.internal` |
| `OR_DB_NAME` | `openremote` |
| `OR_DB_USER` / `OR_DB_PASSWORD` | `openremote` / `__CHANGE_ME__` |
| `OR_KEYCLOAK_HOST` | `keycloak.railway.internal` |

## keycloak

| Variable | Example |
|---|---|
| `KEYCLOAK_ADMIN` | `admin` |
| `KEYCLOAK_ADMIN_PASSWORD` | `__CHANGE_ME__` (was `Admin`) |
| `KC_DB` | `postgres` |
| `KC_DB_URL` | `jdbc:postgresql://timescaledb.railway.internal:5432/openremote` |
| `KC_DB_USERNAME` / `KC_DB_PASSWORD` | `openremote` / `__CHANGE_ME__` |
| `KC_HOSTNAME` | `gps.unitip.global` |
| `KC_HOSTNAME_STRICT` | `false` |
| `KC_HTTP_ENABLED` | `true` |
| `KC_HTTP_PORT` | `8080` |
| `KC_HTTP_RELATIVE_PATH` | `/auth` |
| `KC_PROXY` | `edge` |
| `KC_PROXY_HEADERS` | `xforwarded` |

## traccar

| Variable | Example |
|---|---|
| `APP_ENV` | `production` |
| `APP_NAME` | `traccar` |
| `ADMIN_EMAIL` | `admin@unitip.ro` |
| `ADMIN_PASSWORD` | `__CHANGE_ME__` (was `Unitip123!`) |
| `TRACCAR_HOST` / `TRACCAR_PORT` | `0.0.0.0` / `8082` |
| `PORT` | `8082` (duplicate of above for Railway) |
| `GPS_PORT` | `5055` (device TCP ingest) |
| `TRACCAR_DB_USER` / `TRACCAR_DB_PASSWORD` | shared Postgres creds |

## traccar-transformer

Shim service that reshapes Traccar's native `forward.type=json` (nested `{device, position, event}`) into the flat JSON the `gps-adapter` binary parses. Source in this repo at [`services/traccar-transformer/src/`](../services/traccar-transformer/src).

| Variable | Example | Notes |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `ADAPTER_URL` | `http://gps-adapter.railway.internal:8080/gps/position` | Where the flat payload is POSTed |
| `FORWARD_INVALID` | `false` | If `true`, forward positions with `valid=false` (device has no GPS fix). Default `false` to avoid freezing assets on stale last-known coords. |

## gps-adapter

| Variable | Example |
|---|---|
| `PORT` | `8080` |
| `GIN_MODE` | `release` (Go/Gin production mode) |
| `OPENREMOTE_URL` | `http://manager.railway.internal:8080` |
| `OPENREMOTE_REALM` | `unitip` (was `master` before realm split — see [TOPOLOGY.md](TOPOLOGY.md#realms)) |
| `OPENREMOTE_CLIENT_ID` | `openremote` |
| `OPENREMOTE_USER` / `OPENREMOTE_PASSWORD` | `admin` / `__CHANGE_ME__` (unitip-realm admin password, rotate — see [SECRETS.md](SECRETS.md)) |
| `KEYCLOAK_URL` | `http://keycloak.railway.internal:8080` |
| `TRACCAR_URL` | `http://traccar.railway.internal:8082` |
| `TRACCAR_USER` / `TRACCAR_PASSWORD` | `admin@unitip.ro` / `__CHANGE_ME__` |
| `ADAPTER_SECRET_KEY` | `__CHANGE_ME__` |

## timescaledb

| Variable | Example |
|---|---|
| `POSTGRES_DB` | `openremote` |
| `POSTGRES_USER` | `openremote` |
| `POSTGRES_PASSWORD` | `__CHANGE_ME__` |
| `PGDATA` | `/home/postgres/pgdata/data` |

## backup

| Variable | Example |
|---|---|
| `BACKUP_DIR` | `/backups` |
| `POSTGRES_HOST` | `timescaledb.railway.internal` |
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | shared |
