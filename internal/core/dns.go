package core

import (
	"net"
	"regexp"
	"sort"
	"strings"
)

// domainRe is a pragmatic hostname/domain validator (RFC 1035-ish labels).
var domainRe = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$`)

// validDomain reports whether s looks like a valid domain name.
func validDomain(s string) bool {
	if len(s) > 253 {
		return false
	}
	return domainRe.MatchString(s)
}

// lookupMX returns the MX hostnames for a domain, sorted by preference, with the
// trailing dot stripped to match pydig's output style.
func lookupMX(domain string) ([]string, error) {
	records, err := net.LookupMX(domain)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})
	out := make([]string, 0, len(records))
	for _, r := range records {
		out = append(out, strings.TrimSuffix(r.Host, "."))
	}
	return out, nil
}
