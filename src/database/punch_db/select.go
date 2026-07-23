package punchdb

import (
	"database/sql"
	"strings"
	"time"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	punchstructs "ismila/erp/zkbio/mila_structs/punch_structs"
)

const punchColumns = `
	id, device_id, device_serial, employee_number, punch_state, log_type, event_time,
	verify_mode, source, latitude, longitude, raw_payload,
	delivery_status, delivery_attempts, last_attempt_at, last_error, sent_at, batch_id, created_at`

func scanPunch(rows *sql.Rows) (*punchstructs.Punch, error) {
	var p punchstructs.Punch
	err := rows.Scan(
		&p.ID, &p.DeviceID, &p.DeviceSerial, &p.EmployeeNumber, &p.PunchState, &p.LogType, &p.EventTime,
		&p.VerifyMode, &p.Source, &p.Latitude, &p.Longitude, &p.RawPayload,
		&p.DeliveryStatus, &p.DeliveryAttempts, &p.LastAttemptAt, &p.LastError, &p.SentAt, &p.BatchID, &p.CreatedAt,
	)
	return &p, err
}

// SelectPendingByDevice returns up to `limit` Pending punches for one device,
// oldest first, so the forwarder delivers them in chronological order. maxAttempts
// > 0 filters out punches that have already exhausted their retry budget.
func SelectPendingByDevice(deviceID string, limit, maxAttempts int) ([]*punchstructs.Punch, *milastructs.AnswerResponse) {
	q := `SELECT ` + punchColumns + ` FROM attendance_punches
	      WHERE device_id = ? AND delivery_status = 'Pending'`
	args := []any{deviceID}
	if maxAttempts > 0 {
		q += ` AND delivery_attempts < ?`
		args = append(args, maxAttempts)
	}
	q += ` ORDER BY event_time ASC, created_at ASC LIMIT ?`
	args = append(args, limit)

	var out []*punchstructs.Punch
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			p, err := scanPunch(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	}, q, args...)
	return out, resp
}

// DevicesWithPending returns the ids of devices that currently have deliverable
// Pending punches, so the forwarder only wakes devices that need it.
func DevicesWithPending(maxAttempts int) ([]string, *milastructs.AnswerResponse) {
	q := `SELECT DISTINCT device_id FROM attendance_punches
	      WHERE delivery_status = 'Pending' AND device_id IS NOT NULL`
	var args []any
	if maxAttempts > 0 {
		q += ` AND delivery_attempts < ?`
		args = append(args, maxAttempts)
	}
	var out []string
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			var id sql.NullString
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if id.Valid {
				out = append(out, id.String)
			}
		}
		return nil
	}, q, args...)
	return out, resp
}

// PunchFilter narrows a ListPunches query. Empty fields are ignored.
type PunchFilter struct {
	DeviceID       string
	EmployeeNumber string
	Status         string // Pending | Sent | Failed
	From           *time.Time
	To             *time.Time
}

// ListPunches returns a page of punches (newest first) plus the total count.
func ListPunches(f PunchFilter, page, pageSize int) ([]*punchstructs.Punch, int64, *milastructs.AnswerResponse) {
	where := []string{"1=1"}
	args := []any{}
	if f.DeviceID != "" {
		where = append(where, "device_id = ?")
		args = append(args, f.DeviceID)
	}
	if f.EmployeeNumber != "" {
		where = append(where, "employee_number = ?")
		args = append(args, f.EmployeeNumber)
	}
	if f.Status != "" {
		where = append(where, "delivery_status = ?")
		args = append(args, f.Status)
	}
	if f.From != nil {
		where = append(where, "event_time >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		where = append(where, "event_time <= ?")
		args = append(args, *f.To)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := gendb.SelectRow(`SELECT COUNT(*) FROM attendance_punches WHERE `+cond, args...).Scan(&total); err != nil {
		r := milastructs.Err("count failed: " + err.Error())
		return nil, 0, &r
	}

	offset := (page - 1) * pageSize
	pageArgs := append(append([]any{}, args...), pageSize, offset)
	var out []*punchstructs.Punch
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			p, err := scanPunch(rows)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	}, `SELECT `+punchColumns+` FROM attendance_punches WHERE `+cond+
		` ORDER BY event_time DESC, created_at DESC LIMIT ? OFFSET ?`, pageArgs...)
	return out, total, resp
}

// StatusCounts holds gateway-wide delivery tallies.
type StatusCounts struct {
	Pending, Sent, Failed, Total int64
}

// CountByStatus returns the delivery tallies across all punches.
func CountByStatus() (StatusCounts, *milastructs.AnswerResponse) {
	var c StatusCounts
	resp := gendb.SelectGeneral(func(rows *sql.Rows) error {
		for rows.Next() {
			var status string
			var n int64
			if err := rows.Scan(&status, &n); err != nil {
				return err
			}
			switch status {
			case "Pending":
				c.Pending = n
			case "Sent":
				c.Sent = n
			case "Failed":
				c.Failed = n
			}
			c.Total += n
		}
		return nil
	}, `SELECT delivery_status, COUNT(*) FROM attendance_punches GROUP BY delivery_status`)
	return c, resp
}

// CountPendingFailedForDevice returns Pending and Failed counts for one device.
func CountPendingFailedForDevice(deviceID string) (pending, failed int64) {
	_ = gendb.SelectRow(
		`SELECT
			COALESCE(SUM(delivery_status = 'Pending'), 0),
			COALESCE(SUM(delivery_status = 'Failed'), 0)
		 FROM attendance_punches WHERE device_id = ?`, deviceID).Scan(&pending, &failed)
	return pending, failed
}

// OldestPending returns the event_time of the oldest undelivered punch, if any.
func OldestPending() *time.Time {
	var t sql.NullTime
	_ = gendb.SelectRow(
		`SELECT MIN(event_time) FROM attendance_punches WHERE delivery_status = 'Pending'`).Scan(&t)
	if t.Valid {
		return &t.Time
	}
	return nil
}

// LastDelivery returns the most recent successful delivery time, if any.
func LastDelivery() *time.Time {
	var t sql.NullTime
	_ = gendb.SelectRow(`SELECT MAX(sent_at) FROM attendance_punches WHERE sent_at IS NOT NULL`).Scan(&t)
	if t.Valid {
		return &t.Time
	}
	return nil
}
