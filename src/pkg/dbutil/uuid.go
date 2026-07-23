package dbutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewUUID returns a random RFC-4122 v4 UUID string.
//
// The gateway generates device UUIDs in the app (unlike the HR service, which
// leans on MariaDB's DEFAULT UUID()). Devices have no single unique natural key
// to re-select by after insert — a pull-only unit may have no serial yet — so an
// app-generated id, inserted explicitly and returned directly, is simpler and
// avoids a fragile insert-then-reselect dance. Punches keep DEFAULT UUID().
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is catastrophic and unrecoverable here.
		panic(fmt.Sprintf("dbutil: rand: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}
