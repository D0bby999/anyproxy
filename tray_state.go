// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"strings"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

const defaultTrayListenAddr = "127.0.0.1:10800"

type trayState struct {
	Active      bool
	StatusLabel string
	ToggleLabel string
	Tooltip     string
}

type trayProxyItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type traySnapshot struct {
	Active          bool            `json:"active"`
	StatusLabel     string          `json:"statusLabel"`
	ToggleLabel     string          `json:"toggleLabel"`
	Tooltip         string          `json:"tooltip"`
	ListenAddr      string          `json:"listenAddr"`
	ActiveProxyText string          `json:"activeProxyText"`
	DefaultProxyID  string          `json:"defaultProxyId"`
	ActiveProfile   string          `json:"activeProfile"`
	Compact         bool            `json:"compact"`
	Proxies         []trayProxyItem `json:"proxies"`
	Profiles        []string        `json:"profiles"`
}

func formatTrayState(st driver.EngineStatus) trayState {
	switch st.State {
	case driver.StateRunning:
		return trayState{
			Active:      true,
			StatusLabel: "Status: Running",
			ToggleLabel: "Disable AnyProxy",
			Tooltip:     "AnyProxy — Running (" + defaultTrayListenAddr + ")",
		}
	case driver.StateStarting:
		return trayState{
			Active:      true,
			StatusLabel: "Status: Starting…",
			ToggleLabel: "Disable AnyProxy",
			Tooltip:     "AnyProxy — Starting…",
		}
	case driver.StatePaused:
		return trayState{
			Active:      false,
			StatusLabel: "Status: Paused",
			ToggleLabel: "Enable AnyProxy",
			Tooltip:     "AnyProxy — Paused",
		}
	case driver.StateError:
		return trayState{
			Active:      false,
			StatusLabel: "Status: Error",
			ToggleLabel: "Enable AnyProxy",
			Tooltip:     "AnyProxy — Error",
		}
	default:
		return trayState{
			Active:      false,
			StatusLabel: "Status: Stopped",
			ToggleLabel: "Enable AnyProxy",
			Tooltip:     "AnyProxy — Stopped",
		}
	}
}

func truncateTrayText(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

func formatTrayProxyLabel(px pbprofile.ProxyConfig) string {
	endpoint := fmt.Sprintf("%s %s:%d", strings.ToUpper(string(px.Type)), px.Host, px.Port)
	if name := strings.TrimSpace(px.Name); name != "" {
		return truncateTrayText(fmt.Sprintf("%s (%s)", name, endpoint), 48)
	}
	return truncateTrayText(endpoint, 48)
}

func buildTraySnapshot(st driver.EngineStatus, listenAddr string, prof *pbprofile.Profile, profileNames []string, compact bool) traySnapshot {
	addr := strings.TrimSpace(listenAddr)
	if addr == "" {
		addr = defaultTrayListenAddr
	}
	base := formatTrayState(st)
	if st.State == driver.StateRunning {
		base.Tooltip = "AnyProxy — Running (" + addr + ")"
	}

	snap := traySnapshot{
		Active:          base.Active,
		StatusLabel:     base.StatusLabel,
		ToggleLabel:     base.ToggleLabel,
		Tooltip:         base.Tooltip,
		ListenAddr:      addr,
		ActiveProxyText: "Direct",
		ActiveProfile:   "Default",
		Compact:         compact,
		Proxies:         []trayProxyItem{},
		Profiles:        []string{},
	}

	if prof != nil {
		if name := strings.TrimSpace(prof.Name); name != "" {
			snap.ActiveProfile = name
		}
		snap.DefaultProxyID = prof.DefaultProxyID
		for _, px := range prof.Proxies {
			snap.Proxies = append(snap.Proxies, trayProxyItem{
				ID:    px.ID,
				Label: formatTrayProxyLabel(px),
			})
			if px.ID == prof.DefaultProxyID && prof.DefaultProxyID != "" {
				short := strings.TrimSpace(px.Name)
				if short == "" {
					short = fmt.Sprintf("%s %s:%d", strings.ToUpper(string(px.Type)), px.Host, px.Port)
				}
				snap.ActiveProxyText = truncateTrayText(short, 24)
			}
		}
		if snap.DefaultProxyID != "" && snap.ActiveProxyText == "Direct" {
			snap.DefaultProxyID = ""
		}
	}

	seen := make(map[string]struct{}, len(profileNames)+1)
	for _, n := range profileNames {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		snap.Profiles = append(snap.Profiles, n)
	}
	if snap.ActiveProfile != "" {
		if _, ok := seen[snap.ActiveProfile]; !ok {
			snap.Profiles = append(snap.Profiles, snap.ActiveProfile)
		}
	}
	return snap
}
