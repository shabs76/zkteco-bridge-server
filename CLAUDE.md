# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

## What this service is

The **ZKTeco biometric attendance gateway** (`ismila/erp/zkbio`), a member of the
`ismila/erp` cluster living under `erp_cluster/devices/`. It runs on a Raspberry
Pi 4 (arm64, 4 GB) on the LAN with the ZKTeco devices. It collects attendance
punches from those devices and forwards them to the ERP **HR service**
(`../../erp_hr_service_backend`, module `ismila/erp/hrm`).

It is a **client** of HR, not part of it. It never touches HR's database. It
talks to exactly one HR endpoint, the public device-ingestion route:

```
POST {HR_INGEST_URL}   (e.g. https://<domain>/hrm/device/attendance/logs)
Headers: X-Device-Id, X-Device-Key         # per-device credential; HR derives company from it
Body:    { "logs": [ { employeeNumber, logType(IN|OUT), eventTime, latitude?, longitude? } ] }
Reply:   AnswerResponse{ data: IngestResultResponse{ total, accepted, ignored, failed, duplicate } }
```

Keep the `pkg/hrclient` structs in lock-step with HR's `device_structs`
(`IngestLogsRequest`, `DeviceLogEntry`, `IngestResultResponse`). If the HR
contract changes, this is the file to update.

## Commands

All Go commands run from `src/` (that is where `go.mod` lives; module
`ismila/erp/zkbio`).

```bash
cd src
go build ./...                 # build
go vet ./...                    # vet
go run main.go                  # run (needs a reachable MariaDB + a .env)
gofmt -l .                      # format check
```

There are no tests in this repo yet; `go build ./...` and `go vet ./...` are the
static checks — run them for **both** targets, since `service_windows.go` is
build-tagged out on Linux/macOS:

```bash
GOOS=linux   go vet ./...
GOOS=windows go vet ./...
```
 Behaviour is verified by running against a MariaDB and a device
(or a simulated push with `curl` to `/iclock/cdata`).

Deployment (from the repo root, on the Pi):

```bash
make init        # scaffold .env + env/*.env
make preflight   # validate config
make up          # build + start the stack
make logs        # follow gateway logs
```

## Architecture — four strict layers

Same shape as the HR service:

```
routes/*.go  →  controllers/*_controllers/*.go  →  database/*_db/*.go  →  database/gendb/
```

| Layer | Package | Responsibility |
|---|---|---|
| `routes` | `routes` | Wire path + middleware only. `RegisterManagementRoutes`, `RegisterIClockRoutes`, called from `main.go`. |
| `controllers/<domain>_controllers` | `<domain>controllers` | Bind, validate, call `*_db`, write the response. |
| `database/<domain>_db` | `<domain>db` | Build SQL + args, call `gendb`. No business logic. |
| `database/gendb` | `gendb` | Shared pool + `SaveGeneral`/`SelectGeneral`/`UpdateGeneral`/`SelectRow` + `Migrate`. |

Package names are lowercase with **no underscores** even though directories have
them (`device_controllers` → `package devicecontrollers`). Structs live in
`mila_structs/<domain>_structs/` split `gen.go` (DB model) / `req.go` / `resp.go`,
mirroring HR.

The always-on work lives in `pkg/`:

```
pkg/
├── zk/          clean-room ZKTeco TCP protocol client (pull, port 4370)
├── iclock/      ADMS/iclock push protocol parsing + reply helpers
├── collector/   pull scheduler: polls devices, stores punches
├── forwarder/   delivery engine: batches Pending punches → HR, marks Sent/Failed
├── hrclient/    HTTP client for the HR ingest endpoint (contract mirror)
├── punchmap/    device punch-state → IN/OUT mapping (shared by pull + push)
├── cryptutil/   AES-GCM encryption of HR keys at rest
├── scheduler/   retention purge of delivered punches
├── middleware/  management-API key guard
├── dbutil/      duplicate-key classification + app-side UUID generation
├── validate/    UUID / pagination helpers
└── logger/      slog + lumberjack rotation (copied from HR)
```

### Where new code goes

| Adding | Where |
|---|---|
| New management endpoint | `routes/management.go` + `controllers/<domain>_controllers/*` + `database/<domain>_db/*` + `mila_structs/<domain>_structs/*` |
| New device-facing endpoint | `routes/iclock.go` + `controllers/iclock_controllers` |
| New SQL | `database/<domain>_db/{insert,select,update}.go` |
| Schema change | edit `migrations/001_init.sql` **and** the embedded copy `src/database/gendb/schema.sql` (keep them identical), or add a new migration file and apply it in `gendb.Migrate` |
| A new background worker | `pkg/<name>/`, started from `main.go` under the shared `ctx` |

## Non-obvious things that will bite you

- **The database is the delivery queue.** Correctness across reboots depends on:
  write the punch first (`Pending`), only mark `Sent` after HR returns 200. Never
  mark a punch `Sent` before HR confirms, and never delete `Pending`/`Failed`
  rows.
- **Two UUID strategies.** Devices get an **app-generated** UUID
  (`dbutil.NewUUID`) inserted explicitly, because a pull-only device has no unique
  natural key to re-select by. Punches and batches use MariaDB `DEFAULT UUID()`.
  Don't "unify" these without fixing the re-select problem.
- **Shared DB pool, not per-call open.** Unlike HR (which opens a fresh `*sql.DB`
  per call), this long-running appliance keeps one pool in `gendb` with small
  limits suited to a Pi. This is deliberate — see the comment in `gendb.go`.
- **`gendb` uses `multiStatements=false`.** `Migrate` splits the schema on `;`
  and runs statements one at a time; the schema must not contain a `;` inside a
  string/identifier.
- **`/iclock/*` speaks plain text, not JSON.** Those handlers return `"OK"` /
  option blocks the firmware expects. Do not wrap them in `AnswerResponse` and do
  not put them behind the management API key.
- **Router wildcard conflict.** gin/httprouter will panic if a `/iclock/*any`
  catch-all coexists with static `/iclock/cdata`. Unregistered `/iclock/*` paths
  are handled by the engine `NoRoute` in `main.go` instead.
- **Punch direction is lossy on some devices.** `punchmap` maps the raw state;
  devices not set to In/Out mode emit state 0 for everything (all `IN`). This is
  expected — configure the device or set `punchStateMap`.
- **Pull is best-effort across firmwares.** `pkg/zk` implements the traditional
  PREPARE_DATA transfer path (covers the large majority). A minority of firmwares
  only speak the buffered transfer; those devices should use push mode. Spot-check
  pull per model.

## Security notes

- **HR device keys are encrypted at rest** (`cryptutil`, AES-256-GCM, key from
  `CRED_ENC_KEY`) and never returned by any response — only a display prefix and a
  boolean are exposed. Decryption happens only in the forwarder, in memory.
- **Management API** is guarded by a shared `MGMT_API_KEY` (constant-time compare).
  If unset it logs a warning and runs open — acceptable only on a trusted LAN.
- **phpMyAdmin** should be used with the read-only `erp_ro` account for viewing;
  even a leaked `erp_ro` login cannot read a usable HR key (encrypted).
- **All SQL uses `?` placeholders.** Never `fmt.Sprintf` user input into SQL.

## Responses — two shapes only

```go
milastructs.AnswerResponse{State, Message, IsSuccess, Data, Adv}   // single item / mutation
milastructs.PaginationResponse{State, Data, Message, Pagination}   // list
```

Helpers: `milastructs.Ok/Fail/Err`. States `SuccessState`/`FailedState`/`ErrorState`.
These mirror the HR service exactly so tooling sees one contract across the cluster.

## Deployment

`Dockerfile` is a multi-stage pure-Go build (`golang:1.25-alpine` → `alpine`),
`CGO_ENABLED=0`, driven by `TARGETOS`/`TARGETARCH` from buildx so it produces an
arm64 image for the Pi. `docker-compose.yml` runs the gateway plus a local
MariaDB and a LAN-published phpMyAdmin. `systemd/zkbio.service` brings the stack
up on boot. Keep `go.mod`'s `go` directive and the builder image version in step.

### Windows

The gateway is supported on Windows as a first-class target — same binary, same
schema, same HR contract. Two paths: the Docker Desktop stack, and a native
install running under the Service Control Manager. `make.ps1` (repo root) is the
PowerShell counterpart of the `Makefile` and drives both; `windows/README.md` is
the operator guide and `windows/zkbio.env.example` the native config template.

Things to keep working when you touch this area:

- **`main.go` imports `_ "time/tzdata"`.** Windows ships no `/usr/share/zoneinfo`,
  so without the embedded IANA database `time.LoadLocation` fails and the gateway
  silently falls back to UTC — every punch reaches HR with a wrong wall-clock
  time. Do not drop this import to save the ~450 KB.
- **`TZ` must be an IANA zone name on Windows too** (`Africa/Dar_es_Salaam`, not
  `E. Africa Standard Time`). `scripts/preflight.ps1` rejects the Windows form.
- **`main()` is split.** `runGateway(ctx)` boots everything and blocks on `ctx`,
  returning errors instead of calling `os.Exit`, because the service handler has
  to report a failed start to the SCM. `platformMain()` is the seam:
  `service_windows.go` implements it (SCM detection + the `service` subcommands),
  `service_other.go` is the no-op that makes every other OS fall through to the
  foreground path. New startup steps belong in `runGateway`, not `main`.
- **A Windows service starts in `C:\Windows\System32`.** `service_windows.go`
  chdirs to the executable's directory before running, which is what makes the
  adjacent `.env` and the relative default `LOG_DIR` resolve. Do not add code
  that resolves a config path before that chdir.
- **`.gitattributes` pins `*.sh`/`*.sql`/`Dockerfile` to LF.** A CRLF checkout of
  `db/init/01-databases.sh` makes the MariaDB container fail with
  `bad interpreter: /bin/bash^M`, and the resulting symptom looks like a wrong
  password. Any new file that is bind-mounted into a Linux container needs an LF
  rule.
- **Keep both platforms building.** `GOOS=linux go vet ./...` and
  `GOOS=windows go vet ./...` — the Windows-only file is invisible to a plain
  `go vet` on Linux/macOS.
