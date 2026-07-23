// Package devicedb is the data-access layer for the device registry. It builds
// SQL + args and calls gendb; it holds no business logic.
package devicedb

import (
	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	dbutil "ismila/erp/zkbio/pkg/dbutil"
)

// InsertDevice inserts a new device row with an app-generated UUID and returns
// that id. keyEnc is the already-encrypted HR key (may be nil).
func InsertDevice(d *devicestructs.ZKDevice) (string, *milastructs.AnswerResponse) {
	id := dbutil.NewUUID()
	const q = `
		INSERT INTO zk_devices
			(id, name, serial_number, ip_address, port, comm_password, mode, timezone,
			 hr_device_id, hr_device_key_enc, hr_key_prefix, punch_state_map,
			 is_active, poll_interval_s)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, resp := gendb.SaveGeneral(q,
		id, d.Name, d.SerialNumber, d.IPAddress, d.Port, d.CommPassword, d.Mode, d.Timezone,
		d.HRDeviceID, d.HRDeviceKeyEnc, d.HRKeyPrefix, d.PunchStateMap,
		d.IsActive, d.PollIntervalS,
	)
	if !resp.IsSuccess {
		return "", resp
	}
	return id, resp
}

// UpsertBySerial resolves a device row by serial number, creating a minimal
// inactive row if the serial is unknown (used by push auto-registration). It
// returns the device id.
func UpsertBySerial(serial, defaultName string, autoActive bool) (string, *milastructs.AnswerResponse) {
	if id, _ := SelectDeviceIDBySerial(serial); id != "" {
		return id, nil
	}
	id := dbutil.NewUUID()
	const q = `
		INSERT INTO zk_devices (id, name, serial_number, mode, is_active)
		VALUES (?, ?, ?, 'push', ?)`
	_, resp := gendb.SaveGeneral(q, id, defaultName, serial, autoActive)
	if !resp.IsSuccess {
		// A concurrent push may have created it first — re-resolve on duplicate.
		if dbutil.IsDuplicateKeyError(resp.Message) {
			if existing, _ := SelectDeviceIDBySerial(serial); existing != "" {
				return existing, nil
			}
		}
		return "", resp
	}
	return id, resp
}
