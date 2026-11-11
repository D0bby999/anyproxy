// SPDX-License-Identifier: MIT

package driver

import (
	"context"
	"errors"

	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

type unavailableDriver struct {
	backend string
	message string
	tracker *flows.Tracker
	logs    *flows.LogManager
}

func newUnavailableDriver(backend, message string, tracker *flows.Tracker, logs *flows.LogManager) *unavailableDriver {
	if tracker == nil {
		tracker = flows.NewTracker(flows.MaxLiveFlows)
	}
	if logs == nil {
		logs = flows.NewLogManager()
	}
	return &unavailableDriver{backend: backend, message: message, tracker: tracker, logs: logs}
}

func (d *unavailableDriver) Start(context.Context) error { return errors.New(d.message) }
func (d *unavailableDriver) Stop() error                 { return nil }
func (d *unavailableDriver) SetEnabled(enabled bool) error {
	if enabled {
		return errors.New(d.message)
	}
	return nil
}
func (d *unavailableDriver) Status() EngineStatus {
	return EngineStatus{State: StateStopped, Backend: d.backend, Message: d.message}
}
func (d *unavailableDriver) SyncProfile(_ []pbprofile.ProxyConfig, _ []pbprofile.ProxyRule, opts RuntimeOptions) error {
	d.logs.SetConfig(flows.LogConfig{
		TrafficLoggingEnabled: opts.TrafficLoggingEnabled,
		LogEveryConnection:    opts.LogEveryConnection,
		AutoClearLogs:         opts.AutoClearConnectionLogs,
		Filters:               opts.LogFilters,
	})
	return nil
}
func (d *unavailableDriver) GetFlows(max int) ([]flows.Flow, error) {
	return d.tracker.GetFlows(max), nil
}
func (d *unavailableDriver) ClearClosedFlows() int { return d.tracker.ClearClosed() }
func (d *unavailableDriver) DrainLogs() ([]flows.ConnectionLogEntry, []flows.ActivityLogEntry, error) {
	return d.logs.ConnectionLogs(flows.MaxConnectionLogs), d.logs.ActivityLogs(flows.MaxActivityLogs), nil
}
