// Package punchmap turns a raw ZKTeco punch "state" code into the IN/OUT
// direction the HR service accepts. Both the pull collector and the iclock push
// handler use it so the mapping is identical regardless of how a punch arrived.
package punchmap

import (
	"encoding/json"
	"strconv"
	"strings"
)

// defaultMap covers the standard ZK attendance states. Devices in simple
// access-control mode usually emit 0 (check-in) and 1 (check-out); the break /
// overtime pairs are mapped to the nearest direction.
//
//	0 Check-In      -> IN
//	1 Check-Out     -> OUT
//	2 Break-Out     -> OUT
//	3 Break-In      -> IN
//	4 Overtime-In   -> IN
//	5 Overtime-Out  -> OUT
var defaultMap = map[int]string{
	0: "IN", 1: "OUT", 2: "OUT", 3: "IN", 4: "IN", 5: "OUT",
}

// Map returns "IN" or "OUT" for a raw state, honoring a per-device override map
// (JSON like {"0":"IN","1":"OUT"}) when provided, then the default table, and
// finally falling back to "IN" for unrecognized states. Many devices that are
// not configured for direction emit only state 0, so everything becomes IN and
// the HR shift logic pairs punches by time — configure the device for In/Out
// mode, or set a device punch_state_map, when true IN/OUT is required.
func Map(overrideJSON string, state int) string {
	if s := strings.TrimSpace(overrideJSON); s != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			if v, ok := m[strconv.Itoa(state)]; ok {
				return normalize(v)
			}
		}
	}
	if v, ok := defaultMap[state]; ok {
		return v
	}
	return "IN"
}

func normalize(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "OUT") {
		return "OUT"
	}
	return "IN"
}
