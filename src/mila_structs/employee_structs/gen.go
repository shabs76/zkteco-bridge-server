// Package employeestructs holds the gateway's local employee (device user)
// registry model and its request/response shapes, split gen/req/resp the same
// way the device and punch domains are.
package employeestructs

import (
	"database/sql"
	"time"
)

// Source values for Employee.Source. The two device sources are refreshed on
// every sync; the two operator sources are sticky for the name (see
// employeedb.UpsertFromDevice).
const (
	SourcePull   = "pull"
	SourcePush   = "push"
	SourceImport = "import"
	SourceManual = "manual"
)

// Employee mirrors a row of employees — one person, keyed by the device PIN,
// which is also the HR employee_number.
type Employee struct {
	ID             string
	EmployeeNumber string
	Name           string
	CardNumber     string
	Privilege      int
	DeviceUserID   string
	Source         string
	LastDeviceID   sql.NullString
	IsActive       bool
	LastSeenAt     sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// DeviceUser is what a device reports about one enrolled user, from either the
// pull protocol (pkg/zk) or an ADMS push (pkg/iclock). It is the pre-persistence
// shape handed to the DB layer, deliberately narrower than Employee: a device
// knows nothing about is_active or the operator's corrections.
type DeviceUser struct {
	EmployeeNumber string // device PIN / user id
	Name           string // may be "" — plenty of devices are enrolled without names
	CardNumber     string
	Privilege      int
	DeviceUserID   string // device-internal uid, informational
}
