# gps-adapter — source

Go service that bridges Traccar positions into OpenRemote `TrackerAsset`
instances in the `unitip` realm.

## Where the source lives

**In this repo**, at [`./src/`](./src). Deployed via
`railway up services/gps-adapter/src --path-as-root` after linking the
GPS-Adapter service.

```
services/gps-adapter/
├── SOURCE.md         # this file
├── vars.env          # env var reference
└── src/
    ├── main.go       # ~350 lines, stdlib only
    ├── go.mod
    ├── Dockerfile    # multi-stage golang:1.22-alpine → alpine:3.19
    └── railway.toml  # DOCKERFILE builder, /health healthcheck
```

## Role

Accepts position pushes at `POST /gps/position` in **either** shape the
Traccar side might send:

- **Flat** (what the legacy mystery binary accepted):
  `{ "deviceId": 257, "latitude": ..., "longitude": ..., "speed": ...,
     "valid": true, "batteryLevel": 95 }`

- **Traccar nested** (what Traccar's built-in `forward.type=json` emits):
  `{ "device": { ... }, "position": { ... }, "event": { ... } }`

For every valid position:

1. Looks up the Traccar device name in an in-process cache (queries
   `GET /api/devices/{id}` on miss).
2. Finds the matching OpenRemote asset via cached `traccarDeviceId`, or
   queries `POST /api/{realm}/asset/query` on miss.
3. Creates a new `TrackerAsset` under `FLEET_PARENT_ID` if no asset
   exists, or updates the existing asset's attributes in place.

Attributes pushed to OpenRemote on each update (when available in the
payload): `location`, `altitude`, `speed`, `heading`, `protocol`,
`batteryLevel`, `fuelLevel`, `odometer`, `ignition`. Missing values are
skipped — no zero-writes polluting charts.

OAuth2 password grant against Keycloak (realm admin), one bearer token
cached in-process and refreshed ~10s before expiry.

## Why this replaced the previous binary

The original `gps-adapter` was deployed from an unknown external source
and only accepted the flat shape. That forced a separate
`traccar-transformer` service purely to reshape Traccar's nested JSON to
flat before POSTing to the adapter. By accepting both shapes directly,
the transformer becomes optional — keep it in front if you want a
single knob for payload filtering, or drop it and have Traccar forward
straight to `http://gps-adapter.railway.internal:8080/gps/position`.

## Deployment signature

- Build: `DOCKERFILE`, path `Dockerfile` (in `src/`)
- Deploy: healthcheck `/health` (30s timeout), 1 replica, `ON_FAILURE` restart x10
- Runtime: ~8 MB Go binary on Alpine 3.19, ~20 MB RAM idle

## Things to verify

- `FLEET_PARENT_ID` env var on the GPS-Adapter Railway service matches
  the id of the `Fleet Unitip Global` GroupAsset (currently
  `49P7wc3KE2ZPfk7JsZGvT9`). Newly-created assets are parented to it.
- After a Traccar position forward, log line:
  `[OK] device=257 asset=... lat=... lon=... speed=...km/h`
