// SPDX-License-Identifier: MIT

//go:build windows

package windows

import (
	"encoding/binary"
	"path/filepath"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afInet                 = 2
	afInet6                = 23
	tcpTableOwnerPIDAll    = 5
	udpTableOwnerPID       = 1
	maxPIDCacheEntries     = 256
)

var (
	modIPHlpAPI             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modIPHlpAPI.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modIPHlpAPI.NewProc("GetExtendedUdpTable")
)

type pidCacheEntry struct {
	fullPath string
	baseName string
}

// ProcessResolver resolves local socket ports to owning PIDs and executable names on Windows.
type ProcessResolver struct {
	mu       sync.Mutex
	tblBuf   []byte
	pidCache map[uint32]pidCacheEntry
}

// NewProcessResolver creates a Windows socket-to-process resolver.
func NewProcessResolver() *ProcessResolver {
	return &ProcessResolver{
		tblBuf:   make([]byte, 64*1024),
		pidCache: make(map[uint32]pidCacheEntry, maxPIDCacheEntries),
	}
}

// ResolveSocket looks up the owning PID, executable basename, and full path for a local port.
func (r *ProcessResolver) ResolveSocket(srcPort uint16, isIPv6, isUDP bool) (pid int, baseName, fullPath string) {
	rawPID := r.lookupSocketPID(srcPort, isIPv6, isUDP)
	if rawPID == 0 {
		return 0, "unknown", ""
	}
	base, full := r.resolvePIDImage(rawPID)
	return int(rawPID), base, full
}

func (r *ProcessResolver) lookupSocketPID(srcPort uint16, isIPv6, isUDP bool) uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()

	af := uintptr(afInet)
	if isIPv6 {
		af = uintptr(afInet6)
	}

	size := uint32(len(r.tblBuf))
	for attempt := 0; attempt < 3; attempt++ {
		var ret uintptr
		if isUDP {
			ret, _, _ = procGetExtendedUdpTable.Call(
				uintptr(unsafe.Pointer(&r.tblBuf[0])),
				uintptr(unsafe.Pointer(&size)),
				0,
				af,
				udpTableOwnerPID,
				0,
			)
		} else {
			ret, _, _ = procGetExtendedTcpTable.Call(
				uintptr(unsafe.Pointer(&r.tblBuf[0])),
				uintptr(unsafe.Pointer(&size)),
				0,
				af,
				tcpTableOwnerPIDAll,
				0,
			)
		}
		if ret == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			if size > 16*1024*1024 {
				return 0
			}
			r.tblBuf = make([]byte, size)
			continue
		}
		if ret != 0 {
			return 0
		}
		break
	}

	buf := r.tblBuf[:size]
	if len(buf) < 4 {
		return 0
	}
	count := binary.LittleEndian.Uint32(buf[0:4])
	off := 4

	switch {
	case !isUDP && !isIPv6:
		// MIB_TCPROW_OWNER_PID: 24 bytes (dwState, dwLocalAddr, dwLocalPort, dwRemoteAddr, dwRemotePort, dwOwningPid)
		const rowSize = 24
		for i := uint32(0); i < count && off+rowSize <= len(buf); i++ {
			port := binary.BigEndian.Uint16(buf[off+8 : off+10])
			if port == srcPort {
				return binary.LittleEndian.Uint32(buf[off+20 : off+24])
			}
			off += rowSize
		}

	case !isUDP && isIPv6:
		// MIB_TCP6ROW_OWNER_PID: 56 bytes (ucLocalAddr[16], dwLocalScopeId, dwLocalPort, ucRemoteAddr[16], dwRemoteScopeId, dwRemotePort, dwState, dwOwningPid)
		const rowSize = 56
		for i := uint32(0); i < count && off+rowSize <= len(buf); i++ {
			port := binary.BigEndian.Uint16(buf[off+20 : off+22])
			if port == srcPort {
				return binary.LittleEndian.Uint32(buf[off+52 : off+56])
			}
			off += rowSize
		}

	case isUDP && !isIPv6:
		// MIB_UDPROW_OWNER_PID: 12 bytes (dwLocalAddr, dwLocalPort, dwOwningPid)
		const rowSize = 12
		for i := uint32(0); i < count && off+rowSize <= len(buf); i++ {
			port := binary.BigEndian.Uint16(buf[off+4 : off+6])
			if port == srcPort {
				return binary.LittleEndian.Uint32(buf[off+8 : off+12])
			}
			off += rowSize
		}

	case isUDP && isIPv6:
		// MIB_UDP6ROW_OWNER_PID: 28 bytes (ucLocalAddr[16], dwLocalScopeId, dwLocalPort, dwOwningPid)
		const rowSize = 28
		for i := uint32(0); i < count && off+rowSize <= len(buf); i++ {
			port := binary.BigEndian.Uint16(buf[off+20 : off+22])
			if port == srcPort {
				return binary.LittleEndian.Uint32(buf[off+24 : off+28])
			}
			off += rowSize
		}
	}

	return 0
}

func (r *ProcessResolver) resolvePIDImage(pid uint32) (baseName, fullPath string) {
	r.mu.Lock()
	if cached, ok := r.pidCache[pid]; ok {
		r.mu.Unlock()
		return cached.baseName, cached.fullPath
	}
	r.mu.Unlock()

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "System", ""
	}
	defer windows.CloseHandle(h)

	var buf [windows.MAX_PATH * 2]uint16
	sz := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &sz); err != nil || sz == 0 {
		return "System", ""
	}

	full := windows.UTF16ToString(buf[:sz])
	base := filepath.Base(full)

	r.mu.Lock()
	if len(r.pidCache) >= maxPIDCacheEntries {
		clear(r.pidCache)
	}
	r.pidCache[pid] = pidCacheEntry{fullPath: full, baseName: base}
	r.mu.Unlock()

	return base, full
}

