// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/D0bby999/anyproxy/internal/driver"

type trayState struct {
	Active      bool
	StatusLabel string
	ToggleLabel string
	Tooltip     string
}

func formatTrayState(st driver.EngineStatus) trayState {
	switch st.State {
	case driver.StateRunning:
		return trayState{
			Active:      true,
			StatusLabel: "Status: Running",
			ToggleLabel: "Disable AnyProxy",
			Tooltip:     "AnyProxy — Running (127.0.0.1:10800)",
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
