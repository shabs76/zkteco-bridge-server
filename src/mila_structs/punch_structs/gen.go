package punchstructs

import (
	"database/sql"
	"time"
)

// Punch mirrors a row of attendance_punches.
type Punch struct {
	ID               string
	DeviceID         sql.NullString
	DeviceSerial     string
	EmployeeNumber   string
	PunchState       int
	LogType          string // IN | OUT
	EventTime        time.Time
	VerifyMode       int
	Source           string // pull | push
	Latitude         sql.NullFloat64
	Longitude        sql.NullFloat64
	RawPayload       string
	DeliveryStatus   string // Pending | Sent | Failed
	DeliveryAttempts int
	LastAttemptAt    sql.NullTime
	LastError        string
	SentAt           sql.NullTime
	BatchID          sql.NullString
	CreatedAt        time.Time
}

// NewPunch is the value the collectors hand to the DB layer for insertion. It is
// the pre-persistence shape: no id/status yet (the row defaults handle those).
type NewPunch struct {
	DeviceID       string // may be "" for an unmapped push SN
	DeviceSerial   string
	EmployeeNumber string
	PunchState     int
	LogType        string
	EventTime      time.Time
	VerifyMode     int
	Source         string
	RawPayload     string
}
