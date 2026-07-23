// Package validate holds small input validators shared by controllers.
package validate

import (
	"net"
	"regexp"
	"strconv"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsValidUUID reports whether s is a canonical 36-char UUID.
func IsValidUUID(s string) bool { return uuidRe.MatchString(s) }

// IsValidHostPort reports whether host is a non-empty hostname/IP and port is in
// the valid TCP range.
func IsValidHostPort(host string, port int) bool {
	if host == "" || port < 1 || port > 65535 {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	// Not an IP — accept as a hostname (basic sanity; DNS is resolved at dial).
	return len(host) <= 253
}

// ClampPage normalizes page/pageSize into safe bounds.
func ClampPage(page, pageSize string) (int, int) {
	p, _ := strconv.Atoi(page)
	if p < 1 {
		p = 1
	}
	ps, _ := strconv.Atoi(pageSize)
	if ps < 1 {
		ps = 25
	}
	if ps > 200 {
		ps = 200
	}
	return p, ps
}
