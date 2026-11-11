// SPDX-License-Identifier: MIT

// Package flows manages the Live Connections table, Connection Log ring buffer
// (with deduplication, 30s UDP silence window, and Include/Exclude LogFilters),
// and Activity Log ring buffer.
package flows

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxLiveFlows = 400

// Flow represents a single active or recently closed network flow in the Live Connections view.
type Flow struct {
	ID          string `json:"id"`
	PID         int    `json:"pid"`
	ProcessName string `json:"processName"`
	DestIP      string `json:"destIp"`
	Host        string `json:"host"`
	Port        uint16 `json:"port"`
	IsUDP       bool   `json:"isUdp"`
	IsIPv6      bool   `json:"isIpv6"`
	ProxyID     string `json:"proxyId"`
	ProxyLabel  string `json:"proxyLabel"`
	RuleID      string `json:"ruleId"`
	AgeMs       int64  `json:"ageMs"`
	Sent        int64  `json:"sent"`
	Received    int64  `json:"received"`
	Closed      bool   `json:"closed"`
	StartedAt   int64  `json:"startedAt"`
	ClosedAt    int64  `json:"closedAt,omitempty"`
}

// Tracker maintains up to MaxLiveFlows active and recently closed flows.
type Tracker struct {
	mu      sync.RWMutex
	seq     atomic.Uint64
	byID    map[string]*Flow
	order   []string
	maxSize int
}

// NewTracker creates a live flow tracker bounded to maxSize (defaults to MaxLiveFlows).
func NewTracker(maxSize int) *Tracker {
	if maxSize <= 0 {
		maxSize = MaxLiveFlows
	}
	return &Tracker{
		byID:    make(map[string]*Flow, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// StartFlow registers a new active flow and returns its generated ID.
func (t *Tracker) StartFlow(f Flow) string {
	now := time.Now().UnixMilli()
	if f.ID == "" {
		seq := t.seq.Add(1)
		f.ID = fmt.Sprintf("f-%d-%d", now, seq)
	}
	if f.StartedAt == 0 {
		f.StartedAt = now
	}
	if f.IsIPv6 == false && strings.Contains(f.DestIP, ":") {
		f.IsIPv6 = true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if existing, ok := t.byID[f.ID]; ok {
		*existing = f
		return f.ID
	}

	if len(t.order) >= t.maxSize {
		t.evictOneLocked()
	}

	cp := f
	t.byID[f.ID] = &cp
	t.order = append(t.order, f.ID)
	return f.ID
}

// evictOneLocked removes the oldest closed flow first, or the oldest active flow if all are active.
func (t *Tracker) evictOneLocked() {
	if len(t.order) == 0 {
		return
	}
	evictIdx := -1
	for i, id := range t.order {
		if fl, ok := t.byID[id]; ok && fl.Closed {
			evictIdx = i
			break
		}
	}
	if evictIdx < 0 {
		evictIdx = 0
	}
	victimID := t.order[evictIdx]
	delete(t.byID, victimID)
	t.order = append(t.order[:evictIdx], t.order[evictIdx+1:]...)
}

// AddBytes increments the sent and received payload counters for flow id.
func (t *Tracker) AddBytes(id string, sentDelta, recvDelta int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if fl, ok := t.byID[id]; ok {
		fl.Sent += sentDelta
		fl.Received += recvDelta
	}
}

// CloseFlow marks flow id as closed (retained in greyed-out state until evicted or cleared).
func (t *Tracker) CloseFlow(id string) {
	now := time.Now().UnixMilli()
	t.mu.Lock()
	defer t.mu.Unlock()
	if fl, ok := t.byID[id]; ok && !fl.Closed {
		fl.Closed = true
		fl.ClosedAt = now
		fl.AgeMs = now - fl.StartedAt
	}
}

// ClearClosed removes all closed flows while keeping active flows intact.
func (t *Tracker) ClearClosed() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	removed := 0
	kept := make([]string, 0, len(t.order))
	for _, id := range t.order {
		fl, ok := t.byID[id]
		if !ok {
			continue
		}
		if fl.Closed {
			delete(t.byID, id)
			removed++
		} else {
			kept = append(kept, id)
		}
	}
	t.order = kept
	return removed
}

// ClearAll removes both active and closed flows from the tracker.
func (t *Tracker) ClearAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byID = make(map[string]*Flow, t.maxSize)
	t.order = t.order[:0]
}

// ReplaceFromExternal replaces the tracker's flow snapshot with externally reported flows
// (such as macOS NetworkExtension getFlows IPC responses).
func (t *Tracker) ReplaceFromExternal(ext []Flow) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byID = make(map[string]*Flow, len(ext))
	t.order = make([]string, 0, len(ext))
	for i := range ext {
		if len(t.order) >= t.maxSize {
			break
		}
		cp := ext[i]
		if cp.ID == "" {
			cp.ID = fmt.Sprintf("ext-%d", i+1)
		}
		t.byID[cp.ID] = &cp
		t.order = append(t.order, cp.ID)
	}
}

// GetFlows returns up to limit flows ordered newest first, with active AgeMs computed dynamically.
func (t *Tracker) GetFlows(limit int) []Flow {
	if limit <= 0 || limit > t.maxSize {
		limit = t.maxSize
	}
	now := time.Now().UnixMilli()

	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]Flow, 0, min(limit, len(t.order)))
	for i := len(t.order) - 1; i >= 0 && len(out) < limit; i-- {
		id := t.order[i]
		fl, ok := t.byID[id]
		if !ok {
			continue
		}
		cp := *fl
		if !cp.Closed && cp.StartedAt > 0 {
			cp.AgeMs = max(0, now-cp.StartedAt)
		}
		out = append(out, cp)
	}
	return out
}
