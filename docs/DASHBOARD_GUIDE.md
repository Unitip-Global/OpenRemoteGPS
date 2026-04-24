# Dashboard guide — Unitip Global fleet

Cum construiești o vedere de "platformă GPS" pentru dispeceri și management în
OpenRemote, fără să atingi codul. 10–15 minute de click-uri în UI.

> Toate setările sunt per-realm. Loghează-te cu userul realm-admin
> (`admin` / parola din Railway `OPENREMOTE_PASSWORD` pe serviciul
> GPS-Adapter) la `https://gps.unitip.global/manager/?realm=unitip`.

## 1. Setări de realm — logo, nume, accent color

Pentru ca realm-ul să arate brand-uit, nu "OpenRemote generic":

1. Meniul din stânga sus (hamburger) → **Realms**.
2. Click pe `unitip` → tab **General**.
3. Setează:
   - **Display name**: `Unitip Global` (deja setat)
   - **Accent color**: hex-ul companiei (ex. `#E30613` pentru roșu Unitip). Se aplică la butoane, link-uri, tree selection.
   - **Logo URL**: upload la CDN (orice, ex. un S3 public) și pune URL-ul aici. Dimensiune recomandată ~200x60px transparent PNG. Apare sus stânga.
   - **Logo notification URL**: versiune mai mică pentru notificări (ex. 48x48 favicon).
4. **Save**.

## 2. Dashboard-uri — "Flotă live" și "Alarme"

Dashboard-urile în OpenRemote sunt widget-uri drag-and-drop. Fiecare e "privat"
(vezi doar tu), "shared" (toți userii din realm) sau "public" (fără login).

### Dashboard 1: Flotă live (shared)

Scopul: dispecerul deschide, vede toate vehiculele pe hartă, click pe unul = detalii.

1. Meniul stânga → **Insights**.
2. Buton **+ Create dashboard** → nume `Flota live` → Access **SHARED**.
3. Adaugă widget **Map** (drag-in):
   - **Asset selection**: click → filtrează la `Fleet Unitip Global` (alege tree → Fleet).
   - **Type**: `TrackerAsset`.
   - **Attribute**: `location`.
   - **Default view**: click "Apply current map" ca să salveze centrarea pe România + zoom.
   - **Show asset name on marker**: ON.
   - Save.
4. Adaugă widget **Asset list** alături:
   - Same asset selection (Fleet → TrackerAsset).
   - Attributes to show: `name`, `location` (last update), `speed`, `ignition`, `batteryLevel`.
   - Sort by `location.timestamp` DESC (cele mai recente sus).
5. Adaugă widget **KPI card** pentru "Total vehicule":
   - Data: count of TrackerAsset in Fleet.
   - Title: "Total flotă".
6. **Save dashboard**.

### Dashboard 2: Alarme (shared)

Scopul: vedere rapidă a vehiculelor în stare anormală.

1. **+ Create dashboard** → nume `Alarme` → Access **SHARED**.
2. Widget **Asset list** — filter pe `speed > 90`:
   - Asset type: TrackerAsset
   - Attribute predicate: `speed > 90`
   - Title: "Vehicule cu supraviteză (>90 km/h)"
3. Widget **Asset list** — filter pe `batteryLevel < 20`:
   - Attribute predicate: `batteryLevel < 20`
   - Title: "Baterie scăzută"
4. Widget **Asset list** — filter pe `ignition = true` AND `speed < 2`:
   - Predicates: `ignition == true`, `speed < 2`
   - Title: "Idle cu motorul pornit"
5. Widget **Chart** — count trend 24h:
   - Tip: line chart
   - Datapoint: speed peste toate TrackerAsset, aggregate MAX, bucket 15min
   - Title: "Viteza maximă în flotă (24h)"
6. **Save dashboard**.

### Dashboard 3 (opțional): Vehicul individual (privat, folosit ca șablon)

Pentru click-through dintr-un vehicul specific.

1. **+ Create dashboard** → nume `Vehicul — template` → Access **PRIVATE**.
2. Widget **Map** single-asset (alegi un TrackerAsset anume, ex. B 154 UIP).
3. Widget **Chart** line: `speed`, last 24h.
4. Widget **Chart** line: `fuelLevel` + `batteryLevel` (două serii).
5. Widget **Chart** line: `odometer`, last 7 days (pentru kilometraj).
6. Widget **Gauge**: `fuelLevel` current (0-100%).
7. Widget **KPI**: "Last contact" = `location.timestamp`.
8. **Save**.

Refolosești acest dashboard pentru alt vehicul făcând click "duplicate" și
schimbând asset-ul selectat.

## 3. Enable rules (din Rules UI)

Regulile sunt create via seed script cu `enabled: false`. Se activează manual
după ce ești ok cu thresholdurile:

1. Meniul stânga → **Rules** → **Global / Realm rules** → tab `unitip`.
2. Vezi cele 10 rule-uri: "01 - Supraviteza critica >130kmh", etc.
3. Pentru fiecare, click → **Enabled** toggle → **Save**.
4. Verifică că tabul **Rule history / Triggered** începe să arate rule-uri fire-ate când vehiculele încep să trimită date corespunzătoare.

Regulile curente scriu la `notes` pe asset. Pentru a le transforma în
notificări reale (push mobile / email / webhook):

1. **Notifications** din meniul stânga → **+ New notification channel**
   (email SMTP sau push).
2. În fiecare rule → **Then action** → schimbă din `write-attribute: notes`
   în `notification` → alege channel-ul.

## 4. Permisiuni de user — roluri tipice de flotă

`admin` e superuser realm. Pentru flotă reală vei vrea măcar:

| Rol practic | Client roles OpenRemote | Ce pot face |
|---|---|---|
| **Dispatcher** | `read:assets`, `read:map`, `read:insights`, `read:alarms` | Vede hartă + dashboards, nu modifică |
| **Fleet manager** | `read:*`, `write:rules`, `write:insights` | Toate de mai sus + creează dashboards + activează/dezactivează rules |
| **Operator câmp** (opțional) | `read:assets`, `read:map` (cu restricție asset = propriul vehicul) | Vede doar mașina lui |

Se creează în **Users** → **+ Add user** → parola temporară → **Client roles** tab.

## 5. Icon pe vehicule pe hartă

Din setările asset-ului Fleet (sau per-TrackerAsset):

1. Click pe asset în tree.
2. Tab **General** → câmpul **Icon**.
3. Pune numele unui icon MapLibre supported (default: `marker`; recomandat:
   `truck` sau `car`).
4. Save.

OpenRemote folosește Maki icon set. Lista completă:
https://labs.mapbox.com/maki-icons/. Alternativ, upload SVG custom prin
**Sprite** la nivel de realm.

## 6. Istoric și traseu per vehicul

OpenRemote salvează automat toate atribute-event-urile ca datapoints.
Acces:

1. Click pe un TrackerAsset.
2. Tab **History**.
3. Alege atributul (ex. `location`, `speed`) → perioada (ultimele 24h / 7
   zile / custom).
4. Pentru `location` → se afișează polyline pe hartă (traseul vehiculului).
5. Pentru `speed` / `fuelLevel` → chart line.

Datapoints sunt stocate în Postgres (`asset_datapoint` tabel), cu agregare
automată după 30 zile. Config-ul rularii e `OR_DATA_POINT_MAX_AGE` pe manager.

## 7. Export + alerte pe email

- **Export CSV**: orice list din dashboards are buton **Export** → CSV.
- **Alerte email**: config `OR_EMAIL_HOST` pe manager în Railway (plus user,
  pass, from). Apoi creezi Notification channel de tip Email și îl legi de
  rule-urile dorite.

## 8. Map-ul e pe România — verificat

Realm-ul `unitip` are deja `center: [26.1025, 44.4268]` (București) și
`zoom: 7` setate prin `scripts/openremote_platform_setup.py`. Prima deschidere
a dashboard-ului cu widget Map → vezi România. Dacă nu, hard-refresh
(Ctrl+Shift+R) ca să invalideze cache-ul local.

---

## Senzori IoT adiționali conectați la Teltonika (BLE, 1-Wire, analog, CAN)

Dacă ai conectat la device senzori suplimentari (temperatură în remorcă, senzor
ușă, senzor debit, iButton șofer, CAN bus de camion, etc.), fluxul e în trei
pași.

### Pas 1 — activezi parametrul în Teltonika Configurator (pe device)

Conectezi device-ul prin USB la Configurator → tab **I/O** → caută AVL ID-ul
senzorului → **Enabled: On**, **Priority: Low** (sau High dacă vrei event
imediat). Save → Ctrl+S ca să scrii config pe device.

Fără acest pas, chiar dacă senzorul e conectat fizic, device-ul nu-l
transmite prin GPRS.

**Cele mai folosite AVL ID-uri pentru IoT adițional:**

| Senzor | AVL ID | Unitate / format |
|---|---|---|
| BLE temperature tag 1-4 (eg. Elsys ERS) | io25, io26, io27, io28 | °C × 10 |
| 1-Wire Dallas temperature 1-4 | io72, io73, io74, io75 | °C × 10 |
| Digital input DIN1 (door open, panic) | io1 | 0/1 |
| Digital input DIN2, DIN3 (PTO, siguranță) | io2, io3 | 0/1 |
| Analog input AIN1 (senzor fuel RS485 / nivel rezervor) | io9 | mV |
| Analog input AIN2 | io10 | mV |
| External supply voltage (tensiune baterie vehicul) | io66 | mV |
| Internal battery voltage (backup battery intern) | io67 | mV |
| GSM signal (calitate semnal) | io21 | 0-5 |
| Movement (accelerometru) | io240 | 0/1 |
| Ignition (detect pe DIN1 sau tensiune external) | io239 | 0/1 |
| Odometer total | io199 | metri |
| iButton driver ID | io78 | număr 64-bit |
| Driver 1 name (tacograf) | io403 | text |
| Sleep mode | io200 | 0-3 |
| CAN OBD: RPM / coolant / engine load / etc. | io81-io115 | variabile |

Listă completă pe
[wiki.teltonika-gps.com/view/FMB_AVL_ID_List](https://wiki.teltonika-gps.com/view/FMB_AVL_ID_List).

### Pas 2 — verifici în Traccar UI că ajunge

După un ciclu normal de update (1-2 min cu cer liber sau cu device trezit din
sleep), în Traccar:

1. Click pe device → dreapta jos se deschide panel-ul cu ultimele poziții.
2. Secțiunea **Attributes** arată toate parametrii pe care device-ul i-a
   trimis. Căutăm numele din tabel de mai sus (ex. `io25` pentru BLE
   temperature sau direct `ignition`, `motion`, `batteryLevel` dacă Traccar
   i-a tradus deja).

Dacă parametrul NU apare în Traccar, Configurator-ul n-a fost salvat corect
sau device-ul nu l-a transmis încă (prioritate Low = trimis doar la sleep-out
sau motion).

### Pas 3 — unde îl vezi în OpenRemote

Adapter-ul (`services/gps-adapter/src/main.go`) mapează automat parametrii
frecvenți la atribute numite pe `TrackerAsset`. Sunt deja pre-definite slot-urile:

- `batteryLevel`, `batteryVoltage`, `externalVoltage`, `power`, `ignition`,
  `movement`, `fuelLevel`, `odometer`, `gsmSignal`, `sleepMode`, `driverID`
- `din1`, `din2`, `din3`
- `bleTemperature1-4`, `dallasTemperature1-4`

**Orice alt parametru** pe care Traccar îl primește ajunge automat în atributul
catch-all `rawAttributes` (tip JSON) de pe asset. Exemplu — dacă device-ul
trimite `io88` (some custom CAN value), nu apare ca atribut nume-it, dar îl
vezi în `rawAttributes` ca `{"io88": 42}`.

### Ce faci în UI cu un atribut nou nume-it

Apare singur pe asset. Pentru a-l vedea în dashboard:
1. În widget-ul **Asset list** → **+ Add column** → alegi atributul.
2. Pentru alarme → **Rules** → creează regula pe `<NumeAtribut> OPERATOR valoare`.
3. Pentru chart istoric → **Insights** → widget **Chart line** → alegi atributul.

### Ce faci în UI cu `rawAttributes` (JSON blob)

Două abordări:

**A.** Afișare ca atare în dashboard — widget **Asset attribute** tip JSON,
user-ul vede structura brută, copy-paste pentru debugging.

**B.** Promovezi un sub-atribut la nivel de atribut nume-it:
1. Asset → + Add attribute → Name: `can_rpm`, Type: number.
2. **Agent Link** pe noul atribut → alege "HTTP Agent self" (sau mai simplu —
   rezolvă cu un rule JSON care citește `rawAttributes.io85` și scrie în
   `can_rpm`).

Dacă ai mulți senzori noi pe care vrei să-i promovezi la atribute named, spune-mi
ce AVL IDs folosesc și adaug mapare directă în adapter (fișier
[services/gps-adapter/src/main.go](../services/gps-adapter/src/main.go) —
funcția `pullCommonAttrs`).

## Ce ramâne de făcut dacă vrei feature-uri fleet-specific

OpenRemote core (ce rulăm noi) e IoT generic. Dacă vrei:

- **CarAsset** cu Trip widget (sesiuni de condus, opriri, kilometraj automat)
- **Teltonika MQTT handler** built-in (fără transformer/adapter)
- **Driver asset** cu atribuire automată vehicul

Trebuie trecut pe [openremote/fleet-management](https://github.com/openremote/fleet-management)
— cere fork + build custom, detalii în discuția din repo-ul nostru (commit
`10425e0` și următoarele). Pentru nivelul actual de utilizare (hartă +
dashboards + alerte simple), setup-ul curent acoperă totul.
