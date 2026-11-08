// SPDX-License-Identifier: AGPL-3.0-or-later

package dnsmap

import (
	"bufio"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxEntries      = 32768
	DefaultMaxDomainsPerIP = 8
	DefaultMaxAgeSeconds   = 300.0
)

type entry struct {
	name string
	seen float64
}

type Store struct {
	mu              sync.RWMutex
	entries         map[string][]entry
	order           []string
	maxEntries      int
	maxDomainsPerIP int
}

func New(maxEntries int) *Store {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &Store{
		entries:         make(map[string][]entry),
		maxEntries:      maxEntries,
		maxDomainsPerIP: DefaultMaxDomainsPerIP,
	}
}

func normalizeIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if a, err := netip.ParseAddr(ip); err == nil {
		return a.Unmap().String()
	}
	return strings.ToLower(ip)
}

func (s *Store) Clear() {
	s.mu.Lock()
	s.entries = make(map[string][]entry)
	s.order = nil
	s.mu.Unlock()
}

func NowSec() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

func (s *Store) Record(domain string, ips []string, now float64) {
	name := strings.ToLower(strings.TrimSpace(domain))
	if name == "" || len(ips) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rawIP := range ips {
		ip := normalizeIP(rawIP)
		if ip == "" {
			continue
		}
		if _, exists := s.entries[ip]; !exists {
			s.entries[ip] = nil
			s.order = append(s.order, ip)
			if len(s.order) > s.maxEntries {
				batch := 64
				if s.maxEntries < 128 {
					batch = 1
				}
				overflow := len(s.order) - s.maxEntries + batch
				if overflow > len(s.order) {
					overflow = len(s.order)
				}
				for _, evicted := range s.order[:overflow] {
					delete(s.entries, evicted)
				}
				s.order = append([]string(nil), s.order[overflow:]...)
			}
		}
		doms := s.entries[ip]
		idx := -1
		for i, d := range doms {
			if d.name == name {
				idx = i
				break
			}
		}
		if idx >= 0 {
			doms = append(doms[:idx], doms[idx+1:]...)
		} else if len(doms) >= s.maxDomainsPerIP {
			doms = doms[1:]
		}
		doms = append(doms, entry{name: name, seen: now})
		s.entries[ip] = doms
	}
}

// DomainsForIP returns all recorded domain names for ip, newest first.
func (s *Store) DomainsForIP(ip string) []string {
	key := normalizeIP(ip)
	s.mu.RLock()
	defer s.mu.RUnlock()
	doms := s.entries[key]
	if len(doms) == 0 {
		return nil
	}
	out := make([]string, len(doms))
	for i := range doms {
		out[i] = doms[len(doms)-1-i].name
	}
	return out
}

// LatestDomain returns the most recently recorded safe hostname for ip if it
// was observed within maxAge seconds of now; otherwise "".
func (s *Store) LatestDomain(ip string, maxAge, now float64) string {
	key := normalizeIP(ip)
	s.mu.RLock()
	defer s.mu.RUnlock()
	doms := s.entries[key]
	if len(doms) == 0 {
		return ""
	}
	newest := doms[len(doms)-1]
	if now-newest.seen > maxAge || !IsSafeHostname(newest.name) {
		return ""
	}
	return newest.name
}

// LoadHostsFile parses a system hosts file (/etc/hosts) into the DNS map.
func (s *Store) LoadHostsFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	now := NowSec()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		if _, err := netip.ParseAddr(ip); err != nil {
			continue
		}
		for _, host := range fields[1:] {
			if IsSafeHostname(host) {
				s.Record(host, []string{ip}, now)
			}
		}
	}
	return sc.Err()
}

