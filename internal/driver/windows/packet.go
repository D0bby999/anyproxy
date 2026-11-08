// SPDX-License-Identifier: MIT

// Package windows implements the Windows WinDivert packet interception engine,
// O(1) connection tracking, socket-to-PID resolution, and the LocalSystem Windows Service.
package windows

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

const (
	ProtoTCP = 6
	ProtoUDP = 17

	TCPFlagFIN = 0x01
	TCPFlagSYN = 0x02
	TCPFlagRST = 0x04
	TCPFlagPSH = 0x08
	TCPFlagACK = 0x10

	DefaultTCPRelayPort uint16 = 34010
	DefaultUDPRelayPort uint16 = 34011
)

var (
	ErrPacketTooShort    = errors.New("packet too short")
	ErrUnsupportedIPVer  = errors.New("unsupported IP version")
	ErrUnsupportedL4     = errors.New("unsupported L4 protocol (not TCP/UDP)")
	ErrTruncatedTransport = errors.New("truncated TCP/UDP header")
)

// ParsedPacket holds decoded offsets and metadata for an IPv4 or IPv6 TCP/UDP packet.
type ParsedPacket struct {
	Version    uint8 // 4 or 6
	Protocol   uint8 // ProtoTCP (6) or ProtoUDP (17)
	IPHdrLen   int
	L4HdrLen   int
	PayloadOff int
	SrcIP      netip.Addr
	DstIP      netip.Addr
	SrcPort    uint16
	DstPort    uint16
	TCPFlags   uint8
}

// IsFreshSYN reports whether a TCP packet is a connection-initiating SYN without ACK.
func (p ParsedPacket) IsFreshSYN() bool {
	return p.Protocol == ProtoTCP && (p.TCPFlags&TCPFlagSYN) != 0 && (p.TCPFlags&TCPFlagACK) == 0
}

// IsFinOrRst reports whether a TCP packet carries FIN or RST.
func (p ParsedPacket) IsFinOrRst() bool {
	return p.Protocol == ProtoTCP && (p.TCPFlags&(TCPFlagFIN|TCPFlagRST)) != 0
}

// ParsePacket decodes an raw IPv4 or IPv6 packet containing TCP or UDP.
func ParsePacket(pkt []byte) (ParsedPacket, error) {
	if len(pkt) < 20 {
		return ParsedPacket{}, ErrPacketTooShort
	}

	ver := pkt[0] >> 4
	var out ParsedPacket
	out.Version = ver

	switch ver {
	case 4:
		ihl := int(pkt[0]&0x0F) * 4
		if ihl < 20 || len(pkt) < ihl {
			return ParsedPacket{}, ErrPacketTooShort
		}
		out.IPHdrLen = ihl
		out.Protocol = pkt[9]
		var src4, dst4 [4]byte
		copy(src4[:], pkt[12:16])
		copy(dst4[:], pkt[16:20])
		out.SrcIP = netip.AddrFrom4(src4)
		out.DstIP = netip.AddrFrom4(dst4)

	case 6:
		if len(pkt) < 40 {
			return ParsedPacket{}, ErrPacketTooShort
		}
		out.IPHdrLen = 40
		out.Protocol = pkt[6] // Next Header
		var src6, dst6 [16]byte
		copy(src6[:], pkt[8:24])
		copy(dst6[:], pkt[24:40])
		out.SrcIP = netip.AddrFrom16(src6)
		out.DstIP = netip.AddrFrom16(dst6)

	default:
		return ParsedPacket{}, ErrUnsupportedIPVer
	}

	l4 := pkt[out.IPHdrLen:]
	switch out.Protocol {
	case ProtoTCP:
		if len(l4) < 20 {
			return ParsedPacket{}, ErrTruncatedTransport
		}
		dataOff := int(l4[12]>>4) * 4
		if dataOff < 20 || len(l4) < dataOff {
			return ParsedPacket{}, ErrTruncatedTransport
		}
		out.L4HdrLen = dataOff
		out.SrcPort = binary.BigEndian.Uint16(l4[0:2])
		out.DstPort = binary.BigEndian.Uint16(l4[2:4])
		out.TCPFlags = l4[13]

	case ProtoUDP:
		if len(l4) < 8 {
			return ParsedPacket{}, ErrTruncatedTransport
		}
		out.L4HdrLen = 8
		out.SrcPort = binary.BigEndian.Uint16(l4[0:2])
		out.DstPort = binary.BigEndian.Uint16(l4[2:4])

	default:
		return ParsedPacket{}, ErrUnsupportedL4
	}

	out.PayloadOff = out.IPHdrLen + out.L4HdrLen
	return out, nil
}

// RewriteToLocalRelay rewrites an outbound packet in-place so WinDivert routes it back to the
// local relay on relayPort (swapping SrcIP/DstIP on non-loopback interfaces per WinDivert NAT pattern
// and recalculating IPv4 + TCP/UDP checksums).
func RewriteToLocalRelay(pkt []byte, parsed ParsedPacket, relayPort uint16) error {
	if len(pkt) < parsed.PayloadOff {
		return ErrPacketTooShort
	}

	switch parsed.Version {
	case 4:
		src4 := parsed.SrcIP.As4()
		// Redirect destination IP to original source IP so packet loops back to local stack,
		// and set source IP to original destination IP (standard WinDivert local proxy reflection).
		dst4 := parsed.DstIP.As4()
		copy(pkt[12:16], dst4[:])
		copy(pkt[16:20], src4[:])
	case 6:
		src6 := parsed.SrcIP.As16()
		dst6 := parsed.DstIP.As16()
		copy(pkt[8:24], dst6[:])
		copy(pkt[24:40], src6[:])
	default:
		return ErrUnsupportedIPVer
	}

	l4 := pkt[parsed.IPHdrLen:]
	binary.BigEndian.PutUint16(l4[2:4], relayPort)

	RecalculateChecksums(pkt, parsed.Version, parsed.IPHdrLen, parsed.Protocol)
	return nil
}

// RewriteFromLocalRelay rewrites a reply packet from the local relay back to the original
// destination IP and port expected by the client socket.
func RewriteFromLocalRelay(pkt []byte, parsed ParsedPacket, origSrcIP, origDstIP netip.Addr, origDstPort uint16) error {
	if len(pkt) < parsed.PayloadOff {
		return ErrPacketTooShort
	}

	switch parsed.Version {
	case 4:
		s4 := origDstIP.As4()
		d4 := origSrcIP.As4()
		copy(pkt[12:16], s4[:])
		copy(pkt[16:20], d4[:])
	case 6:
		s6 := origDstIP.As16()
		d6 := origSrcIP.As16()
		copy(pkt[8:24], s6[:])
		copy(pkt[24:40], d6[:])
	default:
		return ErrUnsupportedIPVer
	}

	l4 := pkt[parsed.IPHdrLen:]
	binary.BigEndian.PutUint16(l4[0:2], origDstPort)

	RecalculateChecksums(pkt, parsed.Version, parsed.IPHdrLen, parsed.Protocol)
	return nil
}

// RecalculateChecksums computes and writes RFC 1071 IPv4 header and TCP/UDP transport checksums in-place.
func RecalculateChecksums(pkt []byte, version uint8, ipHdrLen int, proto uint8) {
	if version == 4 && len(pkt) >= ipHdrLen {
		pkt[10] = 0
		pkt[11] = 0
		ipSum := rfc1071Checksum(0, pkt[:ipHdrLen])
		binary.BigEndian.PutUint16(pkt[10:12], ipSum)
	}

	l4 := pkt[ipHdrLen:]
	l4Len := len(l4)

	switch proto {
	case ProtoTCP:
		if l4Len < 20 {
			return
		}
		l4[16] = 0
		l4[17] = 0
	case ProtoUDP:
		if l4Len < 8 {
			return
		}
		l4[6] = 0
		l4[7] = 0
	default:
		return
	}

	var pseudo uint32
	if version == 4 {
		pseudo = sumBytes(pkt[12:20])
		pseudo += uint32(proto)
		pseudo += uint32(l4Len)
	} else {
		pseudo = sumBytes(pkt[8:40])
		pseudo += uint32(l4Len >> 16)
		pseudo += uint32(l4Len & 0xFFFF)
		pseudo += uint32(proto)
	}

	csum := rfc1071Checksum(pseudo, l4)
	if proto == ProtoUDP && csum == 0 {
		csum = 0xFFFF
	}

	if proto == ProtoTCP {
		binary.BigEndian.PutUint16(l4[16:18], csum)
	} else {
		binary.BigEndian.PutUint16(l4[6:8], csum)
	}
}

func sumBytes(b []byte) uint32 {
	var sum uint32
	n := len(b)
	for i := 0; i+1 < n; i += 2 {
		sum += uint32(b[i])<<8 | uint32(b[i+1])
	}
	if n%2 == 1 {
		sum += uint32(b[n-1]) << 8
	}
	return sum
}

func rfc1071Checksum(initial uint32, data []byte) uint16 {
	sum := initial + sumBytes(data)
	for (sum >> 16) != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	return ^uint16(sum)
}

