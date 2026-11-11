// SPDX-License-Identifier: AGPL-3.0-or-later

package flows

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/rules"
)

const (
	MaxConnectionLogs  = 2000
	MaxActivityLogs    = 1000
	AutoClearThreshold = 500
	UDPSilenceWindow   = 30 * time.Second
)

// ConnectionLogEntry represents a single entry in the Connection Log view.
type ConnectionLogEntry struct {
	ID          int64  `json:"id"`
	Timestamp   string `json:"timestamp"`
	PID         int    `json:"pid"`
	ProcessName string `json:"processName"`
	DestIP      string `json:"destIp"`
	Host        string `json:"host"`
	Port        uint16 `json:"port"`
	Protocol    string `json:"protocol"` // "TCP" | "UDP"
	Action      string `json:"action"`   // "DIRECT" | "BLOCK" | "PROXY" | proxy label
	ProxyName   string `json:"proxyName"`
	RuleID      string `json:"ruleId"`
	Count       int    `json:"count"`
}

// ActivityLogEntry represents an operational or diagnostic message in the Activity Log view.
type ActivityLogEntry struct {
	ID        int64  `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // "INFO" | "WARN" | "ERROR"
	Message   string `json:"message"`
}

// LogConfig controls Connection Log behavior.
type LogConfig struct {
	TrafficLoggingEnabled bool
	LogEveryConnection    bool
	AutoClearLogs         bool
	Filters               []pbprofile.LogFilter
}

// LogManager manages both the Connection Log and Activity Log ring buffers.
type LogManager struct {
	mu          sync.RWMutex
	connSeq     atomic.Int64
	actSeq      atomic.Int64
	cfg         LogConfig
	connLogs    []ConnectionLogEntry
	dedupIndex  map[string]int // key -> index in connLogs
	udpLastSeen map[string]time.Time
	actLogs     []ActivityLogEntry
	warnedOnce  map[string]struct{}
}

// NewLogManager creates a ready-to-use LogManager.
func NewLogManager() *LogManager {
	return &LogManager{
		cfg: LogConfig{
			TrafficLoggingEnabled: true,
		},
		connLogs:    make([]ConnectionLogEntry, 0, 256),
		dedupIndex:  make(map[string]int, 256),
		udpLastSeen: make(map[string]time.Time, 128),
		actLogs:     make([]ActivityLogEntry, 0, 128),
		warnedOnce:  make(map[string]struct{}),
	}
}

// SetConfig updates logging preferences and active LogFilters.
func (m *LogManager) SetConfig(cfg LogConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
}

// Config returns a copy of the active LogConfig.
func (m *LogManager) Config() LogConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := m.cfg
	cp.Filters = append([]pbprofile.LogFilter(nil), m.cfg.Filters...)
	return cp
}

// RecordConnection evaluates filters and deduplication rules, then appends or updates a ConnectionLogEntry.
func (m *LogManager) RecordConnection(entry ConnectionLogEntry) bool {
	return m.RecordConnectionAt(entry, time.Now())
}

// RecordConnectionAt is identical to RecordConnection with an explicit clock for deterministic testing.
func (m *LogManager) RecordConnectionAt(entry ConnectionLogEntry, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.cfg.TrafficLoggingEnabled {
		return false
	}

	entry.Protocol = strings.ToUpper(strings.TrimSpace(entry.Protocol))
	if entry.Protocol == "" {
		entry.Protocol = "TCP"
	}

	if !matchesLogFilters(entry, m.cfg.Filters) {
		return false
	}

	destKey := strings.ToLower(strings.TrimSpace(entry.Host))
	if destKey == "" {
		destKey = strings.ToLower(strings.TrimSpace(entry.DestIP))
	}
	dedupKey := strings.ToLower(entry.ProcessName) + "|" + destKey + "|" +
		strconv.Itoa(int(entry.Port)) + "|" + entry.Protocol + "|" + strings.ToUpper(entry.Action)

	ts := now.UTC().Format("15:04:05.000")
	if entry.Timestamp == "" {
		entry.Timestamp = ts
	}

	if !m.cfg.LogEveryConnection {
		if idx, ok := m.dedupIndex[dedupKey]; ok && idx >= 0 && idx < len(m.connLogs) {
			m.connLogs[idx].Count++
			m.connLogs[idx].Timestamp = entry.Timestamp
			return true
		}
	} else if entry.Protocol == "UDP" {
		if last, ok := m.udpLastSeen[dedupKey]; ok && now.Sub(last) < UDPSilenceWindow {
			if idx, ok := m.dedupIndex[dedupKey]; ok && idx >= 0 && idx < len(m.connLogs) {
				m.connLogs[idx].Count++
				m.connLogs[idx].Timestamp = entry.Timestamp
				m.udpLastSeen[dedupKey] = now
				return true
			}
		}
		m.udpLastSeen[dedupKey] = now
	}

	if m.cfg.AutoClearLogs && len(m.connLogs) >= AutoClearThreshold {
		m.connLogs = m.connLogs[:0]
		clear(m.dedupIndex)
	} else if len(m.connLogs) >= MaxConnectionLogs {
		// Drop oldest 25% and rebuild dedup index
		drop := MaxConnectionLogs / 4
		m.connLogs = append([]ConnectionLogEntry(nil), m.connLogs[drop:]...)
		clear(m.dedupIndex)
		for i := range m.connLogs {
			k := buildEntryKey(m.connLogs[i])
			m.dedupIndex[k] = i
		}
	}

	entry.ID = m.connSeq.Add(1)
	if entry.Count <= 0 {
		entry.Count = 1
	}
	idx := len(m.connLogs)
	m.connLogs = append(m.connLogs, entry)
	m.dedupIndex[dedupKey] = idx
	return true
}

func buildEntryKey(e ConnectionLogEntry) string {
	destKey := strings.ToLower(strings.TrimSpace(e.Host))
	if destKey == "" {
		destKey = strings.ToLower(strings.TrimSpace(e.DestIP))
	}
	return strings.ToLower(e.ProcessName) + "|" + destKey + "|" +
		strconv.Itoa(int(e.Port)) + "|" + strings.ToUpper(e.Protocol) + "|" + strings.ToUpper(e.Action)
}

// ConnectionLogs returns up to limit connection log entries (newest first).
func (m *LogManager) ConnectionLogs(limit int) []ConnectionLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.connLogs) {
		limit = len(m.connLogs)
	}
	out := make([]ConnectionLogEntry, 0, limit)
	for i := len(m.connLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.connLogs[i])
	}
	return out
}

// ClearConnectionLogs clears the Connection Log buffer and deduplication state.
func (m *LogManager) ClearConnectionLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connLogs = m.connLogs[:0]
	clear(m.dedupIndex)
	clear(m.udpLastSeen)
}

// LogActivity appends an entry to the Activity Log ring buffer.
func (m *LogManager) LogActivity(level, message string) ActivityLogEntry {
	level = strings.ToUpper(strings.TrimSpace(level))
	if level == "" {
		level = "INFO"
	}
	entry := ActivityLogEntry{
		ID:        m.actSeq.Add(1),
		Timestamp: time.Now().UTC().Format("15:04:05.000"),
		Level:     level,
		Message:   strings.TrimSpace(message),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.actLogs) >= MaxActivityLogs {
		drop := MaxActivityLogs / 4
		m.actLogs = append([]ActivityLogEntry(nil), m.actLogs[drop:]...)
	}
	m.actLogs = append(m.actLogs, entry)
	return entry
}

// WarnOnce logs a warning message only once per unique key (matching upstream warnOnce).
func (m *LogManager) WarnOnce(key, message string) bool {
	m.mu.Lock()
	if _, seen := m.warnedOnce[key]; seen {
		m.mu.Unlock()
		return false
	}
	m.warnedOnce[key] = struct{}{}
	m.mu.Unlock()

	m.LogActivity("WARN", message)
	return true
}

// ActivityLogs returns up to limit activity log entries (newest first).
func (m *LogManager) ActivityLogs(limit int) []ActivityLogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.actLogs) {
		limit = len(m.actLogs)
	}
	out := make([]ActivityLogEntry, 0, limit)
	for i := len(m.actLogs) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.actLogs[i])
	}
	return out
}

// ClearActivityLogs clears the Activity Log buffer.
func (m *LogManager) ClearActivityLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actLogs = m.actLogs[:0]
	clear(m.warnedOnce)
}

func matchesLogFilters(entry ConnectionLogEntry, filters []pbprofile.LogFilter) bool {
	if len(filters) == 0 {
		return true
	}

	hasInclude := false
	matchedInclude := false

	for _, f := range filters {
		typ := strings.ToUpper(strings.TrimSpace(f.Mode))
		if typ == "EXCLUDE" {
			if matchSingleFilter(entry, f) {
				return false
			}
		} else if typ == "INCLUDE" {
			hasInclude = true
			if matchSingleFilter(entry, f) {
				matchedInclude = true
			}
		}
	}

	if hasInclude {
		return matchedInclude
	}
	return true
}

func matchSingleFilter(e ConnectionLogEntry, f pbprofile.LogFilter) bool {
	if !matchFilterField(f.Process, e.ProcessName) {
		return false
	}
	if !matchFilterTarget(f.Host, e.DestIP, e.Host) {
		return false
	}
	if !matchFilterPort(f.Port, e.Port) {
		return false
	}
	if !matchFilterField(f.Protocol, e.Protocol) {
		return false
	}
	if !matchFilterField(f.Action, e.Action) && !matchFilterField(f.Action, e.ProxyName) {
		return false
	}
	return true
}

func globMatchFold(pattern, val string) bool {
	return rules.GlobMatch(strings.ToLower(pattern), strings.ToLower(val))
}

func matchFilterField(spec, val string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" || strings.EqualFold(spec, "ALL") || strings.EqualFold(spec, "BOTH") {
		return true
	}
	for _, tok := range splitTokens(spec) {
		if strings.EqualFold(tok, "ALL") || strings.EqualFold(tok, "BOTH") || globMatchFold(tok, val) {
			return true
		}
	}
	return false
}

func matchFilterTarget(spec, destIP, host string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" || strings.EqualFold(spec, "ALL") {
		return true
	}
	for _, tok := range splitTokens(spec) {
		if (destIP != "" && globMatchFold(tok, destIP)) ||
			(host != "" && globMatchFold(tok, host)) {
			return true
		}
	}
	return false
}

func matchFilterPort(spec string, port uint16) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "*" || strings.EqualFold(spec, "ALL") {
		return true
	}
	portStr := strconv.Itoa(int(port))
	for _, tok := range splitTokens(spec) {
		if dash := strings.IndexByte(tok, '-'); dash >= 0 {
			lo, err1 := strconv.Atoi(strings.TrimSpace(tok[:dash]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(tok[dash+1:]))
			if err1 == nil && err2 == nil && int(port) >= lo && int(port) <= hi {
				return true
			}
		} else if globMatchFold(tok, portStr) {
			return true
		}
	}
	return false
}

func splitTokens(spec string) []string {
	parts := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ';' || r == ','
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
