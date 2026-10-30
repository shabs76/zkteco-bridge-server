// Package iclock implements the parsing and reply helpers for the ZKTeco ADMS /
// "push SDK" protocol — the HTTP protocol newer devices use to push attendance
// to a server (the same mechanism BioTime uses). The device is configured with
// this gateway as its server; it then registers and POSTs tab-separated
// attendance rows to /iclock/cdata.
package iclock

import (
	"strconv"
	"strings"
	"time"
)

// AttRecord is one punch parsed from an ADMS ATTLOG push.
type AttRecord struct {
	PIN        string    // device user id == HR employee_number
	Time       time.Time // parsed in the device/gateway location
	State      int       // punch state (direction)
	VerifyMode int       // verification method
	Raw        string    // the original line, kept for troubleshooting
}

// timeLayouts are the datetime formats ADMS firmwares emit.
var timeLayouts = []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05"}

// ParseAttlog parses an ATTLOG push body. Each non-empty line is one punch with
// tab-separated fields: PIN, timestamp, status(state), verify, workcode, ...
// Malformed lines are skipped; parsing is deliberately lenient because firmwares
// vary in how many trailing columns they send.
func ParseAttlog(body string, loc *time.Location) []AttRecord {
	if loc == nil {
		loc = time.Local
	}
	var out []AttRecord
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		pin := strings.TrimSpace(fields[0])
		t, ok := parseTime(strings.TrimSpace(fields[1]), loc)
		if pin == "" || !ok {
			continue
		}
		rec := AttRecord{PIN: pin, Time: t, Raw: line}
		if len(fields) >= 3 {
			rec.State = atoi(fields[2])
		}
		if len(fields) >= 4 {
			rec.VerifyMode = atoi(fields[3])
		}
		out = append(out, rec)
	}
	return out
}

func parseTime(s string, loc *time.Location) (time.Time, bool) {
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// RegistryReply builds the response to a device's initial GET /iclock/cdata
// handshake. The device keeps pushing as long as it gets a 200 with a
// well-formed option block; these values ask for real-time attendance delivery.
func RegistryReply(sn string) string {
	var b strings.Builder
	b.WriteString("GET OPTION FROM: " + sn + "\n")
	b.WriteString("Stamp=9999\n")
	b.WriteString("OpStamp=9999\n")
	b.WriteString("ErrorDelay=30\n")
	b.WriteString("Delay=10\n")
	b.WriteString("TransTimes=00:00;14:00\n")
	b.WriteString("TransInterval=1\n")
	b.WriteString("TransFlag=1111000000\n")
	b.WriteString("Realtime=1\n")
	b.WriteString("Encrypt=0\n")
	return b.String()
}

// UserRecord is one enrolled user parsed from an ADMS user push.
type UserRecord struct {
	PIN        string // device user id == HR employee_number
	Name       string
	CardNumber string
	Privilege  int
	Raw        string
}

// ParseUserInfo parses the user rows an ADMS device uploads.
//
// Names do NOT ride along on an ATTLOG punch line — the punch carries only the
// PIN. The device sends its user table separately, as USER rows on the OPERLOG
// table (or on a USERINFO table, depending on firmware), typically right after
// registration and again whenever an enrolment changes. Capturing those rows is
// what lets the gateway show a name against a pushed punch.
//
// Both shapes are accepted, with or without the leading USER token:
//
//	USER PIN=1	Name=Asha Mwanga	Pri=0	Passwd=	Card=12345	Grp=1	TZ=0
//	PIN=1	Name=Asha Mwanga	Card=12345
//
// Fields are tab-separated key=value pairs. Unknown keys are ignored and any
// line without a PIN is skipped, because OPERLOG interleaves genuine operator
// log rows (OPLOG ...) with the user rows.
func ParseUserInfo(body string) []UserRecord {
	var out []UserRecord
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Strip an optional leading "USER" token; anything else leading (OPLOG,
		// FP, FACE, BIODATA...) is a row this gateway does not consume.
		rest := trimmed
		if head, tail, ok := strings.Cut(trimmed, " "); ok {
			switch strings.ToUpper(head) {
			case "USER":
				rest = tail
			default:
				if !strings.HasPrefix(strings.ToUpper(head), "PIN=") {
					continue
				}
			}
		} else if !strings.HasPrefix(strings.ToUpper(trimmed), "PIN=") {
			continue
		}

		rec := UserRecord{Raw: line}
		for _, field := range strings.Split(rest, "\t") {
			key, value, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.ToUpper(strings.TrimSpace(key)) {
			case "PIN":
				rec.PIN = value
			case "NAME":
				rec.Name = value
			case "CARD":
				// "0" means no card enrolled; keep it empty so it never
				// overwrites a card number set from an import.
				if value != "0" {
					rec.CardNumber = value
				}
			case "PRI":
				rec.Privilege = atoi(value)
			}
		}
		if rec.PIN == "" {
			continue
		}
		out = append(out, rec)
	}
	return out
}
