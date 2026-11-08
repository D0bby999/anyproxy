// SPDX-License-Identifier: AGPL-3.0-or-later

package dnsmap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestDNSMapStoreUpstreamVectors(t *testing.T) {
	// Use 1024 capacity to match macOS DNSMapStore.swift test vectors.
	s := New(1024)

	s.Record("A.Example.com", []string{"1.1.1.1", "2606::1"}, 100)
	if got := s.DomainsForIP("1.1.1.1"); !reflect.DeepEqual(got, []string{"a.example.com"}) {
		t.Fatalf("lookup lowercased = %v", got)
	}
	if got := s.DomainsForIP("2606::1"); !reflect.DeepEqual(got, []string{"a.example.com"}) {
		t.Fatalf("ipv6 key = %v", got)
	}
	s.Record("b.example.com", []string{"1.1.1.1"}, 110)
	if got := s.DomainsForIP("1.1.1.1"); !reflect.DeepEqual(got, []string{"b.example.com", "a.example.com"}) {
		t.Fatalf("newest first = %v", got)
	}
	s.Record("a.example.com", []string{"1.1.1.1"}, 120)
	if got := s.DomainsForIP("1.1.1.1"); !reflect.DeepEqual(got, []string{"a.example.com", "b.example.com"}) {
		t.Fatalf("seen again becomes newest = %v", got)
	}
	for i := 0; i < 20; i++ {
		s.Record(fmt.Sprintf("d%d.test", i), []string{"9.9.9.9"}, 200)
	}
	if got := s.DomainsForIP("9.9.9.9"); len(got) != 8 || got[0] != "d19.test" {
		t.Fatalf("at most 8 names per ip = %v", got)
	}
	for i := 0; i < 3000; i++ {
		s.Record(fmt.Sprintf("x%d.test", i), []string{fmt.Sprintf("10.%d.%d.1", i/250, i%250)}, 300)
	}
	if got := s.DomainsForIP("10.0.0.1"); len(got) != 0 {
		t.Fatalf("expected 10.0.0.1 evicted, got %v", got)
	}
	if got := s.DomainsForIP("10.11.249.1"); !reflect.DeepEqual(got, []string{"x2999.test"}) {
		t.Fatalf("newest entries kept = %v", got)
	}
	s.Clear()
	if got := s.DomainsForIP("1.1.1.1"); len(got) != 0 {
		t.Fatalf("clear failed: %v", got)
	}

	// LatestDomain ("SendDomainToProxy")
	s.Record("a.example.com", []string{"1.1.1.1"}, 100)
	s.Record("b.example.com", []string{"1.1.1.1"}, 110)
	if got := s.LatestDomain("1.1.1.1", 300, 120); got != "b.example.com" {
		t.Fatalf("newest name wins = %q", got)
	}
	if got := s.LatestDomain("1.1.1.1", 300, 410); got != "b.example.com" {
		t.Fatalf("still good at 300s = %q", got)
	}
	if got := s.LatestDomain("1.1.1.1", 300, 411); got != "" {
		t.Fatalf("expired after 300s = %q", got)
	}
	s.Record("a.example.com", []string{"1.1.1.1"}, 500)
	if got := s.LatestDomain("1.1.1.1", 300, 600); got != "a.example.com" {
		t.Fatalf("fresh answer refreshes age = %q", got)
	}
	if got := s.LatestDomain("8.8.4.4", 300, 600); got != "" {
		t.Fatalf("unknown address = %q", got)
	}

	good := []string{"example.com", "a-b.c_d.example.org", "xn--bcher-kva.example", "localhost", "a"}
	bad := []string{
		"", "exa mple.com", "example.com\r\nHost: evil", "exam\x00ple.com", "exéample.com",
		".example.com", "example.com.", "1.2.3.4", "::1", "example.com:443", "a/b.com", "a@b.com",
		strings.Repeat("a", 254),
	}
	for _, g := range good {
		if !IsSafeHostname(g) {
			t.Fatalf("expected safe hostname: %q", g)
		}
	}
	for _, b := range bad {
		if IsSafeHostname(b) {
			t.Fatalf("expected unsafe hostname rejected: %q", b)
		}
	}
	s.Record("bad name.com", []string{"5.5.5.5"}, 1000)
	if got := s.LatestDomain("5.5.5.5", 300, 1001); got != "" {
		t.Fatalf("unsafe newest name must not be returned: %q", got)
	}
	s.Record("1.2.3.4", []string{"1.2.3.4"}, 1000)
	if got := s.LatestDomain("1.2.3.4", 300, 1001); got != "" {
		t.Fatalf("ip literal is not a name: %q", got)
	}

	// Concurrent read/write race check
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				s.Record(fmt.Sprintf("c%d.test", w), []string{fmt.Sprintf("7.%d.%d.1", w, i%200)}, 1000)
				_ = s.DomainsForIP(fmt.Sprintf("7.%d.1.1", w))
			}
		}(w)
	}
	wg.Wait()
}

func TestParseDNSResponseWithCNAMEChain(t *testing.T) {
	// Build a DNS response packet:
	// Question: www.shop.test (A)
	// Answer 1: www.shop.test -> CNAME cdn.edge.net
	// Answer 2: cdn.edge.net  -> A 198.51.100.7
	// Answer 3: unrelated.org -> A 203.0.113.99 (must be ignored!)
	var pkt []byte
	// Header: ID=0x1234, Flags=0x8180, QD=1, AN=3, NS=0, AR=0
	pkt = append(pkt, 0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00)
	encName := func(n string) []byte {
		var out []byte
		for _, p := range strings.Split(n, ".") {
			out = append(out, byte(len(p)))
			out = append(out, p...)
		}
		out = append(out, 0)
		return out
	}
	// Question: www.shop.test, QTYPE=A(1), QCLASS=IN(1)
	pkt = append(pkt, encName("www.shop.test")...)
	pkt = append(pkt, 0x00, 0x01, 0x00, 0x01)

	// Answer 1: pointer to offset 12 (0xC00C), TYPE=CNAME(5), CLASS=IN(1), TTL=60, RDATA=cdn.edge.net
	cnameRdata := encName("cdn.edge.net")
	pkt = append(pkt, 0xC0, 0x0C, 0x00, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x3C, 0x00, byte(len(cnameRdata)))
	pkt = append(pkt, cnameRdata...)

	// Answer 2: cdn.edge.net, TYPE=A(1), CLASS=IN(1), TTL=60, RDLEN=4, 198.51.100.7
	pkt = append(pkt, encName("cdn.edge.net")...)
	pkt = append(pkt, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x3C, 0x00, 0x04, 198, 51, 100, 7)

	// Answer 3: unrelated.org, TYPE=A(1), CLASS=IN(1), TTL=60, RDLEN=4, 203.0.113.99
	pkt = append(pkt, encName("unrelated.org")...)
	pkt = append(pkt, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x3C, 0x00, 0x04, 203, 0, 113, 99)

	dom, ips, ok := ParseResponse(pkt)
	if !ok || dom != "www.shop.test" || !reflect.DeepEqual(ips, []string{"198.51.100.7"}) {
		t.Fatalf("ParseResponse = (%q, %v, %v), want (www.shop.test, [198.51.100.7], true)", dom, ips, ok)
	}
}

func TestLoadHostsFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hosts")
	content := "# comment\n127.0.0.1 localhost\n192.0.2.55 api.internal.test corp.internal.test\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(1024)
	if err := s.LoadHostsFile(p); err != nil {
		t.Fatalf("LoadHostsFile: %v", err)
	}
	if got := s.DomainsForIP("192.0.2.55"); !reflect.DeepEqual(got, []string{"corp.internal.test", "api.internal.test"}) {
		t.Fatalf("DomainsForIP = %v", got)
	}
}

