package zk

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// AttRecord is one attendance punch read from a device.
type AttRecord struct {
	UserID     string    // enroll/user id on the device == HR employee_number
	State      int       // punch direction state (0 in, 1 out, ...)
	VerifyMode int       // how the user verified (fp/face/card/pwd) — informational
	Time       time.Time // decoded in the given location
}

// ReadAttendance disables the device, reads the attendance buffer, re-enables the
// device, and returns the decoded records. Times are interpreted in loc (the
// device reports naive local times).
func (c *Client) ReadAttendance(loc *time.Location) ([]AttRecord, error) {
	if loc == nil {
		loc = time.Local
	}
	c.disableDevice()
	defer c.enableDevice()

	if err := c.send(cmdAttlogRRQ, nil); err != nil {
		return nil, err
	}
	data, err := c.readData()
	if err != nil {
		return nil, err
	}
	return parseAttendance(data, loc)
}

// readData collects a data transfer that begins with either a direct DATA reply
// (small payloads) or a PREPARE_DATA reply announcing a total size followed by
// one or more DATA packets and a trailing ACK_OK.
func (c *Client) readData() ([]byte, error) {
	cmd, _, payload, err := c.recv()
	if err != nil {
		return nil, err
	}
	switch cmd {
	case cmdData:
		return payload, nil
	case cmdAckOK:
		return nil, nil // no records
	case cmdAckError:
		return nil, errors.New("zk: device returned error to attendance request")
	case cmdPrepareData:
		if len(payload) < 4 {
			return nil, errors.New("zk: short PREPARE_DATA header")
		}
		size := binary.LittleEndian.Uint32(payload[0:4])
		buf := make([]byte, 0, size)
		for uint32(len(buf)) < size {
			c2, _, p2, err := c.recv()
			if err != nil {
				return nil, err
			}
			switch c2 {
			case cmdData:
				buf = append(buf, p2...)
			case cmdAckOK:
				return buf, nil // some devices close early with ACK
			default:
				return nil, fmt.Errorf("zk: unexpected command %d during data transfer", c2)
			}
		}
		// Drain the trailing ACK_OK if present (best effort, short wait).
		_ = c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, _, _ = c.recv()
		return buf, nil
	default:
		return nil, fmt.Errorf("zk: unexpected reply command %d to attendance request", cmd)
	}
}

// parseAttendance decodes fixed-size attendance records. The 40-byte record is
// the modern, near-universal format; a 16-byte legacy format is handled too. An
// unrecognized stride is reported rather than silently mis-parsed.
func parseAttendance(data []byte, loc *time.Location) ([]AttRecord, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var stride int
	switch {
	case len(data)%40 == 0:
		stride = 40
	case len(data)%16 == 0:
		stride = 16
	default:
		return nil, fmt.Errorf("zk: attendance data length %d is not a supported record size (40 or 16)", len(data))
	}

	var recs []AttRecord
	for off := 0; off+stride <= len(data); off += stride {
		rec := data[off : off+stride]
		var r AttRecord
		if stride == 40 {
			// <H 24s B 4s B 8s>: uid, user_id, status(verify), timestamp, punch(state), reserved
			r.UserID = cstr(rec[2:26])
			r.VerifyMode = int(rec[26])
			r.Time = decodeTime(binary.LittleEndian.Uint32(rec[27:31]), loc)
			r.State = int(rec[31])
		} else {
			// 16-byte legacy: <H B 2s 4s B ...>: uid, verify, _, timestamp, state
			r.UserID = fmt.Sprintf("%d", binary.LittleEndian.Uint16(rec[0:2]))
			r.VerifyMode = int(rec[2])
			r.Time = decodeTime(binary.LittleEndian.Uint32(rec[5:9]), loc)
			r.State = int(rec[9])
		}
		if r.UserID == "" || r.Time.IsZero() {
			continue
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// decodeTime reverses the ZK 4-byte packed timestamp into a time in loc.
func decodeTime(t uint32, loc *time.Location) time.Time {
	sec := int(t % 60)
	t /= 60
	minute := int(t % 60)
	t /= 60
	hour := int(t % 24)
	t /= 24
	day := int(t%31) + 1
	t /= 31
	month := int(t%12) + 1
	t /= 12
	year := int(t) + 2000
	return time.Date(year, time.Month(month), day, hour, minute, sec, 0, loc)
}

func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}

// SerialNumber reads the device serial number via the options channel. Returns
// "" if the device does not answer (non-fatal).
func (c *Client) SerialNumber() string {
	return c.getParam("~SerialNumber")
}

// getParam requests a single device parameter and returns the value after '='.
func (c *Client) getParam(name string) string {
	if err := c.send(cmdOptionsRRQ, append([]byte(name), 0)); err != nil {
		return ""
	}
	cmd, _, payload, err := c.recv()
	if err != nil || cmd != cmdAckOK {
		return ""
	}
	s := cstr(payload)
	if i := strings.IndexByte(s, '='); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return ""
}
