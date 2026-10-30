# Running the gateway on Windows

The gateway runs the same on Windows as it does on the Raspberry Pi — same Go
binary, same database schema, same HR contract. Two deployment paths are
supported; pick one.

| | Docker Desktop | Native Windows Service |
|---|---|---|
| What you install | Docker Desktop (WSL2 backend) | Go 1.25+ (to build), MariaDB for Windows |
| What you get | Gateway + web console + MariaDB + phpMyAdmin | Gateway only |
| Starts at boot | Docker Desktop must be set to start at login | Yes — a real service, starts before login |
| Best for | A dedicated box where you want the whole stack | A machine that already runs MariaDB, or where Docker is not allowed |

Both are driven by `make.ps1` in the repo root, the PowerShell counterpart of the
Makefile. Run `.\make.ps1 help` for the full target list.

> **Execution policy.** If PowerShell refuses to run the scripts, allow them for
> the current session only:
> ```powershell
> Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
> ```

---

## Path A — Docker Desktop

Identical to the Pi deployment, so the whole stack (web console, phpMyAdmin) is
available.

```powershell
git clone <this repo>
cd zkteco_bio_server

.\make.ps1 init         # scaffold .env and env\*.env
notepad .env            # ports and TZ
notepad env\zkbio.env   # HR URL, DB password, CRED_ENC_KEY, MGMT_API_KEY
notepad env\db.env      # DB passwords (MYSQL_PASS must match ERP_ZKBIO_DB_PASSWORD)

.\make.ps1 preflight
.\make.ps1 up
.\make.ps1 logs
```

Then open the web console at `http://localhost:8082`.

**Updating later:** `.\make.ps1 update`. It pulls the latest code, re-runs
preflight and rebuilds the gateway; your `.env` files and the database are kept.

Two Windows-specific points:

- **Boot persistence.** There is no systemd here. Enable *Start Docker Desktop
  when you log in* in Docker Desktop's settings, and configure the machine to log
  the operator account in automatically — otherwise the stack only comes up when
  someone signs in. If that is unacceptable, use Path B: a real Windows Service
  starts before any login.
- **Line endings.** `.gitattributes` pins `*.sh`, `*.sql` and the Dockerfile to
  LF. Without it, a Windows checkout gives `db/init/01-databases.sh` CRLF endings
  and the MariaDB container fails with `bad interpreter: /bin/bash^M`, leaving the
  database and users uncreated. If you cloned before `.gitattributes` existed,
  re-normalise with `git rm --cached -r . ; git reset --hard`.

## Path B — native Windows Service

No Docker. The gateway runs directly under the Service Control Manager, which
means it starts at boot before anyone logs in and restarts itself on failure.

### 1. Install MariaDB

Install [MariaDB for Windows](https://mariadb.org/download/) and create the
database and user the gateway will own:

```sql
CREATE DATABASE erp_zkbio CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'erp_zkbio'@'localhost' IDENTIFIED BY '<password>';
GRANT ALL PRIVILEGES ON erp_zkbio.* TO 'erp_zkbio'@'localhost';
FLUSH PRIVILEGES;
```

The gateway creates and migrates its own tables on startup — nothing else to do.

### 2. Build

```powershell
.\make.ps1 native-build          # or: .\make.ps1 native-build -Arch arm64
```

This produces `dist\windows\zkbio.exe`, a `logs\` folder beside it, and a `.env`
seeded from `windows\zkbio.env.example`.

### 3. Configure

```powershell
notepad dist\windows\.env
.\make.ps1 native-preflight
```

`TZ` must be an **IANA** zone name (`Africa/Dar_es_Salaam`), not a Windows one
(`E. Africa Standard Time`) — preflight rejects the latter. The binary embeds the
IANA timezone database, so nothing needs installing for this to work.

### 4. Install and start the service

From an **elevated** PowerShell (Run as Administrator):

```powershell
.\make.ps1 service-install
# If MariaDB runs on the same box, have the gateway wait for it:
#   .\make.ps1 service-install -DependsOn MariaDB
.\make.ps1 service-start
.\make.ps1 service-status
```

`make.ps1` delegates to the binary itself — `zkbio.exe service install|uninstall|start|stop|status`
works standalone if you prefer, and takes the same `-depends` flag.

The service is registered as **`zkbio`** (*ZKTeco Biometric Attendance Gateway*)
with:

- **Automatic (Delayed Start)** — at boot the network stack and MariaDB are often
  not ready yet.
- **Recovery actions** — restart after 15s, then 60s, then 120s. A database that
  is not up yet costs a retry, never a day of attendance.

### 5. Open the firewall

Devices on the LAN push to the gateway, so inbound TCP 8080 must be allowed:

```powershell
New-NetFirewallRule -DisplayName "ZKTeco gateway (zkbio)" `
  -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Private
```

Keep the rule scoped to the **Private** profile — the management API is only
protected by `MGMT_API_KEY`, so it should never face a public network.

### Where things are

| | |
|---|---|
| Binary | `dist\windows\zkbio.exe` |
| Config | `dist\windows\.env` |
| Logs | `dist\windows\logs\server.log` (rotated, 10 MB × 5) |
| Start/stop failures | Event Viewer → Windows Logs → Application, source `zkbio` |

The service sets its working directory to the folder containing `zkbio.exe`, so
`.env` and the relative `LOG_DIR` resolve there rather than in `C:\Windows\System32`.

### Upgrading

```powershell
.\make.ps1 native-update     # elevated: pull, stop, rebuild exe, start
```

`dist\windows\.env` is left alone; only the exe is replaced. If the build fails,
the old exe is started again unchanged.

Nothing is lost across the restart: every punch is written to the database as
`Pending` before delivery is attempted and only marked `Sent` once HR confirms,
so the gateway resumes from the queue on start.

### Removing

```powershell
.\make.ps1 service-uninstall     # elevated
```

---

## Troubleshooting

**"This target changes the Windows Service registration - run PowerShell as Administrator."**
`service-install` and `service-uninstall` talk to the SCM. Right-click PowerShell → Run as Administrator.

**Service starts then immediately stops.**
Almost always config or database. Check, in order:
`dist\windows\logs\server.log`, then Event Viewer → Application → source `zkbio`.
A missing required variable (`MYSQL_HOST`, `MYSQL_USER`, `MYSQL_PASS`,
`MYSQL_DBNAME`, `HR_INGEST_URL`) is reported by name.

**Punches arrive in HR with times a few hours off (often exactly UTC).**
First line of the gateway log reads `timezone tz=... now=...`; check it shows your
zone and the right local time. The container and the service ignore the Windows
clock and zone: only `TZ` counts. If the log is right but the time on the
**device's screen** is wrong, update the gateway (it now sends the device its
time zone) and restart the device once. If `TZ` is wrong or a Windows zone
name, set an IANA name and run `.\make.ps1 preflight` (or `native-preflight`).

**Devices cannot reach the gateway.**
The firewall rule above, and confirm the device's ADMS/cloud server address
points at this machine's LAN IP on port 8080 — not `localhost`.

**Pull mode finds no device.**
Outbound TCP 4370 to the device must be allowed, and the device must be on the
same LAN segment. Push mode is the more reliable path on firmwares that only
speak the buffered transfer.
