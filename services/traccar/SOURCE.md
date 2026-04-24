# traccar — service source

## What this is

The single Railway service left on the `OpenRemoteGPS` environment after the
2026-04-24 refactor. Runs the stock [`traccar/traccar:latest`](https://hub.docker.com/r/traccar/traccar)
image with a custom `startCommand` wrapper that:

1. Writes `/opt/traccar/conf/traccar.xml` with H2 embedded DB config.
2. Installs `/opt/traccar/data/run_backup.sh` — daily SQL-dump + gzip +
   14-day retention — onto the persistent volume.
3. Spawns the backup cron loop in the background (fires at 02:00 UTC).
4. Execs the JVM in the foreground (PID 1).

## Why this layout

OpenRemote, Keycloak, TimescaleDB, the Go adapter and transformer, and the
nginx edge are all gone. `gps.unitip.global` is bound directly to this
service's `:8082`. Device ingest TCP (Teltonika Codec 8) hits the Railway
TCP proxy `shortline.proxy.rlwy.net:57840` → this service's `:5027`.

No separate backup service — Traccar's own container runs the nightly
dump. This avoids the "Railway volumes can only attach to one service"
constraint: backup reads the H2 file from the same volume that holds it.

## Where the code lives

There is no custom code in this repo for this service — the behaviour comes
from the `startCommand` override persisted in Railway's service config. The
canonical copy of that command is kept below so a Railway rebuild or a
manual restore can reinstate it.

### Current Railway `startCommand`

Set on service id `baaea731-6af9-4010-b066-36981e5c2714` in environment
`a921bf3f-c1f4-41cd-b28e-6ad145946c14` via the GraphQL
`serviceInstanceUpdate` mutation. Update it the same way when this file
changes.

```sh
sh -c "cat > /opt/traccar/conf/traccar.xml <<'EOF'
<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<!DOCTYPE properties SYSTEM \"http://java.sun.com/dtd/properties.dtd\">
<properties>
<entry key=\"database.driver\">org.h2.Driver</entry>
<entry key=\"database.url\">jdbc:h2:./data/database;AUTO_SERVER=TRUE</entry>
<entry key=\"database.user\">sa</entry>
<entry key=\"database.password\"></entry>
</properties>
EOF
mkdir -p /opt/traccar/data/backup
cat > /opt/traccar/data/run_backup.sh <<'BSCRIPT'
#!/bin/sh
set -e
ts=\$(date -u +%Y%m%d_%H%M%S)
sql=/opt/traccar/data/backup/traccar_\${ts}.sql
/opt/traccar/jre/bin/java -cp '/opt/traccar/lib/*' org.h2.tools.Script -url 'jdbc:h2:/opt/traccar/data/database;AUTO_SERVER=TRUE' -user sa -script \"\$sql\"
gzip \"\$sql\"
find /opt/traccar/data/backup -name 'traccar_*.sql.gz' -mtime +14 -delete 2>/dev/null || true
echo \"\$(date -u +%FT%TZ) backup ok size=\$(stat -c%s \"\${sql}.gz\" 2>/dev/null || echo 0)B file=\${sql}.gz\" >> /opt/traccar/data/backup/backup.log
BSCRIPT
chmod +x /opt/traccar/data/run_backup.sh
(
  sleep 180
  while true; do
    now=\$(date -u +%s)
    target=\$(date -u -d 'today 02:00' +%s 2>/dev/null || echo 0)
    if [ \"\$target\" -le \"\$now\" ]; then target=\$((target + 86400)); fi
    sleep_s=\$((target - now))
    sleep \"\$sleep_s\"
    /opt/traccar/data/run_backup.sh >> /opt/traccar/data/backup/backup.log 2>&1 || echo \"\$(date -u +%FT%TZ) backup FAILED\" >> /opt/traccar/data/backup/backup.log
    sleep 60
  done
) &
exec /opt/traccar/jre/bin/java -jar tracker-server.jar conf/traccar.xml"
```

The key design choices:

- **`AUTO_SERVER=TRUE`** on the JDBC URL lets the backup `Script` tool
  connect to the same H2 file while Traccar holds the primary lock —
  H2 transparently starts a TCP listener on a loopback port and proxies
  the second connection through.
- **`org.h2.tools.Script`** dumps logical SQL (schema + data), which is
  smaller and more portable than an `org.h2.tools.Backup` binary dump.
- **Background `while sleep` loop** runs in the wrapper shell, not a real
  cron daemon, because the Traccar base image doesn't ship with cron. The
  initial 180 s delay gives the JVM time to open the DB before the first
  fire.

## How to trigger a backup manually

```bash
railway ssh --service traccar /opt/traccar/data/run_backup.sh
railway ssh --service traccar "ls -la /opt/traccar/data/backup/"
```

## How to download a backup

```bash
# Find the file
railway ssh --service traccar "ls -t /opt/traccar/data/backup/traccar_*.sql.gz | head -1"

# Stream it out
railway ssh --service traccar "cat /opt/traccar/data/backup/traccar_20260424_121312.sql.gz" > traccar_20260424_121312.sql.gz
```

## How to restore

Restore from a backup into a fresh H2 database by running the generated
SQL inside a new Traccar container (or locally), then mounting that
database file as `/opt/traccar/data/database.mv.db`:

```bash
# Spin up Traccar locally, wait until DB is initialized, stop it.
docker run --rm -v traccar-fresh:/opt/traccar/data traccar/traccar:latest &
sleep 10 && docker stop $(docker ps -qf ancestor=traccar/traccar:latest)

# Apply the dump
gunzip -c traccar_YYYYMMDD_HHMMSS.sql.gz | \
  docker run --rm -i -v traccar-fresh:/data traccar/traccar:latest \
    /opt/traccar/jre/bin/java -cp '/opt/traccar/lib/*' org.h2.tools.RunScript \
      -url 'jdbc:h2:/data/database' -user sa -script /dev/stdin

# Upload the resulting database.mv.db to Railway's traccar-volume via
# `railway ssh` + stdin redirection, or rsync-through-tar equivalent.
```

## Deployment signature

- Image: `traccar/traccar:latest` (pulled fresh on redeploy)
- `startCommand`: see above
- Volume: `traccar-volume` mounted at `/opt/traccar/data`
- Public HTTP: `gps.unitip.global` → Railway edge → `:8082`
- Public TCP: `shortline.proxy.rlwy.net:57840` → Railway TCP proxy → `:5027`
- Restart policy: `ON_FAILURE`, max 10 retries
