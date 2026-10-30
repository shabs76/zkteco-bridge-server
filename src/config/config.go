// Package config loads and validates the gateway's runtime configuration from
// environment variables (optionally seeded from a .env file). main() calls
// Load() first and hard-exits if anything required is missing, so the rest of
// the process can assume a fully-populated Config.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved gateway configuration.
type Config struct {
	// HTTP
	AppEnv string
	Port   string // management + iclock HTTP port

	// Database (local MariaDB on the Pi)
	MySQLHost   string
	MySQLPort   string
	MySQLUser   string
	MySQLPass   string
	MySQLDBName string

	// HR ingest target. The gateway POSTs batches of punches here with each
	// device's own X-Device-Id / X-Device-Key headers.
	HRIngestURL   string
	HRTimeout     time.Duration
	HRInsecureTLS bool // allow self-signed HR TLS (LAN/testing only)

	// Forwarder tuning
	ForwardInterval time.Duration // how often to flush Pending punches
	ForwardBatch    int           // max punches per HR request (HR caps at 500)
	MaxAttempts     int           // give up (mark Failed) after this many delivery attempts; 0 = never give up

	// Pull collector tuning
	PullEnabled     bool
	PullInterval    time.Duration // default poll cadence per device
	PullConnTimeout time.Duration

	// Employee (device user) name sync over the pull protocol. Enrolments change
	// rarely, so this runs on its own much slower cadence than the punch poll —
	// reading the whole user table every 30s would be pointless load on the device.
	EmployeeSyncEnabled  bool
	EmployeeSyncInterval time.Duration

	// Push (ADMS/iclock) tuning
	PushEnabled    bool
	AutoRegisterSN bool // auto-create an inactive device row for an unknown pushing SN

	// Retention: purge delivered punches older than this (0 = keep forever)
	PunchRetention time.Duration

	// Secret used to encrypt each device's stored HR key at rest (AES-GCM).
	// 32 bytes recommended; if empty, keys are stored as-is (documented).
	CredEncKey string

	// Timezone the gateway interprets naive device timestamps in.
	TimeZone string

	LogDir string
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	c := &Config{
		AppEnv:      getenv("APP_ENV", "production"),
		Port:        getenv("PORT", "8080"),
		MySQLHost:   os.Getenv("MYSQL_HOST"),
		MySQLPort:   getenv("MYSQL_PORT", "3306"),
		MySQLUser:   os.Getenv("MYSQL_USER"),
		MySQLPass:   os.Getenv("MYSQL_PASS"),
		MySQLDBName: os.Getenv("MYSQL_DBNAME"),
		HRIngestURL: os.Getenv("HR_INGEST_URL"),
		CredEncKey:  os.Getenv("CRED_ENC_KEY"),
		TimeZone:    getenv("TZ", "UTC"),
		LogDir:      getenv("LOG_DIR", "logs"),
	}

	c.HRTimeout = getdur("HR_TIMEOUT_SECONDS", 30*time.Second)
	c.HRInsecureTLS = getbool("HR_INSECURE_TLS", false)

	c.ForwardInterval = getdur("FORWARD_INTERVAL_SECONDS", 15*time.Second)
	c.ForwardBatch = getint("FORWARD_BATCH", 200)
	if c.ForwardBatch > 500 {
		c.ForwardBatch = 500 // HR rejects batches larger than 500
	}
	c.MaxAttempts = getint("FORWARD_MAX_ATTEMPTS", 0)

	c.PullEnabled = getbool("PULL_ENABLED", true)
	c.PullInterval = getdur("PULL_INTERVAL_SECONDS", 30*time.Second)
	c.PullConnTimeout = getdur("PULL_CONN_TIMEOUT_SECONDS", 10*time.Second)

	c.EmployeeSyncEnabled = getbool("EMPLOYEE_SYNC_ENABLED", true)
	c.EmployeeSyncInterval = getdur("EMPLOYEE_SYNC_INTERVAL_SECONDS", time.Hour)

	c.PushEnabled = getbool("PUSH_ENABLED", true)
	c.AutoRegisterSN = getbool("PUSH_AUTO_REGISTER_SN", true)

	c.PunchRetention = getdur("PUNCH_RETENTION_SECONDS", 90*24*time.Hour)

	// Required
	missing := []string{}
	for k, v := range map[string]string{
		"MYSQL_HOST":    c.MySQLHost,
		"MYSQL_USER":    c.MySQLUser,
		"MYSQL_PASS":    c.MySQLPass,
		"MYSQL_DBNAME":  c.MySQLDBName,
		"HR_INGEST_URL": c.HRIngestURL,
	} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

// DSN returns the go-sql-driver/mysql connection string. parseTime=true so
// DATETIME columns scan into time.Time; loc is the configured timezone so naive
// datetimes round-trip consistently.
func (c *Config) DSN() string {
	loc := c.TimeZone
	if loc == "" {
		loc = "UTC"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=%s&multiStatements=false",
		c.MySQLUser, c.MySQLPass, c.MySQLHost, c.MySQLPort, c.MySQLDBName, strings.ReplaceAll(loc, "/", "%2F"))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getint(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getbool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getdur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return def
}
