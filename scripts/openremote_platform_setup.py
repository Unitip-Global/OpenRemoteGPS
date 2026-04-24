#!/usr/bin/env python3
"""
OpenRemote platform setup for the Unitip Global fleet tracker.

Idempotent (safe to re-run): picks up the existing Fleet GroupAsset,
existing TrackerAssets, and existing Rulesets, creating only what's
missing.

Usage:
    python scripts/openremote_platform_setup.py

Prerequisites:
    pip install requests
    Env vars:
        OR_BASE_URL       (default: https://gps.unitip.global)
        OR_REALM          (default: unitip)
        OR_ADMIN_USER     (default: admin)
        OR_ADMIN_PASSWORD (required — the unitip realm admin password)
        TRACCAR_BASE_URL  (default: https://traccar-openremotegps.up.railway.app)
        TRACCAR_USER      (default: admin@unitip.ro)
        TRACCAR_PASSWORD  (required)

What it does (see `main()` for ordering):
    1. Ensures the 'Fleet Unitip Global' GroupAsset exists at realm root.
    2. For every device in Traccar, ensures a TrackerAsset child of Fleet
       exists in the realm, keyed by traccarDeviceId.
    3. Ensures ten baseline realm rulesets exist (all created disabled —
       enable from the Rules UI after reviewing thresholds).
"""

import json
import os
import sys
import time
from typing import Any

import requests

OR_BASE_URL = os.environ.get("OR_BASE_URL", "https://gps.unitip.global").rstrip("/")
OR_REALM = os.environ.get("OR_REALM", "unitip")
OR_ADMIN_USER = os.environ.get("OR_ADMIN_USER", "admin")
OR_ADMIN_PASSWORD = os.environ.get("OR_ADMIN_PASSWORD")

TRACCAR_BASE_URL = os.environ.get(
    "TRACCAR_BASE_URL", "https://traccar-openremotegps.up.railway.app"
).rstrip("/")
TRACCAR_USER = os.environ.get("TRACCAR_USER", "admin@unitip.ro")
TRACCAR_PASSWORD = os.environ.get("TRACCAR_PASSWORD")

FLEET_NAME = "Fleet Unitip Global"
BUCHAREST = {"type": "Point", "coordinates": [26.1025, 44.4268]}


def die(msg: str) -> None:
    print(f"FATAL: {msg}", file=sys.stderr)
    sys.exit(1)


def or_token() -> str:
    r = requests.post(
        f"{OR_BASE_URL}/auth/realms/{OR_REALM}/protocol/openid-connect/token",
        data={
            "grant_type": "password",
            "client_id": "openremote",
            "username": OR_ADMIN_USER,
            "password": OR_ADMIN_PASSWORD,
        },
        timeout=30,
    )
    if r.status_code != 200:
        die(f"OpenRemote auth failed: {r.status_code} {r.text[:200]}")
    return r.json()["access_token"]


def or_headers(tok: str) -> dict:
    return {
        "Authorization": f"Bearer {tok}",
        "Accept": "application/json",
        "Content-Type": "application/json",
    }


def query_assets(tok: str, **filters: Any) -> list:
    r = requests.post(
        f"{OR_BASE_URL}/api/{OR_REALM}/asset/query",
        headers=or_headers(tok),
        json=filters or {},
        timeout=60,
    )
    r.raise_for_status()
    return r.json()


def ensure_fleet(tok: str) -> str:
    existing = query_assets(tok, names=[FLEET_NAME])
    for a in existing:
        if a.get("type") == "GroupAsset" and a.get("name") == FLEET_NAME:
            print(f"[fleet] already exists id={a['id']}")
            return a["id"]

    payload = {
        "name": FLEET_NAME,
        "type": "GroupAsset",
        "realm": OR_REALM,
        "attributes": {
            "childAssetType": {"name": "childAssetType", "type": "assetType", "value": "ThingAsset"},
            "location": {"name": "location", "type": "GEO_JSONPoint", "value": BUCHAREST},
            "notes": {
                "name": "notes",
                "type": "text",
                "value": "Flota Unitip Global. Vehicule sincronizate automat din Traccar.",
            },
        },
    }
    r = requests.post(
        f"{OR_BASE_URL}/api/{OR_REALM}/asset",
        headers=or_headers(tok),
        json=payload,
        timeout=30,
    )
    if r.status_code != 200:
        die(f"Could not create Fleet asset: {r.status_code} {r.text[:200]}")
    fleet_id = r.json()["id"]
    print(f"[fleet] created id={fleet_id}")
    return fleet_id


def list_traccar_devices() -> tuple[list, dict]:
    auth = (TRACCAR_USER, TRACCAR_PASSWORD)
    r = requests.get(f"{TRACCAR_BASE_URL}/api/devices", auth=auth, timeout=30)
    r.raise_for_status()
    devices = r.json()
    r = requests.get(f"{TRACCAR_BASE_URL}/api/positions", auth=auth, timeout=30)
    r.raise_for_status()
    positions_by_device = {p["deviceId"]: p for p in r.json()}
    return devices, positions_by_device


def seed_tracker_assets(tok: str, fleet_id: str) -> None:
    devices, positions = list_traccar_devices()
    print(f"[traccar] {len(devices)} devices, {len(positions)} live positions")

    existing = query_assets(tok, types=["TrackerAsset"])
    by_tid = {}
    for a in existing:
        tid = a.get("attributes", {}).get("traccarDeviceId", {}).get("value")
        if tid is not None:
            by_tid[int(tid)] = a
    print(f"[or] {len(existing)} TrackerAssets already in realm {OR_REALM}")

    created = moved = 0
    failures = []

    for idx, d in enumerate(devices):
        if idx and idx % 100 == 0:
            tok = or_token()
            print(f"  refreshed token at device #{idx}")

        tid = d["id"]
        name = d["name"]
        unique_id = d.get("uniqueId") or ""
        pos = positions.get(tid)

        # Already exists → nudge parent to Fleet if needed
        if tid in by_tid:
            a = by_tid[tid]
            if a.get("parentId") == fleet_id:
                continue
            # PUT to move — 403 can happen for live-updating assets; fall back to delete+recreate
            a["parentId"] = fleet_id
            r = requests.put(
                f"{OR_BASE_URL}/api/{OR_REALM}/asset/{a['id']}",
                headers=or_headers(tok),
                json=a,
                timeout=30,
            )
            if r.status_code in (200, 204):
                moved += 1
                continue
            if r.status_code == 403:
                requests.delete(
                    f"{OR_BASE_URL}/api/{OR_REALM}/asset?assetId={a['id']}",
                    headers=or_headers(tok),
                    timeout=30,
                )
                # fall through to create path
            else:
                failures.append((name, "move", r.status_code, r.text[:120]))
                continue

        # Create fresh TrackerAsset under Fleet
        attrs = {
            "location": {
                "name": "location",
                "type": "GEO_JSONPoint",
                "value": (
                    {"type": "Point", "coordinates": [pos["longitude"], pos["latitude"]]}
                    if pos
                    else BUCHAREST
                ),
            },
            "traccarDeviceId": {
                "name": "traccarDeviceId",
                "type": "positiveInteger",
                "value": tid,
            },
        }
        if unique_id:
            attrs["serialNumber"] = {"name": "serialNumber", "type": "text", "value": unique_id}
        if pos and "speed" in pos:
            attrs["speed"] = {"name": "speed", "type": "number", "value": pos["speed"]}

        payload = {
            "name": name,
            "type": "TrackerAsset",
            "realm": OR_REALM,
            "parentId": fleet_id,
            "attributes": attrs,
        }
        r = requests.post(
            f"{OR_BASE_URL}/api/{OR_REALM}/asset",
            headers=or_headers(tok),
            json=payload,
            timeout=30,
        )
        if r.status_code == 200:
            created += 1
        else:
            failures.append((name, "create", r.status_code, r.text[:120]))

    print(f"[seed] created={created} moved={moved} failed={len(failures)}")
    for f in failures[:10]:
        print(f"  - {f}")
    if len(failures) > 10:
        print(f"  ... {len(failures) - 10} more")


# ---- rules ----

def _attr_number_rule(attr: str, op: str, threshold: float, note: str, name: str, desc: str) -> dict:
    return {
        "rules": [
            {
                "name": name,
                "description": desc,
                "priority": 10,
                "when": {
                    "asset": {
                        "types": ["TrackerAsset"],
                        "attributes": {
                            "items": [
                                {
                                    "name": {"predicateType": "string", "match": "EXACT", "value": attr},
                                    "value": {
                                        "predicateType": "number",
                                        "operator": op,
                                        "value": threshold,
                                    },
                                }
                            ]
                        },
                    }
                },
                "then": [
                    {
                        "target": {"useAssetsFromWhen": True},
                        "action": "write-attribute",
                        "attributeName": "notes",
                        "value": note,
                    }
                ],
            }
        ]
    }


def _attr_bool_rule(attr: str, value: bool, note: str, name: str, desc: str) -> dict:
    return {
        "rules": [
            {
                "name": name,
                "description": desc,
                "priority": 10,
                "when": {
                    "asset": {
                        "types": ["TrackerAsset"],
                        "attributes": {
                            "items": [
                                {
                                    "name": {"predicateType": "string", "match": "EXACT", "value": attr},
                                    "value": {"predicateType": "boolean", "value": value},
                                }
                            ]
                        },
                    }
                },
                "then": [
                    {
                        "target": {"useAssetsFromWhen": True},
                        "action": "write-attribute",
                        "attributeName": "notes",
                        "value": note,
                    }
                ],
            }
        ]
    }


RULES_SPEC = [
    # name, body-builder, description
    ("01 - Supraviteza critica >130kmh",
     lambda: _attr_number_rule("speed", "GREATER_THAN", 130, "⚠ Supraviteza critica: > 130 km/h",
                                "Supraviteza critica", "Alerta cand vehiculul depaseste 130 km/h"),
     "Critical overspeed threshold"),
    ("02 - Supraviteza >90kmh",
     lambda: _attr_number_rule("speed", "GREATER_THAN", 90, "⚠ Supraviteza: > 90 km/h",
                                "Supraviteza 90", "Alerta la 90 km/h (drum national)"),
     "National road limit"),
    ("03 - Viteza urbana >50kmh",
     lambda: _attr_number_rule("speed", "GREATER_THAN", 50, "⚠ Viteza mare in zona urbana: > 50 km/h",
                                "Viteza urbana", "Alerta > 50 km/h"),
     "Urban speed limit"),
    ("04 - Baterie critica <10%",
     lambda: _attr_number_rule("batteryLevel", "LESS_THAN", 10,
                                "⚠ Baterie critica: < 10% - device-ul se va opri curand",
                                "Baterie critica", "Alerta < 10% baterie"),
     "Critical battery"),
    ("05 - Baterie scazuta <20%",
     lambda: _attr_number_rule("batteryLevel", "LESS_THAN", 20, "Baterie scazuta: < 20%",
                                "Baterie scazuta", "Alerta < 20% baterie"),
     "Low battery"),
    ("06 - Combustibil scazut <15%",
     lambda: _attr_number_rule("fuelLevel", "LESS_THAN", 15,
                                "⚠ Combustibil scazut: < 15% - alimenteaza",
                                "Combustibil scazut", "Alerta < 15% combustibil"),
     "Low fuel"),
    ("07 - Altitudine extrema >2500m",
     lambda: _attr_number_rule("altitude", "GREATER_THAN", 2500, "⚠ Altitudine mare: vehicul peste 2500m",
                                "Altitudine extrema", "Flag pentru vehicule la altitudine mare"),
     "Extreme altitude"),
    ("08 - Contact pornit",
     lambda: _attr_bool_rule("ignition", True, "⚡ Contact pornit",
                              "Ignition on", "Eveniment pornire contact"),
     "Ignition on"),
    ("09 - Contact oprit",
     lambda: _attr_bool_rule("ignition", False, "⏹ Contact oprit",
                              "Ignition off", "Eveniment oprire contact"),
     "Ignition off"),
    ("10 - Raport saptamanal kilometraj",
     lambda: {
         "rules": [
             {
                 "name": "Raport saptamanal",
                 "description": "Luni 08:00 - raport saptamanal",
                 "priority": 20,
                 "when": {"cron": "0 0 8 ? * MON"},
                 "then": [
                     {
                         "target": {"assets": {"types": ["TrackerAsset"]}},
                         "action": "write-attribute",
                         "attributeName": "notes",
                         "value": "Raport saptamanal generat",
                     }
                 ],
             }
         ]
     },
     "Weekly mileage report"),
]


def ensure_rules(tok: str) -> None:
    r = requests.get(
        f"{OR_BASE_URL}/api/{OR_REALM}/rules/realm/for/{OR_REALM}",
        headers=or_headers(tok),
        timeout=30,
    )
    r.raise_for_status()
    existing_by_name = {rs["name"]: rs for rs in r.json()}

    for name, builder, _desc in RULES_SPEC:
        if name in existing_by_name:
            print(f"[rule] {name}: exists (id={existing_by_name[name]['id']})")
            continue

        body = builder()
        payload = {
            "type": "realm",
            "name": name,
            "lang": "JSON",
            "rules": json.dumps(body, ensure_ascii=False),
            "enabled": False,
            "realm": OR_REALM,
        }
        r = requests.post(
            f"{OR_BASE_URL}/api/{OR_REALM}/rules/realm",
            headers=or_headers(tok),
            json=payload,
            timeout=30,
        )
        ok = "OK" if r.status_code == 200 else f"FAIL {r.status_code} {r.text[:100]}"
        print(f"[rule] {name}: {ok}")


def main() -> None:
    if not OR_ADMIN_PASSWORD:
        die("OR_ADMIN_PASSWORD env var is required")
    if not TRACCAR_PASSWORD:
        die("TRACCAR_PASSWORD env var is required")

    tok = or_token()
    fleet_id = ensure_fleet(tok)
    seed_tracker_assets(tok, fleet_id)
    tok = or_token()  # refresh before long chain
    ensure_rules(tok)

    print("\nDone. Enable rules from the Manager UI after validating thresholds.")


if __name__ == "__main__":
    main()
