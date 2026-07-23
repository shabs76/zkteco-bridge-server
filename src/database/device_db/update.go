package devicedb

import (
	"time"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
)

// UpdateDevice applies a partial update. Only non-nil fields in req are changed.
func UpdateDevice(id string, req *devicestructs.UpdateDeviceRequest) (int64, *milastructs.AnswerResponse) {
	set := []string{}
	args := []any{}
	add := func(frag string, v any) { set = append(set, frag); args = append(args, v) }

	if req.Name != nil {
		add("name = ?", *req.Name)
	}
	if req.SerialNumber != nil {
		set = append(set, "serial_number = NULLIF(?, '')")
		args = append(args, *req.SerialNumber)
	}
	if req.IPAddress != nil {
		add("ip_address = ?", *req.IPAddress)
	}
	if req.Port != nil {
		add("port = ?", *req.Port)
	}
	if req.CommPassword != nil {
		add("comm_password = ?", *req.CommPassword)
	}
	if req.Mode != nil {
		add("mode = ?", *req.Mode)
	}
	if req.Timezone != nil {
		add("timezone = ?", *req.Timezone)
	}
	if req.PunchStateMap != nil {
		add("punch_state_map = ?", *req.PunchStateMap)
	}
	if req.PollIntervalS != nil {
		add("poll_interval_s = ?", *req.PollIntervalS)
	}
	if req.IsActive != nil {
		add("is_active = ?", *req.IsActive)
	}
	if len(set) == 0 {
		r := milastructs.Ok("no changes", nil)
		return 0, &r
	}

	q := "UPDATE zk_devices SET " + joinComma(set) + " WHERE id = ?"
	args = append(args, id)
	return gendb.UpdateGeneral(q, args...)
}

// SetHRCredential stores a device's encrypted HR key + display prefix + id.
func SetHRCredential(id, hrDeviceID string, keyEnc []byte, prefix string) (int64, *milastructs.AnswerResponse) {
	const q = `UPDATE zk_devices SET hr_device_id = ?, hr_device_key_enc = ?, hr_key_prefix = ? WHERE id = ?`
	return gendb.UpdateGeneral(q, hrDeviceID, keyEnc, prefix, id)
}

// DeleteDevice removes a device. Punches keep their history (FK ON DELETE SET NULL).
func DeleteDevice(id string) (int64, *milastructs.AnswerResponse) {
	return gendb.UpdateGeneral(`DELETE FROM zk_devices WHERE id = ?`, id)
}

// TouchLastSeen records that we just heard from a device (pull connect or push).
func TouchLastSeen(id string, t time.Time) {
	_, _ = gendb.UpdateGeneral(`UPDATE zk_devices SET last_seen_at = ? WHERE id = ?`, t, id)
}

// SetSerialIfEmpty records a serial learned from the device during a pull, only
// if we did not already have one (does not clobber an operator-set value).
func SetSerialIfEmpty(id, serial string) {
	if serial == "" {
		return
	}
	_, _ = gendb.UpdateGeneral(
		`UPDATE zk_devices SET serial_number = ? WHERE id = ? AND (serial_number IS NULL OR serial_number = '')`,
		serial, id)
}

// UpdatePullResult records the outcome of a pull cycle: success clears the error
// and advances last_pull_at (and the cursor if a newer event was seen); failure
// records the error message.
func UpdatePullResult(id string, at time.Time, cursor *time.Time, errMsg string) {
	if errMsg != "" {
		_, _ = gendb.UpdateGeneral(
			`UPDATE zk_devices SET last_pull_at = ?, last_pull_error = ? WHERE id = ?`,
			at, truncate(errMsg, 255), id)
		return
	}
	if cursor != nil {
		_, _ = gendb.UpdateGeneral(
			`UPDATE zk_devices SET last_pull_at = ?, last_pull_error = '',
			 last_cursor_time = GREATEST(COALESCE(last_cursor_time, '1970-01-01'), ?) WHERE id = ?`,
			at, *cursor, id)
		return
	}
	_, _ = gendb.UpdateGeneral(
		`UPDATE zk_devices SET last_pull_at = ?, last_pull_error = '' WHERE id = ?`, at, id)
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
