package zk

import (
	"encoding/binary"
	"fmt"
)

// cmdUserTempRRQ (CMD_USERTEMP_RRQ) requests the enrolled-user table. The
// payload byte 0x05 selects the user records rather than fingerprint templates.
const cmdUserTempRRQ uint16 = 9

// User is one enrolled user read from a device.
type User struct {
	UID        int    // device-internal record id (NOT the PIN)
	UserID     string // enroll id / PIN == HR employee_number
	Name       string
	CardNumber string
	Privilege  int // 0 = user, 14 = admin
}

// ReadUsers reads the device's enrolled-user table, which is where the names
// live. Devices are frequently enrolled with a PIN and no name at all, so an
// empty Name is normal and not an error.
//
// Same best-effort caveat as ReadAttendance: this uses the traditional
// PREPARE_DATA transfer path, which most ZKTeco terminals speak. A device whose
// firmware only supports the buffered transfer returns nothing here; use push
// mode, where the device sends its user table to us instead.
func (c *Client) ReadUsers() ([]User, error) {
	c.disableDevice()
	defer c.enableDevice()

	if err := c.send(cmdUserTempRRQ, []byte{0x05}); err != nil {
		return nil, err
	}
	data, err := c.readData()
	if err != nil {
		return nil, err
	}
	return parseUsers(data)
}

// parseUsers decodes the fixed-size user records.
//
// Two record layouts exist in the wild: 72 bytes (modern, the common case) and
// 28 bytes (older/compact firmwares). Some firmwares additionally prefix the
// block with a 4-byte total size; that prefix is detected and stripped rather
// than being mis-parsed as the first record's fields.
func parseUsers(data []byte) ([]User, error) {
	if len(data) == 0 {
		return nil, nil
	}
	stride, body := detectUserStride(data)
	if stride == 0 {
		return nil, fmt.Errorf("zk: user data length %d is not a supported record size (72 or 28)", len(data))
	}

	var out []User
	for off := 0; off+stride <= len(body); off += stride {
		rec := body[off : off+stride]
		var u User
		if stride == 72 {
			// <H B 8s 24s I x 7s x 24s>:
			// uid, privilege, password[8], name[24], card(uint32), pad,
			// group[7], pad, user_id[24]
			u.UID = int(binary.LittleEndian.Uint16(rec[0:2]))
			u.Privilege = int(rec[2])
			u.Name = cstr(rec[11:35])
			u.CardNumber = formatCard(binary.LittleEndian.Uint32(rec[35:39]))
			u.UserID = cstr(rec[48:72])
		} else {
			// <H B 5s 8s I x B h I>:
			// uid, privilege, password[5], name[8], card(uint32), pad,
			// group(uint8), timezone(int16), user_id(uint32)
			u.UID = int(binary.LittleEndian.Uint16(rec[0:2]))
			u.Privilege = int(rec[2])
			u.Name = cstr(rec[8:16])
			u.CardNumber = formatCard(binary.LittleEndian.Uint32(rec[16:20]))
			u.UserID = fmt.Sprintf("%d", binary.LittleEndian.Uint32(rec[24:28]))
		}
		// Some devices pad the table with empty slots; a record with no PIN is
		// one of those, not a user.
		if u.UserID == "" || u.UserID == "0" {
			continue
		}
		out = append(out, u)
	}
	return out, nil
}

// detectUserStride picks the record size and strips a leading 4-byte size header
// if one is present. The 72-byte layout is tried first because a 4-byte-prefixed
// 72-byte block can also be divisible by 28 in rare sizes, and 72 is far more
// common on the devices this gateway targets.
func detectUserStride(data []byte) (int, []byte) {
	for _, stride := range []int{72, 28} {
		if len(data)%stride == 0 {
			return stride, data
		}
	}
	if len(data) > 4 {
		body := data[4:]
		for _, stride := range []int{72, 28} {
			if len(body)%stride == 0 {
				return stride, body
			}
		}
	}
	return 0, nil
}

// formatCard renders the numeric card value, mapping "no card" (0) to the empty
// string so it never overwrites a real card number stored from another source.
func formatCard(v uint32) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%d", v)
}
