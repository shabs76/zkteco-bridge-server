package employeestructs

import "time"

// EmployeeResponse is the public view of an employee row.
type EmployeeResponse struct {
	ID             string    `json:"id"`
	EmployeeNumber string    `json:"employeeNumber"`
	Name           string    `json:"name"`
	CardNumber     string    `json:"cardNumber"`
	Privilege      int       `json:"privilege"`
	DeviceUserID   string    `json:"deviceUserId"`
	Source         string    `json:"source"`
	LastDeviceID   *string   `json:"lastDeviceId"`
	IsActive       bool      `json:"isActive"`
	LastSeenAt     *string   `json:"lastSeenAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ToEmployeeResponse maps the DB model to the response shape.
func ToEmployeeResponse(e *Employee) EmployeeResponse {
	r := EmployeeResponse{
		ID:             e.ID,
		EmployeeNumber: e.EmployeeNumber,
		Name:           e.Name,
		CardNumber:     e.CardNumber,
		Privilege:      e.Privilege,
		DeviceUserID:   e.DeviceUserID,
		Source:         e.Source,
		IsActive:       e.IsActive,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
	if e.LastDeviceID.Valid {
		r.LastDeviceID = &e.LastDeviceID.String
	}
	if e.LastSeenAt.Valid {
		s := e.LastSeenAt.Time.Format(time.RFC3339)
		r.LastSeenAt = &s
	}
	return r
}

// SyncResult reports what one device employee-sync did.
type SyncResult struct {
	DeviceID string `json:"deviceId"`
	Read     int    `json:"read"`    // users the device reported
	Created  int    `json:"created"` // new employee rows
	Updated  int    `json:"updated"` // existing rows refreshed
	Skipped  int    `json:"skipped"` // unusable rows (no PIN)
}

// ImportResult reports what a CSV import did. Errors is capped so one bad file
// cannot produce an unbounded response.
type ImportResult struct {
	Rows     int      `json:"rows"`
	Created  int      `json:"created"`
	Updated  int      `json:"updated"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors"`
	Truncate bool     `json:"errorsTruncated"`
}
