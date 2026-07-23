# ZKBio Gateway Console

A lightweight Angular UI for viewing and managing the ZKTeco biometric attendance
gateway (`ismila/erp/zkbio`) from a browser instead of the terminal. It talks to
the gateway's **management API** over HTTP — it never touches the database or the
HR service directly.

## What it does

| Page | Endpoint(s) | Capability |
|---|---|---|
| **Dashboard** | `GET /dashboard` | Punch counts (pending / failed / sent / total) and a per-device health table. |
| **Devices** | `GET/POST /devices` | List and register devices. |
| **Device detail** | `GET/PATCH/DELETE /devices/:id`, `POST /devices/:id/hr-credential`, `POST /devices/:id/requeue` | Edit, activate/deactivate, set/rotate the HR credential, requeue failed punches, delete, and view that device's recent logs. |
| **Logs** | `GET /punches` | Filterable, paginated punch viewer (device, delivery status, employee #, date range) with IN/OUT and delivery status. |
| **Settings** | — | Gateway base URL, management API key, light/dark theme. Stored in the browser's `localStorage`. |

The HR device key is write-only: the gateway stores it encrypted and never
returns it, so the UI only ever shows a display prefix and a "set / missing" flag.

The gateway URL defaults to **the same host the console is served from, on port
8080**. So when the console runs on the Pi (below), it targets that Pi's gateway
automatically; under `ng serve` it targets `localhost:8080`. Override it anytime
in **Settings**.

## Run on the Pi (Docker — starts on boot)

This is the intended production setup. The console is a service in the repo's
root `docker-compose.yml` (`webui`, on `WEBUI_PORT`, default **8082**). Because
the stack runs `restart: unless-stopped` and is brought up by the systemd unit,
**the console starts automatically on boot** with the rest of the gateway — there
is nothing extra to install.

From the repo root on the Pi:

```bash
make up                       # builds + starts zkbio, webui, mariadb, phpmyadmin
# console:  http://<pi-ip>:8082
```

To (re)build just the console after a change:

```bash
docker compose up -d --build webui
```

Then open `http://<pi-ip>:8082`, go to **Settings**, and set the **management API
key** (`MGMT_API_KEY`). Leave it blank if the gateway runs open. The gateway URL
should already be correct (same host, :8080); click **Test connection** to verify.

## Run locally for development

```bash
cd web
npm install
npm start            # ng serve → http://localhost:4200
```

Requires Node.js 20+ (tested on Node 24). Point it at a reachable gateway in
**Settings**; the API key is sent as the `X-API-Key` header.

## Build the static bundle by hand

```bash
npm run build        # outputs static files to dist/web/browser/
```

Serve `dist/web/browser/` with any static host (the provided `nginx.conf` shows
the SPA fallback). It's a pure SPA — the host must fall back to `index.html` for
client-side routes.

## CORS / auth notes

- The gateway already sets `AllowAllOrigins` and allow-lists the `X-API-Key`
  header (`src/main.go`), so the browser can call it cross-origin with no extra
  server config.
- Auth uses a shared key (`X-API-Key` / `MGMT_API_KEY`), not cookies. Anyone with
  the key and network access has full management rights — keep the console on the
  trusted LAN, same as the gateway.

## Structure

```
Dockerfile              node build → nginx serve (multi-arch, incl. arm64)
nginx.conf              SPA fallback + asset caching
src/app/
├── app.ts                 # shell: sidebar nav, theme toggle, connection status
├── app.routes.ts          # lazy-loaded routes
├── core/
│   ├── models.ts          # TS mirror of the gateway's mila_structs contract
│   ├── api.service.ts     # typed client + apiError() helper
│   ├── config.service.ts  # base URL / API key / theme (localStorage)
│   ├── api-key.interceptor.ts
│   ├── toast.service.ts   # + toast-host.ts
│   └── format.ts          # date helpers
└── features/
    ├── dashboard/
    ├── devices/           # list, detail, device-form, credential-form
    ├── logs/
    └── settings/
```

`core/models.ts` mirrors the gateway's `mila_structs` (`DeviceResponse`,
`PunchResponse`, `DashboardResponse`, the `AnswerResponse` / `PaginationResponse`
envelopes). If the gateway's management contract changes, update that file.
