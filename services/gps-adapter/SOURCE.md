# GPS-Adapter — source code

The Go source for this service is **not** in this repo (this repo captures deployment config only).

## Where it lives today

Unknown at the time this snapshot was taken. On Railway the service builds from a `Dockerfile` with the `DOCKERFILE` builder (nixpacks detected `go`), so the source is in whatever repo Railway is pulling from.

**TODO (denis):** fill this in with the actual GitHub repo URL and branch, e.g.:

```
Repo:   github.com/<org>/<repo>
Branch: main
Path:   /   (or subpath if monorepo)
```

## Deployment signature (from last successful Railway build)

- Build: `DOCKERFILE`, path `Dockerfile`
- Nixpacks providers: `go`
- Deploy: healthcheck `/health` (30s timeout), 1 replica, `ON_FAILURE` restart x10
- Last SUCCESS deploy: `5d8ad8a6-9fc1-4f17-a119-e5960c8660a0` at 2026-03-30T13:20:20Z
- Digest: `sha256:cb66124eefa48a5ffcef2761b3e3e3cc57ca3987efc9f05c383a865064d7c7a6`

## Role

HTTP bridge service (`:8080`) between Traccar (GPS device ingest) and OpenRemote (asset model):

- Authenticates against OpenRemote via Keycloak OAuth2
- Authenticates against Traccar with basic auth
- Translates Traccar device positions into OpenRemote asset location updates
- Exposes `/health` for Railway healthcheck

Shares `ADAPTER_SECRET_KEY` with the `openremote-nginx` edge service for authenticated webhook / push calls.
