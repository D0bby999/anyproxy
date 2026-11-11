// SPDX-License-Identifier: AGPL-3.0-or-later

// Package dnsmap parses DNS responses (A, AAAA, CNAME chains) and maintains a
// bounded IP -> domain-name cache for domain rule matching and SendDomainToProxy.
package dnsmap

import (
	"fmt"
	"net/netip"
	"strings"
)

// IsSafeHostname reports whether name is a safe ASCII hostname (letters,
// digits, '.', '-', '_'), length 1..253, not starting or ending with '.', and
// not an IPv4 or IPv6 literal.
func IsSafeHostname(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	if name[0] == '.' || name[len(name)-1] == '.' {
		return false
	}
	allDigitsAndDots := true
	for i := 0; i < len(name); i++ {
		b := name[i]
		switch {
		case b >= '0' && b <= '9':
		case b == '.':
		case (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || b == '-' || b == '_':
			allDigitsAndDots = false
		default:
			return false
		}
	}
	if allDigitsAndDots {
		return false
	}
	if _, err := netip.ParseAddr(name); err == nil {
		return false
	}
	return true
}

// ParseResponse extracts the queried domain name and any resolved A/AAAA IP
// addresses from a raw RFC 1035 DNS wire-format message. It follows CNAME
// chains starting at the question name sounrelated records cannot poison the
// mapping.
func ParseResponse(b []byte) (domain string, ips []string, ok bool) {
	if len(b) < 12 {
		return "", nil, false
	}
	qdCount := int(b[4])<<8 | int(b[5])
	anCount := int(b[6])<<8 | int(b[7])
	if qdCount < 1 || anCount < 1 {
		return "", nil, false
	}
	i := 12
	qname, next, ok := readName(b, i)
	if !ok || next+4 > len(b) {
		return "", nil, false
	}
	i = next + 4 // skip QTYPE + QCLASS

	for q := 1; q < qdCount; q++ {
		next, ok = skipName(b, i)
		if !ok || next+4 > len(b) {
			return "", nil, false
		}
		i = next + 4
	}

	qname = strings.ToLower(qname)
	if !IsSafeHostname(qname) {
		return "", nil, false
	}

	allowed := map[string]bool{qname: true}
	type rr struct {
		owner string
		rtype int
		rdata []byte
		rdpos int
	}
	records := make([]rr, 0, anCount)
	for a := 0; a < anCount; a++ {
		owner, nextOff, ok := readName(b, i)
		if !ok || nextOff+10 > len(b) {
			break
		}
		rtype := int(b[nextOff])<<8 | int(b[nextOff+1])
		rdlen := int(b[nextOff+8])<<8 | int(b[nextOff+9])
		rdpos := nextOff + 10
		if rdpos+rdlen > len(b) {
			break
		}
		records = append(records, rr{
			owner: strings.ToLower(owner),
			rtype: rtype,
			rdata: b[rdpos : rdpos+rdlen],
			rdpos: rdpos,
		})
		i = rdpos + rdlen
	}

	// Resolve CNAME chain (up to 16 hops) even if CNAME records are out of order.
	for pass := 0; pass < 16; pass++ {
		added := false
		for _, r := range records {
			if r.rtype == 5 && allowed[r.owner] {
				cname, _, ok := readName(b, r.rdpos)
				if ok {
					cname = strings.ToLower(cname)
					if IsSafeHostname(cname) && !allowed[cname] {
						allowed[cname] = true
						added = true
					}
				}
			}
		}
		if !added {
			break
		}
	}

	for _, r := range records {
		if !allowed[r.owner] {
			continue
		}
		switch {
		case r.rtype == 1 && len(r.rdata) == 4:
			ips = append(ips, fmt.Sprintf("%d.%d.%d.%d", r.rdata[0], r.rdata[1], r.rdata[2], r.rdata[3]))
		case r.rtype == 28 && len(r.rdata) == 16:
			var a16 [16]byte
			copy(a16[:], r.rdata)
			ips = append(ips, netip.AddrFrom16(a16).String())
		}
	}
	if len(ips) == 0 {
		return "", nil, false
	}
	return qname, ips, true
}

func readName(b []byte, start int) (string, int, bool) {
	var labels []string
	j := start
	next := start
	jumped := false
	hops := 0
	for j < len(b) {
		hops++
		if hops > 128 {
			return "", 0, false
		}
		l := int(b[j])
		if l == 0 {
			if !jumped {
				next = j + 1
			}
			return strings.Join(labels, "."), next, true
		}
		if l&0xC0 == 0xC0 {
			if j+1 >= len(b) {
				return "", 0, false
			}
			if !jumped {
				next = j + 2
			}
			jumped = true
			j = ((l & 0x3F) << 8) | int(b[j+1])
			continue
		}
		if l&0xC0 != 0 || j+1+l > len(b) {
			return "", 0, false
		}
		labels = append(labels, string(b[j+1:j+1+l]))
		j += 1 + l
	}
	return "", 0, false
}

func skipName(b []byte, start int) (int, bool) {
	j := start
	hops := 0
	for j < len(b) {
		hops++
		if hops > 128 {
			return 0, false
		}
		l := int(b[j])
		if l == 0 {
			return j + 1, true
		}
		if l&0xC0 == 0xC0 {
			if j+1 >= len(b) {
				return 0, false
			}
			return j + 2, true
		}
		if l&0xC0 != 0 || j+1+l > len(b) {
			return 0, false
		}
		j += 1 + l
	}
	return 0, false
}
