# scripts/

Operational scripts for the OpenRemote GPS platform. Runtime-only — NOT tied
to any deployed service.

## `openremote_platform_setup.py`

Idempotent seed + rules setup for the `unitip` realm on the running
OpenRemote stack. Safe to re-run; picks up existing assets and rulesets
and fills in only what is missing.

**What it ensures exists:**

- `Fleet Unitip Global` GroupAsset at the root of the `unitip` realm.
- One `TrackerAsset` per device in Traccar (keyed by `traccarDeviceId`),
  parented to Fleet.
- Ten baseline realm rulesets (speed, battery, fuel, ignition, weekly
  report) — all created **disabled**; enable from the Rules UI after
  reviewing the thresholds for your fleet.
- A `unitip` entry in the manager's MapConfig so the realm's map opens
  centered on Romania (Bucharest), zoom 7, bounded to the country.

**Run:**

```bash
pip install requests

export OR_ADMIN_PASSWORD='<unitip realm admin password from Railway>'
export TRACCAR_PASSWORD='<traccar admin password from Railway>'

python scripts/openremote_platform_setup.py
```

See the script's top-level docstring for all supported env vars.

**Typical runtime:** 30-90 seconds depending on how many Traccar devices
need new OpenRemote assets.

## When to re-run

- After importing new devices into Traccar — re-run to mirror them into
  OpenRemote.
- After a realm rebuild or migration.
- After the baseline rules drift (someone edited/deleted in the UI and
  you want the defaults back).

## What it does NOT do

- Does not touch users, roles, or OAuth clients — those live in Keycloak
  and are provisioned separately (see `docs/TOPOLOGY.md` § Realms).
- Does not enable rules. Enabling is a deliberate step you do from the
  Manager UI.
- Does not configure map center. That is a UI-only setting per realm in
  the current OpenRemote version.
- Does not delete obsolete assets. If a device was removed from Traccar,
  its OpenRemote TrackerAsset stays until you delete it manually.
