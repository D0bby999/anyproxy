// SPDX-License-Identifier: MIT

package netproxy

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// StepStatus represents the status of an individual proxy diagnostic step.
type StepStatus string

const (
	StepRunning StepStatus = "running"
	StepOK      StepStatus = "ok"
	StepWarn    StepStatus = "warn"
	StepFail    StepStatus = "fail"
	StepSkip    StepStatus = "skip"
)

// CheckStep represents one step of the 5-step Proxy Checker.
type CheckStep struct {
	Step      int        `json:"step"`
	Name      string     `json:"name"`
	Status    StepStatus `json:"status"`
	LatencyMs int64      `json:"latencyMs,omitempty"`
	Message   string     `json:"message"`
}

// CheckReport is the aggregated result of CheckProxyEx.
type CheckReport struct {
	ProxyID    string      `json:"proxyId"`
	ProxyName  string      `json:"proxyName"`
	ProxyType  string      `json:"proxyType"`
	ProxyAddr  string      `json:"proxyAddr"`
	Target     string      `json:"target"`
	Success    bool        `json:"success"`
	UDPOK      bool        `json:"udpOk"`
	MinMs      int64       `json:"minMs"`
	AvgMs      int64       `json:"avgMs"`
	MaxMs      int64       `json:"maxMs"`
	Summary    string      `json:"summary"`
	Steps      []CheckStep `json:"steps"`
	FinishedAt string      `json:"finishedAt"`
}

// CheckProxyEx executes the 5-step diagnostic proxy check matching ProxyBridge's Proxy Checker:
//  1. TCP reachability to proxy host:port
//  2. Proxy authentication & tunnel setup to targetHost:targetPort
//  3. HTTP probe over tunnel (or banner check)
//  4. 3x sequential tunnel setup latency measurement (min/avg/max ms)
//  5. SOCKS5 UDP ASSOCIATE probe (warning if unsupported on SOCKS5; skipped on HTTP)
//
// If emit is non-nil, it is called as each step completes so the UI or CLI can stream live progress.
func CheckProxyEx(ctx context.Context, u Upstream, targetHost string, targetPort uint16, emit func(CheckStep)) CheckReport {
	if strings.TrimSpace(targetHost) == "" {
		targetHost = "www.google.com"
	}
	if targetPort == 0 {
		targetPort = 80
	}

	targetStr := net.JoinHostPort(targetHost, strconv.Itoa(int(targetPort)))
	report := CheckReport{
		ProxyID:   u.ID,
		ProxyName: u.Name,
		ProxyType: u.NormalizedType(),
		ProxyAddr: u.Addr(),
		Target:    targetStr,
		Steps:     make([]CheckStep, 0, 5),
	}

	record := func(step CheckStep) {
		report.Steps = append(report.Steps, step)
		if emit != nil {
			emit(step)
		}
	}

	// Step 1: TCP Reachability to Proxy Server
	t0 := time.Now()
	d := net.Dialer{Timeout: 8 * time.Second}
	rawConn, err := d.DialContext(ctx, "tcp", u.Addr())
	tcpMs := time.Since(t0).Milliseconds()
	if err != nil {
		record(CheckStep{
			Step:    1,
			Name:    "TCP Reachability",
			Status:  StepFail,
			Message: fmt.Sprintf("Cannot reach proxy %s: %v", u.Addr(), err),
		})
		report.Summary = fmt.Sprintf("Failed at Step 1 (TCP Reachability): %v", err)
		report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return report
	}
	_ = rawConn.Close()
	record(CheckStep{
		Step:      1,
		Name:      "TCP Reachability",
		Status:    StepOK,
		LatencyMs: tcpMs,
		Message:   fmt.Sprintf("Connected to %s in %d ms", u.Addr(), tcpMs),
	})

	// Step 2: Proxy Handshake & Tunnel Setup
	t1 := time.Now()
	step2Ctx, cancel2 := context.WithTimeout(ctx, 10*time.Second)
	tunConn, err := u.DialTCP(step2Ctx, targetHost, targetHost, targetPort)
	cancel2()
	tunMs := time.Since(t1).Milliseconds()
	if err != nil {
		record(CheckStep{
			Step:    2,
			Name:    "Tunnel & Authentication",
			Status:  StepFail,
			Message: fmt.Sprintf("%s handshake to %s failed: %v", u.NormalizedType(), targetStr, err),
		})
		report.Summary = fmt.Sprintf("Failed at Step 2 (%s Handshake): %v", u.NormalizedType(), err)
		report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		return report
	}
	record(CheckStep{
		Step:      2,
		Name:      "Tunnel & Authentication",
		Status:    StepOK,
		LatencyMs: tunMs,
		Message:   fmt.Sprintf("%s tunnel established to %s in %d ms", u.NormalizedType(), targetStr, tunMs),
	})

	// Step 3: HTTP Request/Response Probe over Tunnel
	t2 := time.Now()
	_ = tunConn.SetDeadline(time.Now().Add(8 * time.Second))
	probeReq := fmt.Sprintf("HEAD / HTTP/1.1\r\nHost: %s\r\nUser-Agent: AnyProxyXA/1.0\r\nConnection: close\r\n\r\n", targetHost)
	_, werr := tunConn.Write([]byte(probeReq))
	var step3 checkProbeResult
	if werr != nil {
		step3 = checkProbeResult{err: werr}
	} else {
		br := bufio.NewReader(tunConn)
		line, rerr := br.ReadString('\n')
		step3 = checkProbeResult{line: strings.TrimSpace(line), err: rerr}
	}
	_ = tunConn.Close()
	probeMs := time.Since(t2).Milliseconds()

	if step3.err != nil && step3.line == "" {
		record(CheckStep{
			Step:      3,
			Name:      "Target Data Probe",
			Status:    StepWarn,
			LatencyMs: probeMs,
			Message:   fmt.Sprintf("Tunnel opened, but HTTP probe read returned: %v", step3.err),
		})
	} else {
		record(CheckStep{
			Step:      3,
			Name:      "Target Data Probe",
			Status:    StepOK,
			LatencyMs: probeMs,
			Message:   fmt.Sprintf("Received response %q in %d ms", step3.line, probeMs),
		})
	}

	// Step 4: 3x Sequential Tunnel Setup Latency Measurement
	const samples = 3
	var minMs, maxMs, sumMs int64
	var okSamples int64
	for i := 0; i < samples; i++ {
		ts := time.Now()
		sCtx, sCancel := context.WithTimeout(ctx, 8*time.Second)
		c, serr := u.DialTCP(sCtx, targetHost, targetHost, targetPort)
		sCancel()
		dt := time.Since(ts).Milliseconds()
		if serr == nil {
			_ = c.Close()
			if okSamples == 0 || dt < minMs {
				minMs = dt
			}
			if dt > maxMs {
				maxMs = dt
			}
			sumMs += dt
			okSamples++
		}
	}
	if okSamples > 0 {
		avgMs := sumMs / okSamples
		report.MinMs = minMs
		report.AvgMs = avgMs
		report.MaxMs = maxMs
		record(CheckStep{
			Step:      4,
			Name:      "Latency Benchmark (3x)",
			Status:    StepOK,
			LatencyMs: avgMs,
			Message:   fmt.Sprintf("%d/%d succeeded — min %d ms / avg %d ms / max %d ms", okSamples, samples, minMs, avgMs, maxMs),
		})
	} else {
		record(CheckStep{
			Step:    4,
			Name:    "Latency Benchmark (3x)",
			Status:  StepWarn,
			Message: "0/3 benchmark samples succeeded after initial tunnel",
		})
	}

	// Step 5: SOCKS5 UDP ASSOCIATE Probe
	if u.NormalizedType() == "SOCKS5" {
		t5 := time.Now()
		uCtx, uCancel := context.WithTimeout(ctx, 8*time.Second)
		assoc, uerr := u.DialUDP(uCtx)
		uCancel()
		udpMs := time.Since(t5).Milliseconds()
		if uerr != nil {
			record(CheckStep{
				Step:      5,
				Name:      "SOCKS5 UDP Associate",
				Status:    StepWarn,
				LatencyMs: udpMs,
				Message:   fmt.Sprintf("TCP works, but UDP ASSOCIATE is not supported by this SOCKS5 server: %v", uerr),
			})
		} else {
			relayStr := assoc.RelayAddr().String()
			_ = assoc.Close()
			report.UDPOK = true
			record(CheckStep{
				Step:      5,
				Name:      "SOCKS5 UDP Associate",
				Status:    StepOK,
				LatencyMs: udpMs,
				Message:   fmt.Sprintf("UDP relay bound at %s in %d ms", relayStr, udpMs),
			})
		}
	} else {
		record(CheckStep{
			Step:    5,
			Name:    "SOCKS5 UDP Associate",
			Status:  StepSkip,
			Message: "Skipped for HTTP proxy (TCP only)",
		})
	}

	report.Success = true
	if report.AvgMs > 0 {
		report.Summary = fmt.Sprintf("%s proxy OK (avg %d ms)", u.NormalizedType(), report.AvgMs)
	} else {
		report.Summary = fmt.Sprintf("%s proxy OK (%d ms)", u.NormalizedType(), tunMs)
	}
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	return report
}

type checkProbeResult struct {
	line string
	err  error
}

