-- ================================================================
-- Migration 001 — ZKTeco biometric gateway schema (database: erp_zkbio)
--
-- This gateway runs on a Raspberry Pi on the same LAN as the ZKTeco devices.
-- It collects punches from those devices (by polling them over TCP 4370 and by
-- accepting ADMS/iclock HTTP pushes) and forwards them to the ERP HR service.
--
-- The three tables below are the durable spine that lets the gateway survive a
-- reboot and always know which punches have already been delivered to HR and
-- acknowledged, and which are still outstanding:
--
--   • zk_devices          — registry of physical ZKTeco units. Each row carries
--     the device's LAN address (for pull) / serial number (for push) AND the HR
--     credential (X-Device-Id + encrypted X-Device-Key) that punches from this
--     unit are forwarded under. Per-device credentials, so HR can attribute a
--     punch to the physical unit it came from.
--   • attendance_punches  — every punch, stored FIRST, then delivered. The
--     delivery_status column is the source of truth across restarts:
--     Pending -> Sent (HR 200) | Failed (gave up). A UNIQUE key makes both
--     collection and re-delivery idempotent.
--   • delivery_batches    — one row per HR forward attempt (monitoring/audit):
--     how many punches, HR's accepted/ignored/failed/duplicate counts, HTTP
--     status. Never needed for correctness, invaluable for debugging.
--
-- Idempotent: CREATE TABLE IF NOT EXISTS — re-running is a clean no-op.
-- ================================================================

CREATE TABLE IF NOT EXISTS zk_devices (
    id                CHAR(36)     NOT NULL DEFAULT UUID() PRIMARY KEY,
    name              VARCHAR(150) NOT NULL DEFAULT '',
    serial_number     VARCHAR(100)          DEFAULT NULL,     -- device SN; how ADMS/iclock push identifies itself. NULL until known (pull devices learn it on first connect); multiple NULLs are allowed by the unique key.
    ip_address        VARCHAR(64)  NOT NULL DEFAULT '',       -- for pull mode
    port              INT          NOT NULL DEFAULT 4370,     -- ZK TCP port
    comm_password     INT          NOT NULL DEFAULT 0,        -- device COMM key (0 = none)
    mode              ENUM('pull','push','both') NOT NULL DEFAULT 'both',
    timezone          VARCHAR(64)  NOT NULL DEFAULT '',       -- overrides the gateway TZ for this unit's naive timestamps

    -- HR credential this device's punches are forwarded under (per-device).
    hr_device_id      VARCHAR(100) NOT NULL DEFAULT '',       -- sent as X-Device-Id
    hr_device_key_enc VARBINARY(512) DEFAULT NULL,            -- AES-GCM ciphertext of the X-Device-Key
    hr_key_prefix     VARCHAR(16)  NOT NULL DEFAULT '',       -- first chars, for display only (never the full key)

    -- Punch-direction mapping. ZK devices report a numeric punch state; HR only
    -- accepts IN/OUT. This JSON maps state->IN|OUT (empty = use the default map
    -- in code). e.g. {"0":"IN","1":"OUT","4":"IN","5":"OUT"}
    punch_state_map   VARCHAR(255) NOT NULL DEFAULT '',

    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    poll_interval_s   INT          NOT NULL DEFAULT 0,        -- 0 = use gateway default
    last_seen_at      DATETIME     DEFAULT NULL,              -- last successful contact (pull connect or push)
    last_pull_at      DATETIME     DEFAULT NULL,
    last_pull_error   VARCHAR(255) NOT NULL DEFAULT '',
    last_cursor_time  DATETIME     DEFAULT NULL,              -- newest event_time seen from this device (pull skip-ahead)

    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_serial (serial_number),
    INDEX idx_dev_active (is_active)
);

CREATE TABLE IF NOT EXISTS attendance_punches (
    id                CHAR(36)     NOT NULL DEFAULT UUID() PRIMARY KEY,
    device_id         CHAR(36)     DEFAULT NULL,              -- FK zk_devices. Always set at insert (push SNs are auto-registered first); only becomes NULL if the device row is later deleted.
    device_serial     VARCHAR(100) NOT NULL DEFAULT '',       -- denormalized device SN for display
    employee_number   VARCHAR(50)  NOT NULL DEFAULT '',        -- device user id / enroll number == HR employee_number
    punch_state       INT          NOT NULL DEFAULT 0,         -- raw device state code (0=in,1=out,...)
    log_type          ENUM('IN','OUT') NOT NULL DEFAULT 'IN',  -- mapped direction sent to HR
    event_time        DATETIME     NOT NULL,                   -- punch time
    verify_mode       INT          NOT NULL DEFAULT 0,         -- 0 pwd,1 fp,2 card,15 face,... (raw, informational)
    source            ENUM('pull','push') NOT NULL DEFAULT 'pull',
    latitude          DECIMAL(9,6) DEFAULT NULL,
    longitude         DECIMAL(9,6) DEFAULT NULL,
    raw_payload       VARCHAR(512) NOT NULL DEFAULT '',

    -- Delivery tracking — the reason this table exists.
    delivery_status   ENUM('Pending','Sent','Failed') NOT NULL DEFAULT 'Pending',
    delivery_attempts INT          NOT NULL DEFAULT 0,
    last_attempt_at   DATETIME     DEFAULT NULL,
    last_error        VARCHAR(255) NOT NULL DEFAULT '',
    sent_at           DATETIME     DEFAULT NULL,
    batch_id          CHAR(36)     DEFAULT NULL,               -- the delivery_batches row that delivered it

    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Idempotency: the same physical punch collected twice (pull re-read, or a
    -- push re-send) collapses to one row. Keyed on device_id so it is stable even
    -- before a device's serial is known. Mirrors HR's uq_devlog intent so a
    -- re-forward is a no-op on both sides.
    UNIQUE KEY uq_punch (device_id, employee_number, log_type, event_time),
    INDEX idx_punch_status  (delivery_status, event_time),
    INDEX idx_punch_device  (device_id, event_time),
    INDEX idx_punch_created (created_at),
    INDEX idx_punch_emp     (employee_number, event_time),
    CONSTRAINT fk_punch_device FOREIGN KEY (device_id) REFERENCES zk_devices(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS delivery_batches (
    id             CHAR(36)     NOT NULL DEFAULT UUID() PRIMARY KEY,
    device_id      CHAR(36)     DEFAULT NULL,
    hr_device_id   VARCHAR(100) NOT NULL DEFAULT '',
    punch_count    INT          NOT NULL DEFAULT 0,
    http_status    INT          NOT NULL DEFAULT 0,
    hr_total       INT          NOT NULL DEFAULT 0,
    hr_accepted    INT          NOT NULL DEFAULT 0,
    hr_ignored     INT          NOT NULL DEFAULT 0,
    hr_failed      INT          NOT NULL DEFAULT 0,
    hr_duplicate   INT          NOT NULL DEFAULT 0,
    success        BOOLEAN      NOT NULL DEFAULT FALSE,
    error          VARCHAR(512) NOT NULL DEFAULT '',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_batch_device  (device_id, created_at),
    INDEX idx_batch_created (created_at)
);

-- ================================================================
-- employees — the gateway's local registry of device users (names).
--
-- ZKTeco devices store a name alongside each enrolled user id. The HR ingest
-- contract carries only employeeNumber (see pkg/hrclient), so names are NOT
-- forwarded — they exist here so operators can tell WHO a punch belongs to when
-- reconciling in the console, in exports, and in the punch list.
--
-- Filled from three directions, all upserting on employee_number:
--   • pull   — the collector reads the device user table over TCP 4370
--   • push   — an ADMS device POSTs USERINFO/OPERLOG rows to /iclock/cdata
--   • import — an operator uploads a CSV (or edits a row by hand -> 'manual')
--
-- employee_number is globally unique, not per-device: the same person enrolled
-- on the Main Gate and the Workshop reader is one row. That matches how HR keys
-- attendance (employeeNumber alone) and is what makes a CSV round-trip sane.
--
-- Precedence: a name set by import/manual is authoritative and a later device
-- sync will NOT overwrite it (see employee_db.UpsertFromDevice). Operators fix a
-- name once; a stale name still sitting in device flash cannot undo the fix.
-- ================================================================
CREATE TABLE IF NOT EXISTS employees (
    id                CHAR(36)     NOT NULL DEFAULT UUID() PRIMARY KEY,
    employee_number   VARCHAR(50)  NOT NULL,                 -- device user id (PIN) == HR employee_number
    name              VARCHAR(150) NOT NULL DEFAULT '',
    card_number       VARCHAR(50)  NOT NULL DEFAULT '',      -- RFID card, if the device carries one
    privilege         INT          NOT NULL DEFAULT 0,       -- device privilege (0 user, 14 admin)
    device_user_id    VARCHAR(50)  NOT NULL DEFAULT '',      -- device-internal uid (NOT the PIN); informational

    -- Where this row's name came from. 'import'/'manual' are sticky: device
    -- syncs refresh the other columns but leave such a name alone.
    source            ENUM('pull','push','import','manual') NOT NULL DEFAULT 'pull',

    last_device_id    CHAR(36)     DEFAULT NULL,             -- FK zk_devices: unit this user was last seen on
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    last_seen_at      DATETIME     DEFAULT NULL,             -- last time a device reported this user
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uq_employee_number (employee_number),
    INDEX idx_emp_name (name),
    INDEX idx_emp_device (last_device_id),
    CONSTRAINT fk_emp_device FOREIGN KEY (last_device_id) REFERENCES zk_devices(id) ON DELETE SET NULL
);
