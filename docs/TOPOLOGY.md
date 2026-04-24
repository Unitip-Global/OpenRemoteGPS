# Topology

Public entry: **https://gps.unitip.global** → `traccar` service on Railway
(HTTP :8082, web UI).

Device ingest: **shortline.proxy.rlwy.net:57840** → Railway TCP proxy →
`traccar` :5027 (Teltonika Codec 8/8E binary).

## Data flow (post 2026-04-24 refactor)

```
 GPS devices           ┌──────────────────────────────────────┐
  (Teltonika)          │  Railway project: ERP                │
   │                   │  Environment: OpenRemoteGPS          │
   │  TCP :5027        │                                      │
   │  (via Railway     │    ┌──────────────────────────┐      │
   │   TCP proxy)      │    │        traccar           │      │
   ▼                   │    │                          │      │
   ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼──►│  ┌────────────────────┐  │      │
                       │    │  │  tracker-server    │  │      │
 Browser / admin       │    │  │  JVM, port 8082    │  │      │
 (Traccar web UI)      │    │  │  embedded H2       │  │      │
   │                   │    │  │  /opt/.../data     │  │      │
   │  HTTPS :443       │    │  └────────────────────┘  │      │
   ▼                   │    │                          │      │
   ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┼──►│  ┌────────────────────┐  │      │
                       │    │  │  backup loop (bg)  │  │      │
                       │    │  │  daily 02:00 UTC   │  │      │
                       │    │  │  → SQL gz          │  │      │
                       │    │  │  → retention 14d   │  │      │
                       │    │  └────────────────────┘  │      │
                       │    │                          │      │
                       │    │  Volume: traccar-volume  │      │
                       │    │  at /opt/traccar/data    │      │
                       │    └──────────────────────────┘      │
                       └──────────────────────────────────────┘

                          (no other services)

Future: adapters will sit either as sidecar services inside this env, or
external consumers of Traccar's REST API / WebSocket feed at
https://gps.unitip.global/api/*.
```

## Ports

| Service | Internal | External | Purpose |
|---|---|---|---|
| traccar | 8082 (HTTP), 5027 (TCP Teltonika) | `gps.unitip.global`, `shortline.proxy.rlwy.net:57840` | Web UI + REST + device ingest |

## Volumes

| Volume | Mounted on | Mount path | Use |
|---|---|---|---|
| `traccar-volume` | traccar | `/opt/traccar/data` | H2 DB (`database.mv.db`) + nightly backups (`backup/traccar_*.sql.gz`) + Traccar logs |

No other volumes. The Railway volumes that previously held OpenRemote
deployment data, manager storage, and the OpenRemote DB dumps
(`openremotegps-backup-volume`, `timescaledb-volume-l4FW`,
`manager-deployment`, `manager-storage`) were deleted along with their
services.

## Private network

Only one service — no internal communication. Adapters added later will
reach Traccar's HTTP API at `traccar.railway.internal:8082` (basic auth
with admin credentials) or via WebSocket `ws://traccar.railway.internal:8082/api/socket`.

## Backups

- **Where**: `/opt/traccar/data/backup/traccar_YYYYMMDD_HHMMSS.sql.gz` inside
  the `traccar-volume`.
- **How**: `org.h2.tools.Script` SQL dump via JDBC against the live H2 DB
  (uses `AUTO_SERVER=TRUE` so backup can read while Traccar holds the
  primary lock).
- **When**: daily at 02:00 UTC, triggered by a `while sleep` loop inside
  the service's `startCommand`. No separate cron service.
- **Retention**: 14 days, enforced by `find -mtime +14 -delete` after each
  successful dump.
- **Download**: `railway ssh --service traccar "cat /opt/traccar/data/backup/traccar_YYYYMMDD_HHMMSS.sql.gz"`
  → stdout-to-file on your laptop. See
  [services/traccar/SOURCE.md](../services/traccar/SOURCE.md) for full
  restore instructions.

## External dependencies

None. The adapter and ERP integration layer will be built later as
separate services.
