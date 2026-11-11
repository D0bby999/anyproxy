// SPDX-License-Identifier: MIT

package inbound

import (
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProcessInfo describes the local process that initiated an inbound proxy connection.
type ProcessInfo struct {
	PID  int
	Name string
	Path string
}

// ProcessResolverFunc resolves a client socket address to its owning local process.
type ProcessResolverFunc func(clientAddr net.Addr) ProcessInfo

type pidResolver struct {
	mu       sync.Mutex
	pidCache map[int]ProcessInfo
	snap     string
	snapAt   time.Time                            // when the netstat that produced snap started
	inflight chan struct{}                        // non-nil while a netstat runs
	run      func(args ...string) ([]byte, error) // exec.Command in prod, fake in tests
}

func newPIDResolver() *pidResolver {
	return &pidResolver{
		pidCache: make(map[int]ProcessInfo, 256),
		run:      func(a ...string) ([]byte, error) { return exec.Command("netstat", a...).Output() },
	}
}

// snapshotSince returns a netstat snapshot whose run started at or after t.
func (r *pidResolver) snapshotSince(t time.Time, args ...string) string {
	for {
		r.mu.Lock()
		if !r.snapAt.Before(t) {
			s := r.snap
			r.mu.Unlock()
			return s
		}
		if ch := r.inflight; ch != nil {
			r.mu.Unlock()
			<-ch
			continue
		}
		ch := make(chan struct{})
		r.inflight = ch
		r.mu.Unlock()

		start := time.Now()
		out, _ := r.run(args...)
		r.mu.Lock()
		r.snap, r.snapAt, r.inflight = string(out), start, nil
		r.mu.Unlock()
		close(ch)
		return string(out)
	}
}

func (r *pidResolver) Resolve(clientAddr net.Addr) ProcessInfo {
	t := time.Now()
	_, port := parseHostPort(clientAddr)
	if port == 0 {
		return ProcessInfo{Name: "unknown"}
	}

	switch runtime.GOOS {
	case "darwin":
		snap := r.snapshotSince(t, "-anv", "-p", "tcp")
		pid, shortName := parseDarwinNetstatPort(snap, port)
		if pid <= 0 {
			return ProcessInfo{Name: "unknown"}
		}
		return r.resolveDarwinPID(pid, shortName)

	case "windows":
		snap := r.snapshotSince(t, "-ano", "-p", "tcp")
		pid := parseWindowsNetstatPort(snap, port)
		if pid <= 0 {
			return ProcessInfo{Name: "unknown"}
		}
		return r.resolveWindowsPID(pid)

	default:
		return ProcessInfo{Name: "unknown"}
	}
}

func parseDarwinNetstatPort(output string, srcPort uint16) (int, string) {
	portSuffix := "." + strconv.Itoa(int(srcPort))
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, portSuffix) {
			continue
		}
		fields := strings.Fields(line)
		// tcp4 0 0 127.0.0.1.58942 127.0.0.1.10800 ESTABLISHED rx tx rhi shi process:pid ...
		if len(fields) < 11 {
			continue
		}
		localAddr := fields[3]
		if !strings.HasSuffix(localAddr, portSuffix) {
			continue
		}
		// Find the token containing "<name>:<pid>" after the 4 numeric buffer columns (fields[6..9]).
		// Process names can contain spaces (e.g., "Google Chrome He:752"), so scan from index 10 onwards.
		for i := 10; i < len(fields); i++ {
			tok := fields[i]
			colon := strings.LastIndexByte(tok, ':')
			if colon <= 0 || colon == len(tok)-1 {
				continue
			}
			pid, err := strconv.Atoi(tok[colon+1:])
			if err != nil || pid <= 0 {
				continue
			}
			nameParts := append([]string(nil), fields[10:i]...)
			nameParts = append(nameParts, tok[:colon])
			return pid, strings.TrimSpace(strings.Join(nameParts, " "))
		}
	}
	return 0, ""
}

func (r *pidResolver) resolveDarwinPID(pid int, fallbackName string) ProcessInfo {
	r.mu.Lock()
	if cached, ok := r.pidCache[pid]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()

	info := ProcessInfo{PID: pid, Name: fallbackName}
	if out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output(); err == nil {
		full := strings.TrimSpace(string(out))
		if full != "" {
			info.Path = full
			info.Name = friendlyProcessName(full, fallbackName)
		}
	}
	if info.Name == "" {
		info.Name = "unknown"
	}

	r.mu.Lock()
	if len(r.pidCache) >= 256 {
		clear(r.pidCache)
	}
	r.pidCache[pid] = info
	r.mu.Unlock()
	return info
}

func friendlyProcessName(fullPath, fallback string) string {
	// Extract macOS bundle name if inside FirstName.app/Contents/...
	if idx := strings.Index(fullPath, ".app/"); idx > 0 {
		prefix := fullPath[:idx]
		if slash := strings.LastIndexByte(prefix, '/'); slash >= 0 {
			return prefix[slash+1:]
		}
		return prefix
	}
	base := filepath.Base(fullPath)
	if base != "" && base != "." && base != "/" {
		return base
	}
	return fallback
}

func parseWindowsNetstatPort(output string, srcPort uint16) int {
	portSuffix := ":" + strconv.Itoa(int(srcPort))
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, portSuffix) {
			continue
		}
		fields := strings.Fields(line)
		// TCP 127.0.0.1:58942 127.0.0.1:10800 ESTABLISHED 1234
		if len(fields) < 5 {
			continue
		}
		if !strings.HasSuffix(fields[1], portSuffix) {
			continue
		}
		if pid, err := strconv.Atoi(fields[len(fields)-1]); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func (r *pidResolver) resolveWindowsPID(pid int) ProcessInfo {
	r.mu.Lock()
	if cached, ok := r.pidCache[pid]; ok {
		r.mu.Unlock()
		return cached
	}
	r.mu.Unlock()

	info := ProcessInfo{PID: pid, Name: "unknown"}
	if out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output(); err == nil {
		line := strings.TrimSpace(string(out))
		// "chrome.exe","1234","Console","1","120,000 K"
		if strings.HasPrefix(line, `"`) {
			if end := strings.Index(line[1:], `"`); end > 0 {
				info.Name = line[1 : 1+end]
				info.Path = info.Name
			}
		}
	}

	r.mu.Lock()
	if len(r.pidCache) >= 256 {
		clear(r.pidCache)
	}
	r.pidCache[pid] = info
	r.mu.Unlock()
	return info
}
