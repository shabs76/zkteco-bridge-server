package punchstructs

import "time"

// PunchResponse is the public view of a stored punch (operator troubleshooting).
type PunchResponse struct {
	ID             string  `json:"id"`
	DeviceID       *string `json:"deviceId"`
	DeviceSerial   string  `json:"deviceSerial"`
	EmployeeNumber string  `json:"employeeNumber"`
	// EmployeeName is filled from the local employee registry when a name is
	// known for this number. It is never sent to HR (see pkg/hrclient) — it is
	// here so an operator can tell who a punch belongs to.
	EmployeeName     string    `json:"employeeName"`
	PunchState       int       `json:"punchState"`
	LogType          string    `json:"logType"`
	EventTime        time.Time `json:"eventTime"`
	VerifyMode       int       `json:"verifyMode"`
	Source           string    `json:"source"`
	DeliveryStatus   string    `json:"deliveryStatus"`
	DeliveryAttempts int       `json:"deliveryAttempts"`
	LastError        string    `json:"lastError"`
	SentAt           *string   `json:"sentAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

// ToPunchResponse maps the DB model to the response shape.
func ToPunchResponse(p *Punch) PunchResponse {
	r := PunchResponse{
		ID:               p.ID,
		DeviceSerial:     p.DeviceSerial,
		EmployeeNumber:   p.EmployeeNumber,
		PunchState:       p.PunchState,
		LogType:          p.LogType,
		EventTime:        p.EventTime,
		VerifyMode:       p.VerifyMode,
		Source:           p.Source,
		DeliveryStatus:   p.DeliveryStatus,
		DeliveryAttempts: p.DeliveryAttempts,
		LastError:        p.LastError,
		CreatedAt:        p.CreatedAt,
	}
	if p.DeviceID.Valid {
		r.DeviceID = &p.DeviceID.String
	}
	if p.SentAt.Valid {
		s := p.SentAt.Time.Format(time.RFC3339)
		r.SentAt = &s
	}
	return r
}
