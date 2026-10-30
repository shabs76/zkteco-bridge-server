package dashboardstructs

// DeviceHealth is a per-device summary line on the dashboard.
type DeviceHealth struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	SerialNumber string  `json:"serialNumber"`
	Mode         string  `json:"mode"`
	IsActive     bool    `json:"isActive"`
	HRCredential bool    `json:"hrCredentialSet"`
	LastSeenAt   *string `json:"lastSeenAt"`
	Pending      int64   `json:"pending"`
	Failed       int64   `json:"failed"`
	Online       bool    `json:"online"` // seen within the staleness window
}

// DashboardResponse is the gateway status overview.
type DashboardResponse struct {
	Devices        int            `json:"devices"`
	ActiveDevices  int            `json:"activeDevices"`
	PendingPunches int64          `json:"pendingPunches"`
	FailedPunches  int64          `json:"failedPunches"`
	SentPunches    int64          `json:"sentPunches"`
	TotalPunches   int64          `json:"totalPunches"`
	Employees      int64          `json:"employees"` // rows in the local name registry
	OldestPending  *string        `json:"oldestPending"`
	LastDelivery   *string        `json:"lastDelivery"`
	DeviceHealth   []DeviceHealth `json:"deviceHealth"`
}
