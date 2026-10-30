package zk

import (
	"encoding/binary"
	"testing"
)

// build72 lays out one 72-byte user record exactly as the documented layout
// specifies: <H B 8s 24s I x 7s x 24s>.
func build72(uid uint16, priv byte, name string, card uint32, userID string) []byte {
	rec := make([]byte, 72)
	binary.LittleEndian.PutUint16(rec[0:2], uid)
	rec[2] = priv
	copy(rec[3:11], "secret") // password
	copy(rec[11:35], name)    // name
	binary.LittleEndian.PutUint32(rec[35:39], card)
	copy(rec[40:47], "1")    // group id
	copy(rec[48:72], userID) // PIN
	return rec
}

// build28 lays out one 28-byte legacy record: <H B 5s 8s I x B h I>.
func build28(uid uint16, priv byte, name string, card uint32, userID uint32) []byte {
	rec := make([]byte, 28)
	binary.LittleEndian.PutUint16(rec[0:2], uid)
	rec[2] = priv
	copy(rec[3:8], "pw")
	copy(rec[8:16], name)
	binary.LittleEndian.PutUint32(rec[16:20], card)
	rec[21] = 1
	binary.LittleEndian.PutUint32(rec[24:28], userID)
	return rec
}

func TestParseUsers72(t *testing.T) {
	data := append(build72(1, 14, "Asha Mwanga", 12345, "1001"),
		build72(2, 0, "Juma Ali", 0, "1002")...)

	users, err := parseUsers(data)
	if err != nil {
		t.Fatalf("parseUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d (%+v)", len(users), users)
	}
	if users[0].UserID != "1001" || users[0].Name != "Asha Mwanga" {
		t.Errorf("record 0 = %+v, want UserID 1001 / Name Asha Mwanga", users[0])
	}
	if users[0].Privilege != 14 || users[0].CardNumber != "12345" {
		t.Errorf("record 0 = %+v, want Privilege 14 / Card 12345", users[0])
	}
	// Card 0 means "no card" and must not become the string "0", which would
	// overwrite a real card number stored from an import.
	if users[1].CardNumber != "" {
		t.Errorf("record 1 card = %q, want empty for card 0", users[1].CardNumber)
	}
	if users[1].Name != "Juma Ali" || users[1].UserID != "1002" {
		t.Errorf("record 1 = %+v", users[1])
	}
}

func TestParseUsers28(t *testing.T) {
	data := append(build28(1, 0, "Asha", 77, 1001), build28(2, 14, "Juma", 0, 1002)...)

	users, err := parseUsers(data)
	if err != nil {
		t.Fatalf("parseUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d (%+v)", len(users), users)
	}
	if users[0].UserID != "1001" || users[0].Name != "Asha" || users[0].CardNumber != "77" {
		t.Errorf("record 0 = %+v", users[0])
	}
	if users[1].UserID != "1002" || users[1].Privilege != 14 {
		t.Errorf("record 1 = %+v", users[1])
	}
}

// Some firmwares prefix the block with a 4-byte total size. It must be stripped,
// not consumed as the first record's uid/privilege/password bytes.
func TestParseUsersSizePrefixStripped(t *testing.T) {
	body := build72(1, 0, "Asha Mwanga", 0, "1001")
	data := make([]byte, 4, 4+len(body))
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(body)))
	data = append(data, body...)

	users, err := parseUsers(data)
	if err != nil {
		t.Fatalf("parseUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("want 1 user, got %d (%+v)", len(users), users)
	}
	if users[0].UserID != "1001" || users[0].Name != "Asha Mwanga" {
		t.Errorf("got %+v, want UserID 1001 / Name Asha Mwanga", users[0])
	}
}

// Empty enrolment slots (all zero) are padding, not users.
func TestParseUsersSkipsEmptySlots(t *testing.T) {
	data := append(build72(1, 0, "Asha", 0, "1001"), make([]byte, 72)...)
	users, err := parseUsers(data)
	if err != nil {
		t.Fatalf("parseUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("want 1 user (empty slot skipped), got %d (%+v)", len(users), users)
	}
}

// A length matching neither layout is reported rather than silently mis-parsed.
func TestParseUsersUnknownStride(t *testing.T) {
	if _, err := parseUsers(make([]byte, 50)); err == nil {
		t.Fatal("want an error for an unsupported record size, got nil")
	}
}

// A device with no enrolled users returns nothing, not an error.
func TestParseUsersEmpty(t *testing.T) {
	users, err := parseUsers(nil)
	if err != nil || len(users) != 0 {
		t.Fatalf("want (nil, nil), got (%+v, %v)", users, err)
	}
}
