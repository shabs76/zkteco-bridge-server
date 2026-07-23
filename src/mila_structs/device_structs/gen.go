package devicestructs

import (
	"database/sql"
	"time"
)

// ZKDevice mirrors the zk_devices table (a registered physical ZKTeco unit).
// HRDeviceKeyEnc is the AES-GCM ciphertext of the HR X-Device-Key and is NEVER
// exposed in any response — only the forwarder decrypts it in memory.
type ZKDevice struct {
	ID           string
	Name         string
	SerialNumber string
	IPAddress    string
	Port         int
	CommPassword int
	Mode         string // pull | push | both
	Timezone     string

	HRDeviceID     string
	HRDeviceKeyEnc []byte
	HRKeyPrefix    string

	PunchStateMap string

	IsActive       bool
	PollIntervalS  int
	LastSeenAt     sql.NullTime
	LastPullAt     sql.NullTime
	LastPullError  string
	LastCursorTime sql.NullTime

	CreatedAt time.Time
	UpdatedAt time.Time
}

// HasHRCredential reports whether a usable HR credential is configured.
func (d *ZKDevice) HasHRCredential() bool {
	return d.HRDeviceID != "" && len(d.HRDeviceKeyEnc) > 0
}
