// Package employeedb is the data-access layer for the local employee (device
// user) registry. It builds SQL + args and calls gendb; it holds no business
// logic.
package employeedb

import (
	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
	dbutil "ismila/erp/zkbio/pkg/dbutil"
)

// Upsert outcomes, so callers can report created-vs-updated counts.
const (
	Created = iota
	Updated
	Failed
)

// UpsertFromDevice records what a device reported about one enrolled user.
//
// The name precedence rule lives in this statement: a row whose name was set by
// an operator (source 'import' or 'manual') keeps that name, because the
// operator's correction is authoritative and the device's flash may still hold
// the stale spelling. Every other column is refreshed from the device, and a
// device name is still accepted when the stored one is empty.
//
// An empty device name never blanks a stored one — many units are enrolled with
// a PIN and no name at all, and a sync must not erase what push or an import
// already established.
func UpsertFromDevice(u *employeestructs.DeviceUser, deviceID, source string) (int, *milastructs.AnswerResponse) {
	const q = `
		INSERT INTO employees
			(id, employee_number, name, card_number, privilege, device_user_id,
			 source, last_device_id, last_seen_at)
		VALUES (UUID(), ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NOW())
		ON DUPLICATE KEY UPDATE
			name = CASE
				WHEN source IN ('import','manual') AND name <> '' THEN name
				WHEN VALUES(name) <> ''                           THEN VALUES(name)
				ELSE name
			END,
			card_number    = IF(VALUES(card_number) <> '', VALUES(card_number), card_number),
			privilege      = VALUES(privilege),
			device_user_id = IF(VALUES(device_user_id) <> '', VALUES(device_user_id), device_user_id),
			last_device_id = COALESCE(VALUES(last_device_id), last_device_id),
			last_seen_at   = NOW()`
	n, resp := gendb.SaveGeneral(q,
		u.EmployeeNumber, u.Name, u.CardNumber, u.Privilege, u.DeviceUserID,
		source, deviceID,
	)
	if !resp.IsSuccess {
		return Failed, resp
	}
	// MySQL/MariaDB RowsAffected for INSERT .. ON DUPLICATE KEY UPDATE: 1 = a new
	// row, 2 = an existing row actually changed, 0 = matched but identical. Only
	// 1 is a creation; both other cases mean the employee already existed.
	if n == 1 {
		return Created, resp
	}
	return Updated, resp
}

// UpsertFromOperator writes a row an operator supplied (CSV import or the API).
// Unlike the device path this is authoritative: the given name always wins, and
// the row is stamped with the operator source so later device syncs leave it be.
func UpsertFromOperator(e *employeestructs.Employee, source string) (int, *milastructs.AnswerResponse) {
	const q = `
		INSERT INTO employees
			(id, employee_number, name, card_number, privilege, source, is_active)
		VALUES (UUID(), ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name        = VALUES(name),
			card_number = VALUES(card_number),
			privilege   = VALUES(privilege),
			is_active   = VALUES(is_active),
			source      = VALUES(source)`
	n, resp := gendb.SaveGeneral(q,
		e.EmployeeNumber, e.Name, e.CardNumber, e.Privilege, source, e.IsActive,
	)
	if !resp.IsSuccess {
		if dbutil.IsForeignKeyError(resp.Message) {
			r := milastructs.Fail("unknown device reference")
			return Failed, &r
		}
		return Failed, resp
	}
	if n == 1 {
		return Created, resp
	}
	return Updated, resp
}
