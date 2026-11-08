// SPDX-License-Identifier: MIT

package windows

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"
)

func buildTestIPv4TCPPacket(srcIP, dstIP netip.Addr, srcPort, dstPort uint16, flags uint8, payload []byte) []byte {
	totalLen := 20 + 20 + len(payload)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45 // Version 4, IHL 5 (20 bytes)
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen))
	pkt[8] = 64       // TTL
	pkt[9] = ProtoTCP // Protocol
	s4 := srcIP.As4()
	d4 := dstIP.As4()
	copy(pkt[12:16], s4[:])
	copy(pkt[16:20], d4[:])

	tcp := pkt[20:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	tcp[12] = 5 << 4 // Data offset = 5 (20 bytes)
	tcp[13] = flags
	copy(tcp[20:], payload)

	RecalculateChecksums(pkt, 4, 20, ProtoTCP)
	return pkt
}

func buildTestIPv6UDPPacket(srcIP, dstIP netip.Addr, srcPort, dstPort uint16, payload []byte) []byte {
	udpLen := 8 + len(payload)
	pkt := make([]byte, 40+udpLen)
	pkt[0] = 0x60 // Version 6
	binary.BigEndian.PutUint16(pkt[4:6], uint16(udpLen))
	pkt[6] = ProtoUDP // Next Header
	pkt[7] = 64       // Hop Limit
	s6 := srcIP.As16()
	d6 := dstIP.As16()
	copy(pkt[8:24], s6[:])
	copy(pkt[24:40], d6[:])

	udp := pkt[40:]
	binary.BigEndian.PutUint16(udp[0:2], srcPort)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	copy(udp[8:], payload)

	RecalculateChecksums(pkt, 6, 40, ProtoUDP)
	return pkt
}

func verifyIPv4AndL4Checksum(t *testing.T, pkt []byte, parsed ParsedPacket) {
	t.Helper()
	if got := rfc1071Checksum(0, pkt[:parsed.IPHdrLen]); got != 0 {
		t.Fatalf("invalid IPv4 header checksum residual: 0x%04x", got)
	}
	l4 := pkt[parsed.IPHdrLen:]
	var pseudo uint32
	pseudo += sumBytes(pkt[12:20])
	pseudo += uint32(parsed.Protocol)
	pseudo += uint32(len(l4))
	if got := rfc1071Checksum(pseudo, l4); got != 0 {
		t.Fatalf("invalid IPv4 L4 checksum residual: 0x%04x", got)
	}
}

func verifyIPv6L4Checksum(t *testing.T, pkt []byte, parsed ParsedPacket) {
	t.Helper()
	l4 := pkt[parsed.IPHdrLen:]
	var pseudo uint32
	pseudo += sumBytes(pkt[8:40])
	pseudo += uint32(len(l4) >> 16)
	pseudo += uint32(len(l4) & 0xFFFF)
	pseudo += uint32(parsed.Protocol)
	if got := rfc1071Checksum(pseudo, l4); got != 0 {
		t.Fatalf("invalid IPv6 L4 checksum residual: 0x%04x", got)
	}
}

func TestIPv4TCPParseRewriteAndChecksums(t *testing.T) {
	origSrc := netip.MustParseAddr("192.168.1.50")
	origDst := netip.MustParseAddr("93.184.216.34")
	pkt := buildTestIPv4TCPPacket(origSrc, origDst, 51234, 443, TCPFlagSYN, []byte("hello"))

	parsed, err := ParsePacket(pkt)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}
	if !parsed.IsFreshSYN() || parsed.IsFinOrRst() {
		t.Fatalf("expected fresh SYN, got flags 0x%02x", parsed.TCPFlags)
	}
	verifyIPv4AndL4Checksum(t, pkt, parsed)

	// Rewrite outbound to local TCP relay (34010)
	if err := RewriteToLocalRelay(pkt, parsed, DefaultTCPRelayPort); err != nil {
		t.Fatalf("RewriteToLocalRelay: %v", err)
	}
	reParsed, err := ParsePacket(pkt)
	if err != nil {
		t.Fatalf("ParsePacket after rewrite: %v", err)
	}
	if reParsed.SrcIP != origDst || reParsed.DstIP != origSrc || reParsed.DstPort != DefaultTCPRelayPort {
		t.Fatalf("unexpected rewritten fields: %+v", reParsed)
	}
	verifyIPv4AndL4Checksum(t, pkt, reParsed)

	// Rewrite inbound from local TCP relay back to client
	replyPkt := buildTestIPv4TCPPacket(origSrc, origDst, DefaultTCPRelayPort, 51234, TCPFlagSYN|TCPFlagACK, nil)
	replyParsed, err := ParsePacket(replyPkt)
	if err != nil {
		t.Fatalf("ParsePacket reply: %v", err)
	}
	if replyParsed.IsFreshSYN() {
		t.Fatalf("SYN|ACK must not be treated as fresh SYN")
	}
	if err := RewriteFromLocalRelay(replyPkt, replyParsed, origSrc, origDst, 443); err != nil {
		t.Fatalf("RewriteFromLocalRelay: %v", err)
	}
	finalParsed, err := ParsePacket(replyPkt)
	if err != nil {
		t.Fatalf("ParsePacket final: %v", err)
	}
	if finalParsed.SrcIP != origDst || finalParsed.DstIP != origSrc || finalParsed.SrcPort != 443 || finalParsed.DstPort != 51234 {
		t.Fatalf("unexpected reverse NAT fields: %+v", finalParsed)
	}
	verifyIPv4AndL4Checksum(t, replyPkt, finalParsed)
}

func TestIPv6UDPParseRewriteAndChecksums(t *testing.T) {
	origSrc := netip.MustParseAddr("2001:db8::10")
	origDst := netip.MustParseAddr("2001:4860:4860::8888")
	pkt := buildTestIPv6UDPPacket(origSrc, origDst, 60111, 53, []byte{0x12, 0x34, 0x01, 0x00, 0x00})

	parsed, err := ParsePacket(pkt)
	if err != nil {
		t.Fatalf("ParsePacket IPv6 UDP: %v", err)
	}
	verifyIPv6L4Checksum(t, pkt, parsed)

	if err := RewriteToLocalRelay(pkt, parsed, DefaultUDPRelayPort); err != nil {
		t.Fatalf("RewriteToLocalRelay IPv6: %v", err)
	}
	reParsed, err := ParsePacket(pkt)
	if err != nil {
		t.Fatalf("ParsePacket IPv6 after rewrite: %v", err)
	}
	if reParsed.DstPort != DefaultUDPRelayPort || reParsed.DstIP != origSrc || reParsed.SrcIP != origDst {
		t.Fatalf("unexpected IPv6 rewrite: %+v", reParsed)
	}
	verifyIPv6L4Checksum(t, pkt, reParsed)
}

func TestConnTableSYNResetAndProxyLoopExclusion(t *testing.T) {
	ct := NewConnTable()

	proxyAddr := netip.MustParseAddrPort("127.0.0.1:1080")
	ct.SetProxyEndpoints([]netip.AddrPort{proxyAddr})
	if !ct.IsProxyEndpoint(netip.MustParseAddr("127.0.0.1"), 1080) {
		t.Fatalf("expected 127.0.0.1:1080 to be recognized as proxy endpoint")
	}
	if ct.IsProxyEndpoint(netip.MustParseAddr("127.0.0.1"), 8080) {
		t.Fatalf("unexpected proxy endpoint match for port 8080")
	}

	// Track a TCP connection on ephemeral port 54000
	ct.TrackTCP(TrackedConn{
		Protocol:      ProtoTCP,
		ClientSrcIP:   netip.MustParseAddr("10.0.0.2"),
		ClientSrcPort: 54000,
		OrigDstIP:     netip.MustParseAddr("1.1.1.1"),
		OrigDstPort:   443,
		ProxyConfigID: "p1",
		FlowID:        "flow-1",
	})
	if got := ct.GetTCPPortDecision(54000); got != PortDecisionTracked {
		t.Fatalf("expected PortDecisionTracked, got %d", got)
	}

	// Simulate fresh SYN arriving on reused ephemeral port 54000 -> must evict stale state
	evicted := ct.ResetStaleTCPState(54000)
	if evicted == nil || evicted.FlowID != "flow-1" {
		t.Fatalf("expected ResetStaleTCPState to evict flow-1, got %+v", evicted)
	}
	if got := ct.GetTCPPortDecision(54000); got != PortDecisionUnknown {
		t.Fatalf("expected PortDecisionUnknown after reset, got %d", got)
	}

	// Verify O(1) forward and reverse UDP conntrack + idle sweep
	dstIP := netip.MustParseAddr("8.8.8.8")
	ct.TrackUDP(TrackedConn{
		Protocol:      ProtoUDP,
		ClientSrcIP:   netip.MustParseAddr("10.0.0.2"),
		ClientSrcPort: 55000,
		OrigDstIP:     dstIP,
		OrigDstPort:   53,
		ProxyConfigID: "p1",
		FlowID:        "udp-1",
	})
	if _, ok := ct.LookupUDPByClientPort(55000); !ok {
		t.Fatalf("expected LookupUDPByClientPort to find 55000")
	}
	if _, ok := ct.LookupUDPReverse(55000, dstIP, 53); !ok {
		t.Fatalf("expected LookupUDPReverse to find (55000, 8.8.8.8, 53)")
	}

	swept := ct.SweepIdle(time.Now().Add(3*time.Minute), ConnIdleTimeout)
	if len(swept) != 1 || swept[0].FlowID != "udp-1" {
		t.Fatalf("expected SweepIdle to evict udp-1, got %+v", swept)
	}
}

