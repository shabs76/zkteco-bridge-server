// Package dbutil classifies MariaDB/MySQL driver errors so callers can map them
// to the right HTTP status (duplicate key -> 409, FK violation -> 400) without
// importing the driver everywhere. Mirrors the HR service's dbutil.
package dbutil

import "strings"

// IsDuplicateKeyError reports whether an error message is a MySQL duplicate-key
// violation (error 1062). We match on message text because the *_db layer often
// only has the stringified error to work with.
func IsDuplicateKeyError(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "error 1062") ||
		strings.Contains(m, "duplicate entry") ||
		strings.Contains(m, "duplicate key")
}

// IsForeignKeyError reports whether an error is a foreign-key constraint failure
// (errors 1451 / 1452).
func IsForeignKeyError(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "error 1451") ||
		strings.Contains(m, "error 1452") ||
		strings.Contains(m, "foreign key constraint fails")
}
