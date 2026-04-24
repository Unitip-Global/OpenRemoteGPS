# Variables reference

Single-service stack. All vars apply to the `traccar` Railway service.
Values marked `__CHANGE_ME__` in `*.vars.env` and `.env.example` are
**secrets** — see [SECRETS.md](SECRETS.md) for which ones need rotation.

Railway auto-injects its own `RAILWAY_*` variables (service ID, env name,
private/public domain, volume IDs, TCP proxy domain). Those are
intentionally **not** duplicated in `vars.env` because Railway owns them.

## traccar (the only service)

| Variable | Example | Notes |
|---|---|---|
| `TZ` | `Europe/Bucharest` | Process timezone; used for log timestamps and for the backup loop's "02:00 local" calculation (but the loop itself computes in UTC — see SOURCE.md). |
| `APP_ENV` | `production` | Traccar app env label |
| `APP_NAME` | `traccar` | Traccar app name label |
| `ADMIN_EMAIL` | `admin@unitip.ro` | Seed admin user. Used only on first startup; afterwards the credential lives in the H2 DB. |
| `ADMIN_PASSWORD` | `__CHANGE_ME__` | Seed password. Rotate via Traccar UI (Account → Change password), then update this placeholder. |
| `TRACCAR_HOST` | `0.0.0.0` | HTTP listen address |
| `TRACCAR_PORT` | `8082` | HTTP listen port (reached via `gps.unitip.global`) |
| `PORT` | `8082` | Duplicate of `TRACCAR_PORT`, required by Railway's healthcheck conventions. |
| `GPS_PORT` | `5027` | Teltonika Codec 8/8E TCP ingest port, reached via `shortline.proxy.rlwy.net:57840` |
| `TRACCAR_BACKUP_RETENTION_DAYS` | `14` | Documented for the operator. Actual enforcement is in the `find -mtime +14` clause inside `/opt/traccar/data/run_backup.sh` — change both together. |

### Things NOT set as env vars

- **Database config** — `traccar.xml` is written at container boot by the
  `startCommand` wrapper, using hardcoded H2 URL
  `jdbc:h2:./data/database;AUTO_SERVER=TRUE`. Changing DB requires
  changing the startCommand (see SOURCE.md).
- **Forward config** — not set. Devices post to Traccar; nothing downstream.
  When the ERP adapter is added later, either (a) it pulls from Traccar's
  REST API, or (b) we reintroduce `forward.enable=true` + `forward.url` in
  traccar.xml via an updated startCommand.

## Removed variables (for reference — if they come up in old git history)

The following were removed together with the services that used them on
2026-04-24. Don't re-add without adding the corresponding service back.

- `OR_*` — OpenRemote manager / nginx edge
- `KC_*` — Keycloak
- `OPENREMOTE_*` — gps-adapter Go service
- `KEYCLOAK_*` — keycloak
- `POSTGRES_*` / `PGDATA` — timescaledb
- `ADAPTER_SECRET_KEY` — nginx ↔ adapter shared secret
- `TRACCAR_DB_USER` / `TRACCAR_DB_PASSWORD` — Traccar Postgres creds
  (aspirational; Traccar always ran on H2)
- `TRACCAR_FORWARD_TARGET` — marker var used to trigger redeploy
- `JAVA_TOOL_OPTIONS` — experiment to inject `-Dforward.*`
- `TRANSFORMER_ADAPTER_URL` / `TRANSFORMER_FORWARD_INVALID` — transformer
- `FLEET_PARENT_ID` / `FORWARD_INVALID` — gps-adapter
