package devicestructs

// CreateDeviceRequest registers a physical ZKTeco unit with the gateway.
// The HR credential can be supplied here or set later via SetHRCredentialRequest.
type CreateDeviceRequest struct {
	Name         string `json:"name" binding:"required"`
	SerialNumber string `json:"serialNumber"` // required for push mode; how the device identifies itself
	IPAddress    string `json:"ipAddress"`    // required for pull mode
	Port         int    `json:"port"`         // default 4370
	CommPassword int    `json:"commPassword"` // device COMM key (0 = none)
	Mode         string `json:"mode"`         // pull | push | both (default both)
	Timezone     string `json:"timezone"`     // optional per-device TZ for naive timestamps

	// HR credential (per-device). Obtain by POST /devices on the HR service.
	HRDeviceID  string `json:"hrDeviceId"`
	HRDeviceKey string `json:"hrDeviceKey"` // plaintext X-Device-Key; encrypted at rest, never returned

	PunchStateMap string `json:"punchStateMap"` // optional JSON: {"0":"IN","1":"OUT"}
	PollIntervalS int    `json:"pollIntervalSeconds"`
}

// UpdateDeviceRequest is a partial update — only non-nil fields are applied.
type UpdateDeviceRequest struct {
	Name          *string `json:"name"`
	SerialNumber  *string `json:"serialNumber"`
	IPAddress     *string `json:"ipAddress"`
	Port          *int    `json:"port"`
	CommPassword  *int    `json:"commPassword"`
	Mode          *string `json:"mode"`
	Timezone      *string `json:"timezone"`
	PunchStateMap *string `json:"punchStateMap"`
	PollIntervalS *int    `json:"pollIntervalSeconds"`
	IsActive      *bool   `json:"isActive"`
}

// SetHRCredentialRequest sets/rotates the HR credential for a device.
type SetHRCredentialRequest struct {
	HRDeviceID  string `json:"hrDeviceId" binding:"required"`
	HRDeviceKey string `json:"hrDeviceKey" binding:"required"`
}
