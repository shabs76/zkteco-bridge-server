package punchdb

import (
	"strings"
	"time"

	gendb "ismila/erp/zkbio/database/gendb"
	milastructs "ismila/erp/zkbio/mila_structs"
)

// MarkSent flips a set of punches to Sent in one statement — called after HR
// returns 200 for the batch that carried them. sentAt and batchID are recorded
// for the audit trail.
func MarkSent(ids []string, batchID string, sentAt time.Time) (int64, *milastructs.AnswerResponse) {
	if len(ids) == 0 {
		r := milastructs.Ok("nothing to mark", nil)
		return 0, &r
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	q := `UPDATE attendance_punches
	      SET delivery_status = 'Sent', sent_at = ?, batch_id = ?, last_error = '',
	          delivery_attempts = delivery_attempts + 1, last_attempt_at = ?
	      WHERE id IN (` + placeholders + `)`
	args := make([]any, 0, len(ids)+3)
	args = append(args, sentAt, batchID, sentAt)
	for _, id := range ids {
		args = append(args, id)
	}
	return gendb.UpdateGeneral(q, args...)
}

// MarkAttemptFailed records a failed delivery attempt without giving up: the
// punch stays Pending, its attempt counter increments, and the error is stored.
// If maxAttempts > 0 and the budget is now exhausted, the punch flips to Failed
// so it stops being retried (and shows up for an operator to investigate).
func MarkAttemptFailed(ids []string, errMsg string, at time.Time, maxAttempts int) (int64, *milastructs.AnswerResponse) {
	if len(ids) == 0 {
		r := milastructs.Ok("nothing to mark", nil)
		return 0, &r
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	statusExpr := "'Pending'"
	if maxAttempts > 0 {
		// Once attempts+1 reaches the cap, park the punch as Failed.
		statusExpr = "IF(delivery_attempts + 1 >= ?, 'Failed', 'Pending')"
	}
	q := `UPDATE attendance_punches
	      SET delivery_attempts = delivery_attempts + 1, last_attempt_at = ?, last_error = ?,
	          delivery_status = ` + statusExpr + `
	      WHERE id IN (` + placeholders + `)`
	args := make([]any, 0, len(ids)+3)
	args = append(args, at, truncate(errMsg, 255))
	if maxAttempts > 0 {
		args = append(args, maxAttempts)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	return gendb.UpdateGeneral(q, args...)
}

// RequeueFailed flips Failed punches for a device back to Pending (operator
// action after fixing a credential or a shift setup), resetting the attempt
// counter so they get a fresh retry budget.
func RequeueFailed(deviceID string) (int64, *milastructs.AnswerResponse) {
	return gendb.UpdateGeneral(
		`UPDATE attendance_punches
		 SET delivery_status = 'Pending', delivery_attempts = 0, last_error = ''
		 WHERE device_id = ? AND delivery_status = 'Failed'`, deviceID)
}

// PurgeDelivered deletes Sent punches older than the cutoff (retention). Pending
// and Failed rows are never purged — undelivered data is never silently dropped.
func PurgeDelivered(before time.Time) (int64, *milastructs.AnswerResponse) {
	return gendb.UpdateGeneral(
		`DELETE FROM attendance_punches WHERE delivery_status = 'Sent' AND sent_at < ?`, before)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
