// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyproxyxa/internal/driver"
	"github.com/D0bby999/anyproxyxa/internal/flows"
	"github.com/D0bby999/anyproxyxa/internal/netproxy"
	"github.com/D0bby999/anyproxyxa/internal/pbprofile"
	"github.com/D0bby999/anyproxyxa/internal/service"
	"github.com/D0bby999/anyproxyxa/internal/update"
)

// StartupError returns any initialization error encountered during Wails startup.
func (a *App) StartupError() string {
	if a.startErr != nil {
		return a.startErr.Error()
	}
	return ""
}

// ListProfiles returns all profile names and the active profile name.
func (a *App) ListProfiles() (service.ProfileListSummary, error) {
	if err := a.ready(); err != nil {
		return service.ProfileListSummary{}, err
	}
	return a.svc.ListProfiles()
}

// ActiveProfile returns the currently active profile.
func (a *App) ActiveProfile() (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.ActiveProfile()
}

// SwitchProfile switches the active profile by name and hot-reloads the driver.
func (a *App) SwitchProfile(name string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.SwitchProfile(name)
}

// CreateProfile creates a new empty profile and makes it active.
func (a *App) CreateProfile(name string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.CreateProfile(name)
}

// DuplicateProfile duplicates srcName into newName and activates it.
func (a *App) DuplicateProfile(srcName, newName string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.DuplicateProfile(srcName, newName)
}

// RenameProfile renames oldName to newName.
func (a *App) RenameProfile(oldName, newName string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.RenameProfile(oldName, newName)
}

// DeleteProfile deletes a profile by name.
func (a *App) DeleteProfile(name string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.DeleteProfile(name)
}

// ImportProfileJSON imports a .pbprofile JSON string.
func (a *App) ImportProfileJSON(raw string, makeActive bool) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.ImportProfileJSON([]byte(raw), makeActive)
}

// ExportProfileJSON exports the named profile (or active profile) as a .pbprofile JSON string.
func (a *App) ExportProfileJSON(name string, includePasswords bool) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	raw, err := a.svc.ExportProfileJSON(name, includePasswords)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// SaveProxy adds or updates a proxy server in the active profile.
func (a *App) SaveProxy(px pbprofile.ProxyConfig) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.SaveProxy(px)
}

// DeleteProxy deletes a proxy server by ID from the active profile.
func (a *App) DeleteProxy(proxyID string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.DeleteProxy(proxyID)
}

// SetDefaultProxy sets the default proxy ID on the active profile.
func (a *App) SetDefaultProxy(proxyID string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.SetDefaultProxy(proxyID)
}

// TestProxy runs the 5-step Proxy Checker against proxyID and emits step events.
func (a *App) TestProxy(proxyID, targetHost string, targetPort int) (netproxy.CheckReport, error) {
	if err := a.ready(); err != nil {
		return netproxy.CheckReport{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if targetPort <= 0 || targetPort > 65535 {
		targetPort = 80
	}
	return a.svc.TestProxy(ctx, proxyID, targetHost, uint16(targetPort), nil)
}

// SaveRule adds or updates a routing rule in the active profile.
func (a *App) SaveRule(r pbprofile.ProxyRule) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.SaveRule(r)
}

// DeleteRule deletes a routing rule by ID from the active profile.
func (a *App) DeleteRule(ruleID string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.DeleteRule(ruleID)
}

// ToggleRule enables or disables a routing rule by ID.
func (a *App) ToggleRule(ruleID string, enabled bool) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.ToggleRule(ruleID, enabled)
}

// MoveRule moves a rule up (delta < 0) or down (delta > 0) in priority order.
func (a *App) MoveRule(ruleID string, delta int) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.MoveRule(ruleID, delta)
}

// CloneRule duplicates a rule directly below the original.
func (a *App) CloneRule(ruleID string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.CloneRule(ruleID)
}

// ImportRulesJSON imports a standalone rules JSON array into the active profile.
func (a *App) ImportRulesJSON(raw string) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.ImportRulesJSON([]byte(raw))
}

// ExportRulesJSON exports the selected rule IDs (or all rules) as a JSON string.
func (a *App) ExportRulesJSON(ruleIDs []string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	raw, err := a.svc.ExportRulesJSON(ruleIDs)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// StartEngine activates the OS interception driver.
func (a *App) StartEngine() (driver.EngineStatus, error) {
	if err := a.ready(); err != nil {
		return driver.EngineStatus{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.svc.StartEngine(ctx)
}

// StopEngine stops the OS interception driver.
func (a *App) StopEngine() (driver.EngineStatus, error) {
	if err := a.ready(); err != nil {
		return driver.EngineStatus{}, err
	}
	return a.svc.StopEngine()
}

// SetEngineEnabled toggles the master interception switch.
func (a *App) SetEngineEnabled(enabled bool) (driver.EngineStatus, error) {
	if err := a.ready(); err != nil {
		return driver.EngineStatus{}, err
	}
	return a.svc.SetEngineEnabled(enabled)
}

// Status returns the current OS driver status.
func (a *App) Status() driver.EngineStatus {
	if err := a.ready(); err != nil {
		return driver.EngineStatus{State: driver.StateError, Message: err.Error()}
	}
	return a.svc.Status()
}

// UpdateOptions updates profile-level runtime options.
func (a *App) UpdateOptions(trafficLogging, dnsViaProxy, logEveryConn, autoClearLogs bool) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.UpdateOptions(trafficLogging, dnsViaProxy, logEveryConn, autoClearLogs)
}

// SaveLogFilters saves the active profile's log filters.
func (a *App) SaveLogFilters(filters []pbprofile.LogFilter) (pbprofile.Profile, error) {
	if err := a.ready(); err != nil {
		return pbprofile.Profile{}, err
	}
	return a.svc.SaveLogFilters(filters)
}

// GetFlows returns up to limit live + recently closed flows.
func (a *App) GetFlows(limit int) ([]flows.Flow, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.GetFlows(limit)
}

// ClearClosedFlows removes closed flows from the live flow table.
func (a *App) ClearClosedFlows() int {
	if err := a.ready(); err != nil {
		return 0
	}
	return a.svc.ClearClosedFlows()
}

// GetConnectionLogs returns up to limit connection log entries.
func (a *App) GetConnectionLogs(limit int) ([]flows.ConnectionLogEntry, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.GetConnectionLogs(limit)
}

// ClearConnectionLogs clears the connection log buffer.
func (a *App) ClearConnectionLogs() {
	if a.svc != nil {
		a.svc.ClearConnectionLogs()
	}
}

// GetActivityLogs returns up to limit activity log entries.
func (a *App) GetActivityLogs(limit int) ([]flows.ActivityLogEntry, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.GetActivityLogs(limit)
}

// ClearActivityLogs clears the activity log buffer.
func (a *App) ClearActivityLogs() {
	if a.svc != nil {
		a.svc.ClearActivityLogs()
	}
}

// SimulateFlow evaluates a synthetic flow against the active rule engine and records it in Live Flows & Connection Logs.
func (a *App) SimulateFlow(pid int, processName, destIP, domain string, port int, isUDP bool) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	if port <= 0 || port > 65535 {
		port = 443
	}
	dec, flowID := a.svc.SimulateFlow(pid, processName, destIP, domain, uint16(port), isUDP)
	return string(dec.Action) + ":" + flowID, nil
}

// PickProcessExecutable opens a native file dialog so the user can choose an application or executable for a rule.
func (a *App) PickProcessExecutable() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Process Executable or Application Bundle",
	})
}

// CheckForUpdates retrieves the latest release info from the update manifest.
func (a *App) CheckForUpdates() (update.ReleaseInfo, error) {
	ctx := context.Background()
	if a.ctx != nil {
		ctx = a.ctx
	}
	return update.CheckForUpdates(ctx, "")
}

