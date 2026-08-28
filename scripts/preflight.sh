#!/usr/bin/env bash
# Fail fast on obvious misconfiguration before starting the stack.
set -euo pipefail

cd "$(dirname "$0")/.."

fail=0
note() { echo "  - $1"; }

echo "Preflight checks:"

# Required files
for f in .env env/zkbio.env env/db.env env/phpmyadmin.env; do
  if [ ! -f "$f" ]; then
    note "MISSING $f  (run 'make init' first)"; fail=1
  fi
done
[ $fail -eq 0 ] || { echo "Preflight failed."; exit 1; }

# No leftover CHANGEME placeholders. Scan only the real config files, never the
# *.env.example templates (they keep their CHANGEME placeholders by design).
if grep -qI "CHANGEME" .env env/*.env 2>/dev/null; then
  note "CHANGEME placeholders still present — set real values:"
  grep -lI "CHANGEME" .env env/*.env 2>/dev/null | sed 's/^/      /'
  fail=1
fi

# Passwords must match between the two files
zk_pass=$(grep -E '^MYSQL_PASS=' env/zkbio.env | cut -d= -f2- || true)
db_pass=$(grep -E '^ERP_ZKBIO_DB_PASSWORD=' env/db.env | cut -d= -f2- || true)
if [ -n "${zk_pass}" ] && [ "${zk_pass}" != "${db_pass}" ]; then
  note "MYSQL_PASS (env/zkbio.env) does not match ERP_ZKBIO_DB_PASSWORD (env/db.env)"
  fail=1
fi

# HR ingest URL sanity
hr_url=$(grep -E '^HR_INGEST_URL=' env/zkbio.env | cut -d= -f2- || true)
case "${hr_url}" in
  http://*|https://*) : ;;
  *) note "HR_INGEST_URL looks unset or malformed: '${hr_url}'"; fail=1 ;;
esac
case "${hr_url}" in
  */device/attendance/logs) : ;;
  *) note "HR_INGEST_URL should end in /device/attendance/logs (got '${hr_url}')" ;;
esac

# Encryption key length hint
enc=$(grep -E '^CRED_ENC_KEY=' env/zkbio.env | cut -d= -f2- || true)
if [ -z "${enc}" ]; then
  note "CRED_ENC_KEY is empty — HR device keys will NOT be encrypted at rest"
fi

if [ $fail -ne 0 ]; then
  echo "Preflight failed."; exit 1
fi
echo "Preflight OK."
