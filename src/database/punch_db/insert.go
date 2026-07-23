// Package punchdb is the data-access layer for stored punches and their delivery
// state.
package punchdb

import (
	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
	dbutil "ismila/erp/zkbio/pkg/dbutil"
)

// InsertOutcome is the result of trying to store one punch.
type InsertOutcome int

const (
	// Inserted — a brand-new punch row was created (now Pending delivery).
	Inserted InsertOutcome = iota
	// Duplicate — this exact punch already exists; nothing was written.
	Duplicate
	// InsertError — an unexpected DB error occurred.
	InsertError
)

// InsertPunch stores a raw punch idempotently. A repeat of the same
// (device, employee, log_type, event_time) collapses to Duplicate and writes
// nothing. New rows start life as delivery_status = 'Pending'.
func InsertPunch(p *punchstructs.NewPunch) (InsertOutcome, *milastructs.AnswerResponse) {
	const q = `
		INSERT INTO attendance_punches
			(device_id, device_serial, employee_number, punch_state, log_type,
			 event_time, verify_mode, source, raw_payload)
		VALUES (NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`
	_, resp := gendb.SaveGeneral(q,
		p.DeviceID, p.DeviceSerial, p.EmployeeNumber, p.PunchState, p.LogType,
		p.EventTime, p.VerifyMode, p.Source, p.RawPayload,
	)
	if resp.IsSuccess {
		return Inserted, resp
	}
	if dbutil.IsDuplicateKeyError(resp.Message) {
		ok := milastructs.Ok("duplicate", nil)
		return Duplicate, &ok
	}
	return InsertError, resp
}
