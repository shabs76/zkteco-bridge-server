package devicestructs

import "time"

// DeviceResponse is the safe public view of a device. It never includes the HR
// secret — only whether one is set and its display prefix.
type DeviceResponse struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	SerialNumber  string  `json:"serialNumber"`
	IPAddress     string  `json:"ipAddress"`
	Port          int     `json:"port"`
	Mode          string  `json:"mode"`
	Timezone      string  `json:"timezone"`
	HRDeviceID    string  `json:"hrDeviceId"`
	HRKeyPrefix   string  `json:"hrKeyPrefix"`
	HRCredential  bool    `json:"hrCredentialSet"`
	PunchStateMap string  `json:"punchStateMap"`
	IsActive      bool    `json:"isActive"`
	PollIntervalS int     `json:"pollIntervalSeconds"`
	LastSeenAt    *string `json:"lastSeenAt"`
	LastPullAt    *string `json:"lastPullAt"`
	LastPullError string  `json:"lastPullError"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToDeviceResponse maps the DB model to the safe response shape.
func ToDeviceResponse(d *ZKDevice) DeviceResponse {
	r := DeviceResponse{
		ID:            d.ID,
		Name:          d.Name,
		SerialNumber:  d.SerialNumber,
		IPAddress:     d.IPAddress,
		Port:          d.Port,
		Mode:          d.Mode,
		Timezone:      d.Timezone,
		HRDeviceID:    d.HRDeviceID,
		HRKeyPrefix:   d.HRKeyPrefix,
		HRCredential:  d.HasHRCredential(),
		PunchStateMap: d.PunchStateMap,
		IsActive:      d.IsActive,
		PollIntervalS: d.PollIntervalS,
		LastPullError: d.LastPullError,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
	if d.LastSeenAt.Valid {
		s := d.LastSeenAt.Time.Format(time.RFC3339)
		r.LastSeenAt = &s
	}
	if d.LastPullAt.Valid {
		s := d.LastPullAt.Time.Format(time.RFC3339)
		r.LastPullAt = &s
	}
	return r
}
