// SPDX-License-Identifier: AGPL-3.0-or-later

// Package rules compiles and evaluates ProxyBridge traffic routing rules.
package rules

import "net/netip"

// IsLoopbackAddr reports whether addr is IPv4 127.0.0.0/8, IPv6 ::1, or IPv4-mapped ::ffff:127.x.x.x.
func IsLoopbackAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	u := addr.Unmap()
	return u.IsLoopback()
}

// ShouldBypassProxy reports whether a PROXY decision must be overridden to DIRECT
// to preserve system network stability (broadcast, multicast, link-local/APIPA,
// site-local, unspecified, DHCP/DHCPv6, stub DNS on loopback:53, or loopback when
// localhostViaProxy is false).
func ShouldBypassProxy(destIP string, destPort uint16, isUDP bool, localhostViaProxy bool) bool {
	if isUDP && (destPort == 67 || destPort == 68 || destPort == 546 || destPort == 547) {
		return true
	}
	addr, err := netip.ParseAddr(destIP)
	if err != nil {
		return false
	}
	u := addr.Unmap()
	if u.IsLoopback() {
		// Never proxy DNS to a loopback resolver stub (e.g. 127.0.0.53:53)
		if destPort == 53 {
			return true
		}
		return !localhostViaProxy
	}
	if u.IsUnspecified() || u.IsMulticast() || u.IsLinkLocalUnicast() || u.IsLinkLocalMulticast() {
		return true
	}
	if u.Is4() {
		b := u.As4()
		// 0.0.0.0/8, 255.255.255.255, or subnet broadcast x.x.x.255
		if b[0] == 0 || b[3] == 255 {
			return true
		}
		return false
	}
	// IPv6 deprecated site-local fec0::/10
	b16 := u.As16()
	if b16[0] == 0xfe && (b16[1]&0xc0) == 0xc0 {
		return true
	}
	return false
}

