# traccar-transformer — source

Go service that bridges the JSON shape gap between Traccar's position forwarder
and the existing `gps-adapter`.

## Where the source lives

**In this repo**, at [`./src/`](./src). Deployed via `railway up services/traccar-transformer/src/`
after linking the Railway service. Same pattern as the nginx edge.

```
services/traccar-transformer/
├── SOURCE.md         # this file
├── vars.env          # env var reference
└── src/
    ├── main.go       # ~140 lines, stdlib only
    ├── go.mod
    ├── Dockerfile    # multi-stage: golang:1.22-alpine → alpine:3.19
    └── railway.toml  # DOCKERFILE builder, /health healthcheck
```

## Role

Traccar's `forward.type=json` posts positions as nested JSON:

```json
{
  "device":   { "id": 257, "name": "B 154 UIP", "uniqueId": "354017112585365" },
  "position": { "deviceId": 257, "latitude": 44.50929, "longitude": 26.14591,
                "speed": 0, "valid": true,
                "attributes": { "batteryLevel": 95, ... } },
  "event":    { ... }
}
```

The deployed `gps-adapter` binary parses only the **flat** shape:

```json
{ "deviceId": 257, "uniqueId": "354017112585365",
  "latitude": 44.50929, "longitude": 26.14591, "speed": 0,
  "valid": true, "batteryLevel": 95 }
```

This service receives nested on `POST /webhook`, reshapes to flat, posts to
`ADAPTER_URL` (default `http://gps-adapter.railway.internal:8080/gps/position`),
and returns the status to Traccar.

## Behaviour

- Skips positions where `valid=false` unless `FORWARD_INVALID=true`.
  A Teltonika without GPS fix spams the last known coordinates with
  `valid=false` and `sat=0`; those would freeze the asset on stale position.
- Extracts `batteryLevel` from Traccar's `position.attributes` map
  (Traccar moves per-protocol extras into `attributes`, not top-level).
- One log line per forwarded position:
  `fwd deviceId=257 name="B 154 UIP" lat=44.50929 lon=26.14591 valid=true adapter=204`.
- Internal-only service. No public Railway domain required.

## Config

See [vars.env](./vars.env). Three knobs:

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | Listen port |
| `ADAPTER_URL` | `http://gps-adapter.railway.internal:8080/gps/position` | Where to POST the flat payload |
| `FORWARD_INVALID` | `false` | Set `true` to forward `valid=false` positions too |

## Deployment signature

- Build: `DOCKERFILE`, path `Dockerfile` (in `src/`)
- Deploy: healthcheck `/health` (30s timeout), 1 replica, `ON_FAILURE` restart x10
- Runtime: ~7 MB Go binary on Alpine 3.19

## Things to verify

- Is `traccar` posting to `http://traccar-transformer.railway.internal:8080/webhook`?
  Check the `forward.url` entry in the live `traccar.xml` (set via Traccar service's
  startCommand override).
- Does `gps-adapter` log `[OK] device=<N> asset=<id>` within ~30s of a Teltonika
  sending a valid position? If not, check this service's logs for `fwd` lines.
