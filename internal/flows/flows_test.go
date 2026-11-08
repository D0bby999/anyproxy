// SPDX-License-Identifier: MIT

package flows

import (
	"fmt"
	"testing"
	"time"

	"github.com/D0bby999/anyproxyxa/internal/pbprofile"
)

func TestTrackerLifecycleAndEviction(t *testing.T) {
	tr := NewTracker(3)

	id1 := tr.StartFlow(Flow{PID: 100, ProcessName: "curl", DestIP: "1.1.1.1", Port: 443, ProxyID: "DIRECT"})
	id2 := tr.StartFlow(Flow{PID: 101, ProcessName: "safari", DestIP: "2001:4860:4860::8888", Port: 443, ProxyID: "p1"})
	id3 := tr.StartFlow(Flow{PID: 102, ProcessName: "git", DestIP: "140.82.121.4", Port: 22, ProxyID: "p1"})

	tr.AddBytes(id2, 512, 2048)
	tr.CloseFlow(id1)

	// Adding a 4th flow when capacity is 3 should evict id1 (the closed flow) first
	id4 := tr.StartFlow(Flow{PID: 103, ProcessName: "node", DestIP: "8.8.8.8", Port: 53, IsUDP: true, ProxyID: "p1"})

	flows := tr.GetFlows(10)
	if len(flows) != 3 {
		t.Fatalf("expected 3 flows, got %d", len(flows))
	}
	for _, fl := range flows {
		if fl.ID == id1 {
			t.Fatalf("expected closed flow id1 to be evicted first")
		}
		if fl.ID == id2 {
			if !fl.IsIPv6 || fl.Sent != 512 || fl.Received != 2048 {
				t.Fatalf("unexpected flow id2 state: %+v", fl)
			}
		}
	}

	tr.CloseFlow(id3)
	if removed := tr.ClearClosed(); removed != 1 {
		t.Fatalf("expected ClearClosed to remove 1 flow (id3), got %d", removed)
	}
	remaining := tr.GetFlows(10)
	if len(remaining) != 2 || remaining[0].ID != id4 || remaining[1].ID != id2 {
		t.Fatalf("unexpected remaining flows: %+v", remaining)
	}
}

func TestLogManagerDedupUDPWindowAndFilters(t *testing.T) {
	lm := NewLogManager()

	// 1. Default deduplication mode (LogEveryConnection = false)
	e := ConnectionLogEntry{
		ProcessName: "curl",
		DestIP:      "93.184.216.34",
		Host:        "example.com",
		Port:        443,
		Protocol:    "TCP",
		Action:      "PROXY",
		ProxyName:   "SOCKS5 127.0.0.1:1080",
	}
	lm.RecordConnection(e)
	lm.RecordConnection(e)
	lm.RecordConnection(e)

	logs := lm.ConnectionLogs(10)
	if len(logs) != 1 || logs[0].Count != 3 {
		t.Fatalf("expected 1 deduplicated entry with Count=3, got %+v", logs)
	}

	// 2. LogEveryConnection = true: TCP logs every connection; UDP deduplicates within 30s window
	lm.ClearConnectionLogs()
	lm.SetConfig(LogConfig{
		TrafficLoggingEnabled: true,
		LogEveryConnection:    true,
	})
	t0 := time.Now()
	lm.RecordConnectionAt(e, t0)
	lm.RecordConnectionAt(e, t0.Add(1*time.Second))
	if got := len(lm.ConnectionLogs(10)); got != 2 {
		t.Fatalf("expected 2 separate TCP log entries when LogEveryConnection=true, got %d", got)
	}

	lm.ClearConnectionLogs()
	udpEntry := ConnectionLogEntry{
		ProcessName: "mDNSResponder",
		DestIP:      "8.8.8.8",
		Port:        53,
		Protocol:    "UDP",
		Action:      "DIRECT",
	}
	lm.RecordConnectionAt(udpEntry, t0)
	lm.RecordConnectionAt(udpEntry, t0.Add(10*time.Second)) // within 30s silence window -> increments Count
	lm.RecordConnectionAt(udpEntry, t0.Add(45*time.Second)) // >30s after last seen -> new entry
	udpLogs := lm.ConnectionLogs(10)
	if len(udpLogs) != 2 || udpLogs[1].Count != 2 || udpLogs[0].Count != 1 {
		t.Fatalf("unexpected UDP 30s window logs: %+v", udpLogs)
	}

	// 3. LogFilters: EXCLUDE takes priority over INCLUDE
	lm.ClearConnectionLogs()
	lm.SetConfig(LogConfig{
		TrafficLoggingEnabled: true,
		LogEveryConnection:    true,
		Filters: []pbprofile.LogFilter{
			{
				ID:       "1",
				Mode:     "EXCLUDE",
				Process:  "mdns*",
				Host:     "*",
				Port:     "53;5353",
				Protocol: "UDP",
				Action:   "*",
			},
			{
				ID:       "2",
				Mode:     "INCLUDE",
				Process:  "*",
				Host:     "*",
				Port:     "80;443;53",
				Protocol: "*",
				Action:   "*",
			},
		},
	})

	if lm.RecordConnection(udpEntry) {
		t.Fatalf("expected mDNSResponder UDP 53 to be suppressed by EXCLUDE filter")
	}
	sshEntry := ConnectionLogEntry{
		ProcessName: "ssh",
		DestIP:      "10.0.0.5",
		Port:        22,
		Protocol:    "TCP",
		Action:      "DIRECT",
	}
	if lm.RecordConnection(sshEntry) {
		t.Fatalf("expected port 22 to be suppressed because INCLUDE filter only allows 80;443;53")
	}
	if !lm.RecordConnection(e) {
		t.Fatalf("expected curl HTTPS 443 to pass INCLUDE filter")
	}

	// 4. WarnOnce deduplication
	if !lm.WarnOnce("http-udp:p1", "HTTP proxy p1 does not support UDP; routing DIRECT") {
		t.Fatalf("expected first WarnOnce to return true")
	}
	if lm.WarnOnce("http-udp:p1", "HTTP proxy p1 does not support UDP; routing DIRECT") {
		t.Fatalf("expected second WarnOnce with same key to return false")
	}
	if got := len(lm.ActivityLogs(10)); got != 1 {
		t.Fatalf("expected 1 activity log entry, got %d", got)
	}

	// 5. AutoClearLogs at 500 entries
	lm.ClearConnectionLogs()
	lm.SetConfig(LogConfig{
		TrafficLoggingEnabled: true,
		LogEveryConnection:    true,
		AutoClearLogs:         true,
	})
	for i := 0; i < AutoClearThreshold+5; i++ {
		lm.RecordConnection(ConnectionLogEntry{
			ProcessName: fmt.Sprintf("proc-%d", i),
			DestIP:      "1.1.1.1",
			Port:        443,
			Protocol:    "TCP",
			Action:      "DIRECT",
		})
	}
	if got := len(lm.ConnectionLogs(1000)); got != 5 {
		t.Fatalf("expected AutoClearLogs to reset at %d and leave 5 entries, got %d", AutoClearThreshold, got)
	}
}
