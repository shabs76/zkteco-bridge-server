package iclock

import (
	"strings"
	"testing"
	"time"
)

func TestParseUserInfoWithUserToken(t *testing.T) {
	body := "USER PIN=1001\tName=Asha Mwanga\tPri=14\tPasswd=\tCard=12345\tGrp=1\tTZ=0\n" +
		"USER PIN=1002\tName=Juma Ali\tPri=0\tCard=0\n"

	recs := ParseUserInfo(body)
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d (%+v)", len(recs), recs)
	}
	if recs[0].PIN != "1001" || recs[0].Name != "Asha Mwanga" {
		t.Errorf("record 0 = %+v", recs[0])
	}
	if recs[0].Privilege != 14 || recs[0].CardNumber != "12345" {
		t.Errorf("record 0 = %+v, want Pri 14 / Card 12345", recs[0])
	}
	// Card=0 means "no card" and must not be stored as the literal "0".
	if recs[1].CardNumber != "" {
		t.Errorf("record 1 card = %q, want empty", recs[1].CardNumber)
	}
}

func TestParseUserInfoWithoutUserToken(t *testing.T) {
	recs := ParseUserInfo("PIN=7\tName=Neema\tCard=99\n")
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d (%+v)", len(recs), recs)
	}
	if recs[0].PIN != "7" || recs[0].Name != "Neema" || recs[0].CardNumber != "99" {
		t.Errorf("got %+v", recs[0])
	}
}

// OPERLOG interleaves operator-log rows with user rows; only the user rows are
// ours to consume.
func TestParseUserInfoSkipsNonUserRows(t *testing.T) {
	body := "OPLOG 4\t1\t0\t2026-01-02 08:00:00\t0\t0\n" +
		"USER PIN=1001\tName=Asha\n" +
		"FP PIN=1001\tFID=0\tSize=100\n"

	recs := ParseUserInfo(body)
	if len(recs) != 1 {
		t.Fatalf("want only the USER row, got %d (%+v)", len(recs), recs)
	}
	if recs[0].PIN != "1001" {
		t.Errorf("got %+v", recs[0])
	}
}

// A user with no name enrolled is still a user — the row is kept so the PIN is
// registered, and the empty name simply never overwrites anything.
func TestParseUserInfoKeepsNamelessUser(t *testing.T) {
	recs := ParseUserInfo("USER PIN=1003\tName=\tPri=0\n")
	if len(recs) != 1 || recs[0].PIN != "1003" || recs[0].Name != "" {
		t.Fatalf("got %+v", recs)
	}
}

func TestParseUserInfoIgnoresJunk(t *testing.T) {
	if recs := ParseUserInfo("\n\t\nnonsense\n"); len(recs) != 0 {
		t.Fatalf("want no records, got %+v", recs)
	}
}

// The option block must tell the device its zone, or the firmware keeps its
// clock in GMT (from the HTTP Date header) and stamps every punch hours early.
func TestRegistryReplyTimeZone(t *testing.T) {
	dar, err := time.LoadLocation("Africa/Dar_es_Salaam")
	if err != nil {
		t.Fatal(err)
	}
	if got := RegistryReply("SN1", dar); !strings.Contains(got, "\nTimeZone=3\n") {
		t.Errorf("Dar es Salaam reply missing TimeZone=3:\n%s", got)
	}
	if got := RegistryReply("SN1", time.UTC); !strings.Contains(got, "\nTimeZone=0\n") {
		t.Errorf("UTC reply missing TimeZone=0:\n%s", got)
	}

	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	if got := RegistryReply("SN1", kolkata); strings.Contains(got, "TimeZone=") {
		t.Errorf("part-hour zone must omit TimeZone, got:\n%s", got)
	}
}

func TestTimeZoneHoursNegative(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	winter := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	if h, ok := TimeZoneHours(ny, winter); !ok || h != -5 {
		t.Errorf("New York in January = %d, %v; want -5, true", h, ok)
	}
}
