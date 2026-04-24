# Secrets — rotation checklist

All secrets currently deployed in the Railway `OpenRemoteGPS` env are **weak and/or placeholder values**. This document is the tracking list for rotation. **Real values live only in Railway** — this repo stores names and placeholders.

## Secrets that must be rotated

| Railway var | Current value (as of 2026-04-24) | Severity | Used by | Rotation action |
|---|---|---|---|---|
| `POSTGRES_PASSWORD` / `OR_DB_PASSWORD` / `KC_DB_PASSWORD` / `TRACCAR_DB_PASSWORD` | `o_parola_puternica` (literal "a strong password" — a placeholder, not one) | **Critical** | timescaledb, manager, keycloak, traccar, backup | 1) rotate in `timescaledb` service vars; 2) update in all four consumers; 3) restart manager + keycloak + traccar + backup |
| `OR_ADMIN_PASSWORD` (master realm) | `secret` | **Critical** | manager, nginx | Master realm superadmin. Rotate; update any admin tooling that logs in as OpenRemote master admin |
| unitip realm admin password | `__CHANGE_ME__` (24-char random generated 2026-04-24, stored in Railway `OPENREMOTE_PASSWORD` on GPS-Adapter service) | **Critical** | gps-adapter | Used by adapter to authenticate against Keycloak realm `unitip`. Rotate together with the Keycloak user in `unitip` realm |
| `OPENREMOTE_PASSWORD` (gps-adapter) | mirrors unitip realm admin password | **Critical** | gps-adapter | Must match the Keycloak user's password in realm `unitip` |
| `KEYCLOAK_ADMIN_PASSWORD` | `Admin` | **Critical** | keycloak | Rotate |
| `TRACCAR_PASSWORD` / `ADMIN_PASSWORD` (traccar) | `Unitip123!` | High | traccar, gps-adapter | Rotate; update adapter var |
| `ADAPTER_SECRET_KEY` | `6061d01001b71ed152ad24b6ff76af5e54ff0ba7fc2ef686f63e704326af4dfc` | Medium | nginx, gps-adapter | Already 32-byte hex — not weak, but shared. Rotate on suspicion only |

## Shared-credential risk

The same Postgres user/password (`openremote` / `o_parola_puternica`) is used by **manager, keycloak, and traccar** against the same database. Anyone who reads one service's env reads the DB. Long-term fix: give each service its own DB user with only the privileges it needs.

## Behaviour flags worth reviewing (not secrets, but risky)

| Var | Current | Recommendation |
|---|---|---|
| `OR_SETUP_RUN_ON_RESTART` | `true` | Flip to `false` once the OpenRemote realm/rules are stable. `true` re-runs setup on every container restart and can re-seed demo data. |
| `OR_KEYCLOAK_SECURE` | `false` | OK — TLS is terminated at the Railway edge; internal traffic is plaintext over the private network. Do not expose Keycloak directly. |
| `OR_SSL_PORT` | `-1` | OK — same reason. |

## Rotation playbook

1. Generate new value (e.g. `openssl rand -base64 32` for passwords, `openssl rand -hex 32` for `ADAPTER_SECRET_KEY`).
2. Set it on Railway: `railway variables set KEY=VALUE --service <name> --environment OpenRemoteGPS`.
3. For a password used by multiple services, update **all** of them before restarting any — otherwise services fail to connect mid-rotation.
4. Redeploy affected services (Railway dashboard → "Redeploy" or `railway up`).
5. Update any ERP-side reference (for example, Core ERP's `OpenRemoteProvider` may hold a client secret in its own env).
6. Smoke-test: login to `https://gps.unitip.global`, check traccar UI, check `gps-adapter` `/health`, confirm Core ERP fleet module still ingests positions.

## What this repo stores vs what Railway stores

- **This repo:** variable **names**, non-secret values (timezone, hostnames, ports, service URLs), and `__CHANGE_ME__` placeholders.
- **Railway:** actual secret **values**. Treat Railway as the vault.
- **Never:** commit real passwords, DB URIs with creds, `ADAPTER_SECRET_KEY`, etc., to this repo.
