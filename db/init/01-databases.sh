#!/bin/bash
# Creates the gateway database and a least-privilege user, plus a SELECT-only
# reporting account for phpMyAdmin.
#
# Runs ONCE, on first start, only while /var/lib/mysql is empty. It is a .sh
# (not .sql) because it needs the passwords from env/db.env — the MariaDB
# entrypoint does not expand variables inside .sql files.
#
# Re-running against an existing volume does nothing. The gateway itself creates
# and migrates the tables on startup (idempotent), so this only provisions the
# database and users.
set -euo pipefail

for var in ERP_ZKBIO_DB_PASSWORD ERP_READONLY_DB_PASSWORD; do
  if [ -z "${!var:-}" ]; then
    echo "db-init: $var is unset — refusing to create a passwordless user" >&2
    exit 1
  fi
done

# '%' is scoped to the internal compose network; the DB port is not published to
# the host, so it is not reachable off-box.
mariadb -u root -p"${MARIADB_ROOT_PASSWORD}" <<-SQL
	CREATE DATABASE IF NOT EXISTS erp_zkbio CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

	CREATE USER IF NOT EXISTS 'erp_zkbio'@'%' IDENTIFIED BY '${ERP_ZKBIO_DB_PASSWORD}';
	CREATE USER IF NOT EXISTS 'erp_ro'@'%'    IDENTIFIED BY '${ERP_READONLY_DB_PASSWORD}';

	-- The gateway owns its schema.
	GRANT ALL PRIVILEGES ON erp_zkbio.* TO 'erp_zkbio'@'%';

	-- Read-only browsing for phpMyAdmin. A leaked erp_ro login can read rows but
	-- cannot change anything; device HR keys are stored AES-encrypted, so they
	-- are not usable even if read.
	GRANT SELECT ON erp_zkbio.* TO 'erp_ro'@'%';

	FLUSH PRIVILEGES;
SQL

echo "db-init: created database erp_zkbio and users erp_zkbio (rw) + erp_ro (read-only)"
