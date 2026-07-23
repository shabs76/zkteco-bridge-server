// Package batchdb records one row per HR forward attempt for monitoring/audit.
package batchdb

import (
	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
	dbutil "ismila/erp/zkbio/pkg/dbutil"
)

// Batch is the audit record of a single delivery attempt to HR.
type Batch struct {
	DeviceID    string
	HRDeviceID  string
	PunchCount  int
	HTTPStatus  int
	HRTotal     int
	HRAccepted  int
	HRIgnored   int
	HRFailed    int
	HRDuplicate int
	Success     bool
	Error       string
}

// Insert writes a delivery-batch audit row and returns its id.
func Insert(b *Batch) (string, *milastructs.AnswerResponse) {
	id := dbutil.NewUUID()
	const q = `
		INSERT INTO delivery_batches
			(id, device_id, hr_device_id, punch_count, http_status,
			 hr_total, hr_accepted, hr_ignored, hr_failed, hr_duplicate, success, error)
		VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, resp := gendb.SaveGeneral(q,
		id, b.DeviceID, b.HRDeviceID, b.PunchCount, b.HTTPStatus,
		b.HRTotal, b.HRAccepted, b.HRIgnored, b.HRFailed, b.HRDuplicate, b.Success, truncate(b.Error, 512),
	)
	if !resp.IsSuccess {
		return "", resp
	}
	return id, resp
}

// PurgeOld deletes batch audit rows older than the given number of days.
func PurgeOld(days int) {
	_, _ = gendb.UpdateGeneral(
		`DELETE FROM delivery_batches WHERE created_at < (NOW() - INTERVAL ? DAY)`, days)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
