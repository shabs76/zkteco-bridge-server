# ZKTeco Biometric Attendance Gateway (`zkbio`)

A small, self-contained appliance that runs on a **Raspberry Pi 4** on the same
LAN as your ZKTeco biometric devices. It collects attendance punches from those
devices, stores every punch durably with a delivery status, and forwards them to
the ERP **HR service** (`ismila/erp/hrm`) — the same cluster this repo lives in
under `devices/`.

It is the bridge between the ZKTeco devices and the HR module's biometric
ingestion endpoint (`POST /device/attendance/logs`), and it is built to be
unattended: it starts with the Pi, and after any restart it knows exactly which
punches have already been delivered and which have not, so nothing is lost and
nothing is sent twice.

Stack: **Go (Gin) + MariaDB + phpMyAdmin + an Angular web console**, all in
Docker Compose.

---

## What it does

- **Collects punches two ways**, so it works with most ZKTeco devices — the way
  open-source tools (pyzk, node-zklib) and BioTime do:
  - **Pull** — the gateway connects out to each device over TCP **4370** on a
    timer and reads new attendance logs (the classic ZK protocol).
  - **Push** — devices configured with the gateway as their **ADMS / "cloud"
    server** connect in and stream punches in real time over HTTP
    (`/iclock/cdata`).
- **Stores every punch first**, in local MariaDB, with a delivery status
  (`Pending` → `Sent` / `Failed`). A unique key makes both collection and
  re-delivery idempotent.
- **Forwards to HR** in batches, each under that device's own HR credential
  (`X-Device-Id` / `X-Device-Key`), retrying on failure with the database as the
  queue.
- **Survives reboots** — on boot it resumes delivering whatever is still
  `Pending`. A punch becomes `Sent` only after HR confirms receipt.
- **Ships a web console** (Angular SPA) so operators can register devices, set HR
  credentials, watch delivery health and browse logs from a browser instead of
  `curl` — no terminal needed.
- **Exposes phpMyAdmin on the LAN** so staff can view the raw data in a browser.

---

## Architecture

```
   ZKTeco devices                     Raspberry Pi (this appliance)                    Cloud
 ┌──────────────┐  pull TCP 4370  ┌──────────────────────────────────────┐
 │  Device A    │◀────────────────│  zkbio (Go/Gin)                :8080 │
 │  Device B    │                 │   ├─ collector  (pull poller)        │  HTTPS   ┌─────────────┐
 │  ...         │────────────────▶│   ├─ iclock     (push endpoints)     │─────────▶│  ERP HR svc │
 └──────────────┘  push /iclock   │   ├─ forwarder  (delivery)           │ X-Device │  /device/   │
                                  │   └─ management API + dashboard      │ headers  │  attendance │
                                  │                 │                    │          │  /logs      │
                                  │             MariaDB (durable queue)  │          └─────────────┘
   Operator browser               │                                      │
 ┌──────────────┐ loads SPA :8082 │   phpMyAdmin                   :8081 │
 │  web console │◀────────────────│   webui (Angular SPA / nginx)  :8082 │
 │  (Angular)   │────────────────▶│   management API  :8080 (X-API-Key) │
 └──────────────┘ REST /devices…  └──────────────────────────────────────┘
```

All four services run under one Docker Compose stack on the Pi. Note the split
in the console's data path: the browser **loads** the static Angular bundle from
the `webui` (nginx) service, but every management call goes **directly** from the
browser to the gateway's API on `:8080` (with `X-API-Key`) — `webui` never
proxies the API and never touches the database or HR.

The gateway code follows the same four-layer layout as the other ERP Go services
(`routes → controllers → database/*_db → database/gendb`); see `CLAUDE.md`.

### The HR ingest contract (must stay in lock-step)

The gateway delivers to HR exactly what HR expects:

```
POST {HR_INGEST_URL}
Headers: X-Device-Id, X-Device-Key         (per-device credential; company derived from it)
Body:    { "logs": [ { "employeeNumber": "...", "logType": "IN|OUT",
                       "eventTime": "RFC3339", "latitude": null, "longitude": null } ] }
Reply:   AnswerResponse{ data: { total, accepted, ignored, failed, duplicate } }
```

HR stores each raw punch **before** processing it, so an HTTP 200 means "durably
received" — the gateway then marks those punches `Sent`. HR de-duplicates on
`(device, employeeNumber, logType, eventTime)`, so a retried batch is a no-op on
both sides.

---

## Quick start (on the Pi)

```bash
git clone <this repo>          # into e.g. /opt/zkteco_bio_server
cd zkteco_bio_server

make init                      # copies .env and env/*.env from the examples
# edit .env, env/zkbio.env, env/db.env, env/phpmyadmin.env — replace every CHANGEME
#   - set HR_INGEST_URL to https://<your-erp-domain>/hrm/device/attendance/logs
#   - generate CRED_ENC_KEY:  openssl rand -base64 32
#   - generate MGMT_API_KEY:  openssl rand -hex 24
#   - make MYSQL_PASS == ERP_ZKBIO_DB_PASSWORD

make up                        # preflight + build + start (detached)
```

Then, to survive power cuts, install the systemd unit (see `systemd/README.md`).

Endpoints once running:
- **Web console: `http://<pi-ip>:8082`** — the operator UI (see below)
- Gateway API + device push: `http://<pi-ip>:8080`
- phpMyAdmin: `http://<pi-ip>:8081` (log in as `erp_ro` for read-only viewing)

The gateway creates and migrates its own database tables on startup, so there is
no separate migration step.

---

## Quick start (on Windows)

The same gateway runs on Windows — same binary, same schema, same HR contract.
`make.ps1` in the repo root is the PowerShell counterpart of the Makefile
(`.\make.ps1 help` lists every target). Two paths, both covered in detail in
[`windows/README.md`](windows/README.md):

**Docker Desktop** — the whole stack, exactly as on the Pi:

```powershell
.\make.ps1 init          # scaffold .env and env\*.env, then edit them
.\make.ps1 preflight
.\make.ps1 up
```

**Native Windows Service** — no Docker; bring your own MariaDB. The gateway
registers itself with the Service Control Manager, so it starts at boot before
anyone logs in and restarts itself on failure:

```powershell
.\make.ps1 native-build              # -> dist\windows\zkbio.exe + .env
notepad dist\windows\.env
.\make.ps1 native-preflight
# then, from an elevated PowerShell:
.\make.ps1 service-install -DependsOn MariaDB
.\make.ps1 service-start
```

Two things to know before you start:

- **`TZ` must be an IANA zone name** (`Africa/Dar_es_Salaam`), never a Windows one
  (`E. Africa Standard Time`). The binary embeds the IANA timezone database so no
  tzdata needs installing, but it does not understand Windows zone IDs — and a
  zone it cannot parse falls back to UTC, shifting every punch. Preflight rejects
  the Windows form.
- **Open inbound TCP 8080 on the Private profile** so LAN devices can push to it.

---

## Registering a device

Each physical ZKTeco unit maps to its **own HR device credential**, so HR can
tell which unit a punch came from.

1. **On the HR service**, an admin creates the credential:
   `POST /hrm/devices  { "name": "Main Gate", "location": "..." }` →
   returns `deviceIdentifier` (X-Device-Id) and a one-time `apiKey` (X-Device-Key).

2. **On the gateway**, register the unit and attach that credential:

   ```bash
   curl -X POST http://<pi-ip>:8080/devices \
     -H "X-API-Key: $MGMT_API_KEY" -H "Content-Type: application/json" \
     -d '{
           "name": "Main Gate",
           "mode": "both",
           "ipAddress": "192.168.1.201",
           "serialNumber": "CJXK201960000",
           "port": 4370,
           "hrDeviceId": "dev_1a2b3c4d5e6f7a8b",
           "hrDeviceKey": "the-one-time-apiKey-from-HR"
         }'
   ```

   or run `make register-device` for an interactive prompt.

### Configuring the devices themselves

- **Pull mode** — nothing needed on the device beyond a static IP the gateway can
  reach on port 4370. If the device has a COMM/communication password, pass it as
  `commPassword`.
- **Push mode** — in the device's network / cloud (ADMS) settings, set the server
  address to the Pi's IP and port **8080** (path `/iclock`). The device will
  register and start pushing. The gateway tells it which time zone to keep
  its clock in (from `TZ`, or the device's own `timezone` if set). If `PUSH_AUTO_REGISTER_SN=true`, an unknown device
  that starts pushing is auto-added (inactive of credential) so you only need to
  attach its HR credential afterward via
  `POST /devices/{id}/hr-credential`.

### Punch direction (IN/OUT)

HR needs `IN`/`OUT`. Devices report a numeric punch **state**; the gateway maps
it (0→IN, 1→OUT, plus break/overtime pairs). If a device is not configured for
direction and reports only state 0, everything maps to `IN` and HR's shift logic
pairs punches by time. To force a mapping, set a device `punchStateMap`, e.g.
`{"0":"IN","1":"OUT"}`.

---

## Employee names

The gateway keeps a local registry of device users, so an operator can tell *who*
a punch belongs to instead of reading bare PINs. Names reach it three ways:

| Source | How | Marked |
|---|---|---|
| **Pull** | The collector reads the device's enrolled-user table over TCP 4370, on its own slow cadence (`EMPLOYEE_SYNC_INTERVAL_SECONDS`, hourly by default) — enrolments change rarely. `POST /devices/:id/sync-employees` runs it immediately. | `pull` |
| **Push** | An ADMS device uploads its user table to `/iclock/cdata` as `USER` rows on the `OPERLOG`/`USERINFO` table. | `push` |
| **Import** | `POST /employees/import` with a CSV. | `import` |

> **Names are not forwarded to HR.** The HR ingest contract carries
> `employeeNumber` only (see `pkg/hrclient`), and this gateway keeps that
> contract in lock-step with HR. The registry is for the console, exports and
> reconciliation.

> **A punch line never carries a name.** In both protocols the punch itself is
> just a PIN and a timestamp; the device sends its user table separately. So a
> name appears against a punch once that user table has been read at least once —
> not necessarily on the very first punch from a newly enrolled person.

### Operator corrections win

A name set by CSV import or by editing a row (`PATCH /employees/:id`) is marked
`import` / `manual` and a later device sync **will not overwrite it**. Fix a
misspelling once and it stays fixed, even though the device's flash still holds
the old spelling. Everything else — card number, privilege, last-seen — keeps
refreshing from the device.

### Import / export

```bash
# Export (honours the same filters as the list endpoint)
curl -H "X-API-Key: $MGMT_API_KEY" http://<host>:8080/employees/export -o employees.csv

# Import — raw CSV body...
curl -X POST -H "X-API-Key: $MGMT_API_KEY" -H 'Content-Type: text/csv' \
     --data-binary @employees.csv http://<host>:8080/employees/import

# ...or a multipart upload (what the web console sends)
curl -X POST -H "X-API-Key: $MGMT_API_KEY" \
     -F file=@employees.csv http://<host>:8080/employees/import
```

The export columns are exactly what the import accepts, so a file can be
exported, edited in a spreadsheet and imported straight back. Only
`employeeNumber` is required; headers are matched case- and separator-insensitively
(`employee_number`, `Employee Number`), a UTF-8 BOM from Excel is tolerated, and
`pin` / `employeeId` / `userId` and `full name` are accepted as aliases. Rows
upsert on `employeeNumber`, so re-importing a corrected file fixes names rather
than duplicating people. The response reports created/updated/skipped/failed
counts with per-line errors.

---

## Management API

All JSON endpoints require the header `X-API-Key: <MGMT_API_KEY>` (or
`Authorization: Bearer <MGMT_API_KEY>`). The device-facing `/iclock/*` endpoints
are unauthenticated by design (LAN-local, device-driven).

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | liveness |
| GET | `/dashboard` | counts (pending/failed/sent) + per-device health |
| GET | `/devices` | list devices (paginated) |
| POST | `/devices` | register a device |
| GET | `/devices/:id` | get one device |
| PATCH | `/devices/:id` | update a device |
| DELETE | `/devices/:id` | delete a device (punch history retained) |
| POST | `/devices/:id/hr-credential` | set / rotate the HR credential |
| POST | `/devices/:id/requeue` | requeue this device's `Failed` punches |
| GET | `/punches` | list stored punches + delivery status (filter: `deviceId`, `employeeNumber`, `status`, `from`, `to`) |

---

## Management console (web UI)

A small **Angular single-page app** that puts the whole management API behind a
browser UI, so day-to-day operation needs no terminal. It lives in [`web/`](web/)
and is served as a fourth Docker Compose service on **`http://<pi-ip>:8082`**.

It talks only to the gateway's management API (it never touches the database or
HR directly). Pages:

- **Dashboard** — pending / failed / sent / total punch counts and a per-device
  health table (online / offline, HR-credential status).
- **Devices** — list and **register** devices; per-device detail with **edit**,
  **activate/deactivate**, **set / rotate HR credential**, **requeue failed
  punches**, **delete**, and that device's recent logs.
- **Logs** — filterable, paginated punch viewer (device, delivery status,
  employee #, date range) showing IN/OUT and delivery state.
- **Settings** — gateway URL, management API key and light/dark theme, stored in
  the browser.

**Configure it once:** open `http://<pi-ip>:8082`, go to **Settings**, and enter
the **management API key** (`MGMT_API_KEY`). The gateway URL defaults to the same
host on port 8080, so on the Pi it usually needs no change; click **Test
connection** to confirm. The HR device key stays write-only — the console only
ever shows a display prefix and a "set / missing" flag.

Because it runs under the same Compose stack (`restart: unless-stopped`) that the
systemd unit brings up, **the console starts automatically on boot** along with
the gateway — nothing extra to install. Set its port with `WEBUI_PORT` in `.env`.

For local development against a running gateway, run it directly with
`cd web && npm install && npm start` (see [`web/README.md`](web/README.md)).

---

## How "nothing is lost, nothing is sent twice" works

- Every punch is written to `attendance_punches` **before** any delivery attempt,
  as `Pending`.
- A unique key `(device_id, employee_number, log_type, event_time)` means the same
  physical punch — re-read on the next poll, or re-pushed — collapses to one row.
- The forwarder marks a punch `Sent` only after HR returns HTTP 200 for its batch.
  A crash between "HR received it" and "we recorded Sent" just means the batch is
  re-sent and HR de-duplicates it.
- On boot there is no in-memory state to rebuild: the forwarder queries what is
  still `Pending` and drains it.
- Delivered punches are purged after `PUNCH_RETENTION_SECONDS`; `Pending` and
  `Failed` punches are **never** purged.

---

## Updating an installed gateway

Your config (`.env`, `env/*.env`) and the database are kept; only the code and
the gateway image change. Punches queued while the gateway restarts are not lost.

```bash
make update              # Pi / Linux
```

```powershell
.\make.ps1 update        # Windows, Docker Desktop
.\make.ps1 native-update # Windows, native service (elevated PowerShell)
```

Each one pulls the latest code, re-runs preflight and restarts. If the pull
refuses because of local changes, `git status` shows what is in the way.

After updating, check the first log lines (`make logs`) for
`timezone tz=Africa/Dar_es_Salaam` with the correct local time next to it.

---

## Troubleshooting

- **Punch times in HR are hours off (usually shown in UTC)** — open a recent
  punch in phpMyAdmin (`attendance_punches`) and compare `raw_payload` with
  `event_time`:
  - *`raw_payload` already shows the wrong time* — the **device clock** is off.
    Push devices set their clock from the gateway and need the `TimeZone` the
    gateway now sends; restart the device once after updating so it fetches it,
    then check the time on its screen. A pull device keeps its own clock: set it
    on the device menu.
  - *`raw_payload` is right but `event_time` is wrong* — the gateway's zone is
    wrong. Check the `timezone` line in the gateway log, `TZ` in `.env`, and the
    device's own `timezone` field in the console (it overrides `TZ`).
  - Punches already delivered with a wrong time are not corrected
    automatically; fix them in HR.
- **A device shows `hrCredentialSet: false` on the dashboard** — its punches are
  being stored but held `Pending`. Attach a credential with
  `POST /devices/:id/hr-credential`.
- **`Failed` punches** — usually HR rejected the device credential (rotate it and
  `POST /devices/:id/requeue`) or an employee number is unknown to HR. Check the
  punch's `lastError` and the `delivery_batches` table.
- **Pull not working on a particular model** — a minority of firmwares only speak
  the newer buffered transfer; switch that device to push (ADMS) mode. Pull
  results should be spot-checked per model on first deployment.
- **View raw data** — phpMyAdmin at `:8081`, tables `zk_devices`,
  `attendance_punches`, `delivery_batches`.

## Repo layout

```
Dockerfile              arm64 multi-stage build for the Pi (gateway)
docker-compose.yml      zkbio + webui + mariadb + phpmyadmin
.env.example            compose-level config (ports, TZ)
web/                    Angular management console (SPA); own Dockerfile + nginx
env/*.env.example       per-service runtime config
db/init/                first-start DB + user creation
migrations/001_init.sql canonical schema (the app also applies it on boot)
systemd/                boot-persistence unit (Linux)
make.ps1                Windows operator shortcuts (Makefile counterpart)
windows/                native Windows Service deployment: env template + guide
scripts/                preflight + register-device helpers (.sh and .ps1)
src/                    the Go module (ismila/erp/zkbio) — see CLAUDE.md
```
