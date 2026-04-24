# OpenRemoteSetup

> The repo name is legacy. As of 2026-04-24 this stack runs **Traccar only**
> — OpenRemote, Keycloak, TimescaleDB, the Go adapter, the transformer and
> the nginx edge have all been removed. Adapters (future ERP integration)
> will be added back as separate services.

Infrastructure-as-Code snapshot of the Railway **OpenRemoteGPS** environment
(project `ERP`, env id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`).

Public entry point: **https://gps.unitip.global** → Traccar web UI.
Device TCP ingest: **shortline.proxy.rlwy.net:57840** → Traccar `:5027`
(Teltonika Codec 8 / 8E).

## Current stack (1 service)

| Service | Image / Build | Purpose | Public |
|---|---|---|---|
| `traccar` | `traccar/traccar:latest` + `startCommand` override | GPS ingest (Teltonika TCP :5027), web UI (HTTP :8082), embedded H2 DB, daily self-backup at 02:00 UTC | ✅ `gps.unitip.global` + TCP proxy |

Nothing else. No Postgres, no Keycloak, no reverse proxy, no adapter.

## Data persistence

- **Live DB**: H2 on-disk at `/opt/traccar/data/database.mv.db`, served from
  the `traccar-volume` Railway volume.
- **Backups**: daily gzip'd SQL dump under `/opt/traccar/data/backup/`,
  retention 14 days. Dump runs in-container via a `while sleep` loop
  spawned by `startCommand` (no separate cron service — Railway volumes
  can only attach to one service, so backup reads the H2 file from the
  same container). See [services/traccar/SOURCE.md](services/traccar/SOURCE.md)
  for the full script and restore instructions.

## Layout

```
OpenRemoteSetup/
├── README.md                # this file
├── docker-compose.yml       # local dev parity with Railway
├── .env.example             # minimal env reference
├── services/
│   └── traccar/
│       ├── vars.env         # env var reference
│       └── SOURCE.md        # Railway startCommand + backup script + restore notes
└── docs/
    ├── TOPOLOGY.md          # data flow + ports
    ├── VARIABLES.md         # full variable reference
    └── SECRETS.md           # what to rotate, where it lives
```

## How to use this repo

**Local dev:**
```bash
cp .env.example .env             # fill ADMIN_PASSWORD
docker compose up -d
# Traccar web UI → http://localhost:8082  (login admin@unitip.ro / ADMIN_PASSWORD)
# Device TCP ingest → localhost:5027 (Teltonika Codec 8/8E)
# One-off backup → docker compose exec traccar /opt/traccar/data/run_backup.sh
```

**Updating Railway:**

- Traccar uses the stock public image; the customization lives in the
  service's `startCommand` (Railway-managed setting, not a file in this
  repo). To change it, edit the canonical copy in
  [services/traccar/SOURCE.md](services/traccar/SOURCE.md), then apply the
  change via the GraphQL `serviceInstanceUpdate` mutation on the Railway
  API. Commit the updated SOURCE.md so the repo stays the record of truth.
- For env var changes, edit `services/traccar/vars.env` here and set the
  matching value on the Railway service. Treat drift as a bug.

**Rotating a secret:** change in Railway, then update `docs/SECRETS.md`
with the rotation date and placeholder. Never commit the real value.

## Related / future

- Adapters (ERP integration, custom webhooks, asset-level bridges) will be
  added as separate Railway services. Each new service gets its own dir
  under `services/<name>/` and its own row in this table.
- Custom domain + TCP proxy stay on Traccar; new services can use Railway
  internal DNS `traccar.railway.internal` to reach the REST API and the
  internal WebSocket feed.

## Railway references

- Project: `ERP` (id `be3b01c9-51b2-4ff8-83ac-60cdca93c5a6`)
- Environment: `OpenRemoteGPS` (id `a921bf3f-c1f4-41cd-b28e-6ad145946c14`)
- Service: `traccar` (id `baaea731-6af9-4010-b066-36981e5c2714`)
- Region: `us-east4-eqdc4a`
- Timezone: `Europe/Bucharest`
