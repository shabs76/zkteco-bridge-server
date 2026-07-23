package devicedb

import (
	"database/sql"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
)

// deviceColumns is the shared SELECT list. COALESCE(serial_number,”) so the
// nullable SN scans into a plain string.
const deviceColumns = `
	id, name, COALESCE(serial_number,''), ip_address, port, comm_password, mode, timezone,
	hr_device_id, hr_device_key_enc, hr_key_prefix, punch_state_map,
	is_active, poll_interval_s, last_seen_at, last_pull_at, last_pull_error, last_cursor_time,
	created_at, updated_at`

func scanDevice(rows *sql.Rows) (*devicestructs.ZKDevice, error) {
	var d devicestructs.ZKDevice
	err := rows.Scan(
		&d.ID, &d.Name, &d.SerialNumber, &d.IPAddress, &d.Port, &d.CommPassword, &d.Mode, &d.Timezone,
		&d.HRDeviceID, &d.HRDeviceKeyEnc, &d.HRKeyPrefix, &d.PunchStateMap,
		&d.IsActive, &d.PollIntervalS, &d.LastSeenAt, &d.LastPullAt, &d.LastPullError, &d.LastCursorTime,
		&d.CreatedAt, &d.UpdatedAt,
	)
	return &d, err
}

// SelectDeviceByID returns a single device or (nil, nil) if not found.
func SelectDeviceByID(id string) (*devicestructs.ZKDevice, *milastructs.AnswerResponse) {
	var out *devicestructs.ZKDevice
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		if rows.Next() {
			d, err := scanDevice(rows)
			if err != nil {
				return err
			}
			out = d
		}
		return nil
	}, `SELECT `+deviceColumns+` FROM zk_devices WHERE id = ?`, id)
	return out, resp
}

// SelectDeviceIDBySerial resolves a device id from its serial number.
func SelectDeviceIDBySerial(serial string) (string, *milastructs.AnswerResponse) {
	var id string
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		if rows.Next() {
			return rows.Scan(&id)
		}
		return nil
	}, `SELECT id FROM zk_devices WHERE serial_number = ? LIMIT 1`, serial)
	return id, resp
}

// SelectActiveDevices returns all active devices whose mode includes the given
// capability ("pull" or "push"). Pass "" for all active devices.
func SelectActiveDevices(capability string) ([]*devicestructs.ZKDevice, *milastructs.AnswerResponse) {
	q := `SELECT ` + deviceColumns + ` FROM zk_devices WHERE is_active = TRUE`
	var args []any
	switch capability {
	case "pull":
		q += ` AND mode IN ('pull','both')`
	case "push":
		q += ` AND mode IN ('push','both')`
	}
	q += ` ORDER BY created_at ASC`

	var out []*devicestructs.ZKDevice
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			d, err := scanDevice(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return nil
	}, q, args...)
	return out, resp
}

// ListDevices returns a page of devices plus the total count.
func ListDevices(page, pageSize int) ([]*devicestructs.ZKDevice, int64, *milastructs.AnswerResponse) {
	var total int64
	if err := gendb.SelectRow(`SELECT COUNT(*) FROM zk_devices`).Scan(&total); err != nil {
		r := milastructs.Err("count failed: " + err.Error())
		return nil, 0, &r
	}
	offset := (page - 1) * pageSize
	var out []*devicestructs.ZKDevice
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			d, err := scanDevice(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return nil
	}, `SELECT `+deviceColumns+` FROM zk_devices ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, offset)
	return out, total, resp
}
