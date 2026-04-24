# Secrets — rotation checklist

Single-service stack after the 2026-04-24 refactor. The only remaining
service on Railway is `traccar`. All prior OpenRemote / Keycloak /
Postgres / adapter credentials were retired when their services were
deleted.

## Secrets in use today

| Railway var | Current value | Severity | Used by | Rotation action |
|---|---|---|---|---|
| `ADMIN_PASSWORD` (traccar) | `Unitip123!` | **Critical** | traccar | Rotate via Traccar web UI (Account → Change password) after the first successful login, then replace the env var value with the new password so `ADMIN_EMAIL` auth still seeds identically on a fresh container. |

That's the whole list. If you spot another `__CHANGE_ME__` anywhere in
this repo, it's stale documentation — file an issue or delete the line.

## What was removed (historical reference)

Rotating these is a no-op now — the consumers are gone:

- `POSTGRES_PASSWORD` / `OR_DB_PASSWORD` / `KC_DB_PASSWORD` /
  `TRACCAR_DB_PASSWORD` (shared DB password, was `o_parola_puternica`)
- `OR_ADMIN_PASSWORD` (was `secret`, used by OpenRemote manager)
- `OPENREMOTE_PASSWORD` (gps-adapter OAuth2 password grant)
- `KEYCLOAK_ADMIN_PASSWORD` (was `Admin`)
- `ADAPTER_SECRET_KEY` (nginx ↔ adapter shared secret)
- `unitip` Keycloak realm admin password (24-char random, generated 2026-04-24)

If any of those values were ever reused outside this stack (Core ERP,
personal accounts, etc.), rotate them in the other system — the fact that
they're no longer in use here doesn't retroactively delete them from
Core ERP's env.

## Non-secret flags worth reviewing

| Var | Current | Recommendation |
|---|---|---|
| `TZ` | `Europe/Bucharest` | OK — log timestamps readable, backup script still fires at 02:00 UTC regardless. |

Traccar's own config (database URL, forward settings, etc.) is not an env
var — it's baked into the `startCommand` override on the Railway service.
If you rotate those, update the canonical copy in
[services/traccar/SOURCE.md](../services/traccar/SOURCE.md) so the repo
stays the record of truth.

## Rotation playbook

For `ADMIN_PASSWORD`:

1. Log in to https://gps.unitip.global as `admin@unitip.ro` with the
   current password.
2. Account menu → Change password → set a new strong value.
3. Copy the new password into Railway: `railway variable set
   ADMIN_PASSWORD='<new>' --service traccar --environment OpenRemoteGPS`.
   (This matters for disaster recovery — if the H2 volume is ever wiped,
   Traccar re-seeds from this env var.)
4. Update any ERP-side / monitoring clients that log in with this
   credential.

## What this repo stores vs what Railway stores

- **This repo:** variable **names**, non-secret values (timezone, ports,
  hostnames), and `__CHANGE_ME__` placeholders.
- **Railway:** actual secret **values**. Treat Railway as the vault.
- **Never:** commit real passwords, DB URIs with creds, etc., to this
  repo.
