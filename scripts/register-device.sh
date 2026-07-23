#!/usr/bin/env bash
# Interactive helper to register a ZKTeco device with the gateway via its API.
#
# You need the HR device credential first: an HR admin creates it with
#   POST /hrm/devices        (on the ERP HR service)
# which returns a deviceIdentifier (X-Device-Id) and a one-time apiKey
# (X-Device-Key). Paste those below.
set -euo pipefail

read -rp "Gateway base URL [http://localhost:8080]: " BASE
BASE=${BASE:-http://localhost:8080}
read -rp "Management API key (MGMT_API_KEY): " APIKEY
read -rp "Device name (e.g. Main Gate): " NAME
read -rp "Mode [both/pull/push] (both): " MODE
MODE=${MODE:-both}
read -rp "Device IP (for pull, blank to skip): " IP
read -rp "Device serial number (for push, blank to skip): " SN
read -rp "Device TCP port [4370]: " PORT
PORT=${PORT:-4370}
read -rp "HR device id (X-Device-Id): " HRID
read -rsp "HR device key (X-Device-Key, hidden): " HRKEY; echo

payload=$(cat <<JSON
{
  "name": "${NAME}",
  "mode": "${MODE}",
  "ipAddress": "${IP}",
  "serialNumber": "${SN}",
  "port": ${PORT},
  "hrDeviceId": "${HRID}",
  "hrDeviceKey": "${HRKEY}"
}
JSON
)

echo "Registering..."
curl -sS -X POST "${BASE}/devices" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${APIKEY}" \
  -d "${payload}"
echo
