// SPDX-License-Identifier: AGPL-3.0-or-later

// Package service coordinates SQLite profile persistence, the encrypted secrets
// vault, the rule & DNS engines, the 5-step proxy checker, and the OS interception driver.
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/D0bby999/anyproxy/internal/dnsmap"
	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/flows"
	"github.com/D0bby999/anyproxy/internal/inbound"
	"github.com/D0bby999/anyproxy/internal/netproxy"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/store"
)

func validateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name cannot be empty")
	}
	if len(name) > 128 {
		return errors.New("profile name exceeds 128 characters")
	}
	p := pbprofile.DefaultProfile(name)
	return pbprofile.ValidateProfile(&p)
}

// ProfileListSummary lists all saved profile names and the active profile name.
type ProfileListSummary struct {
	Names  []string `json:"names"`
	Active string   `json:"active"`
}

// Service is the central application controller shared by the Wails GUI and headless CLI.
type Service struct {
	mu         sync.Mutex
	store      *store.Store
	vault      secrets.Store
	dns        *dnsmap.Store
	tracker    *flows.Tracker
	logs       *flows.LogManager
	drv        driver.Driver
	inbound    *inbound.Server
	inboundErr error
	onEvent    func(topic string, payload any)
}

// New creates a Service backed by store, vault, and driver. If drv is nil, NewPlatformDriver is used.
func New(st *store.Store, vault secrets.Store, drv driver.Driver) *Service {
	dns := dnsmap.New(0)
	_ = dns.LoadHostsFile("/etc/hosts")
	tracker := flows.NewTracker(flows.MaxLiveFlows)
	logs := flows.NewLogManager()
	if drv == nil {
		drv = driver.NewPlatformDriver(dns, tracker, logs)
	}
	inboundSrv := inbound.NewServer(inbound.Config{
		ListenAddr: inbound.DefaultListenAddr,
		Vault:      vault,
		DNS:        dns,
		Tracker:    tracker,
		Logs:       logs,
	})
	inboundErr := inboundSrv.Start()
	if inboundErr != nil {
		logs.LogActivity("ERROR", inboundErr.Error())
	}

	s := &Service{
		store:      st,
		vault:      vault,
		dns:        dns,
		tracker:    tracker,
		logs:       logs,
		drv:        drv,
		inbound:    inboundSrv,
		inboundErr: inboundErr,
	}
	_ = s.syncActiveProfileToDriver()
	return s
}

// SetEventEmitter registers a callback (such as Wails runtime.EventsEmit) for UI notifications.
func (s *Service) SetEventEmitter(fn func(topic string, payload any)) {
	s.mu.Lock()
	s.onEvent = fn
	s.mu.Unlock()
}

func (s *Service) emit(topic string, payload any) {
	s.mu.Lock()
	fn := s.onEvent
	s.mu.Unlock()
	if fn != nil {
		fn(topic, payload)
	}
}

const settingSysProxyOn = "sysproxy_on"

// Close stops the OS interception driver and closes the store.
func (s *Service) Close() error {
	_ = s.drv.Stop()
	if s.store != nil {
		_ = s.store.SetSetting(settingSysProxyOn, "0")
	}
	if s.inbound != nil {
		_ = s.inbound.Stop()
	}
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

// RecoverSystemProxy turns off a system proxy left on by a previous crash.
func (s *Service) RecoverSystemProxy() {
	if s.store == nil {
		return
	}
	if v, _ := s.store.GetSetting(settingSysProxyOn); v != "1" {
		return
	}
	_ = s.drv.Stop()
	_ = s.store.SetSetting(settingSysProxyOn, "0")
	if s.logs != nil {
		s.logs.LogActivity("WARN", "Turned off a system proxy left on by a previous session")
	}
}

// InboundAddr returns the local forward proxy listening endpoint (e.g. "127.0.0.1:10800").
func (s *Service) InboundAddr() string {
	if s.inbound != nil {
		return s.inbound.ListenAddr()
	}
	return ""
}

// DNS returns the shared DNS cache store.
func (s *Service) DNS() *dnsmap.Store { return s.dns }

// Tracker returns the shared live flow tracker.
func (s *Service) Tracker() *flows.Tracker { return s.tracker }

// Logs returns the shared connection and activity log manager.
func (s *Service) Logs() *flows.LogManager { return s.logs }

// Driver returns the active OS interception driver.
func (s *Service) Driver() driver.Driver { return s.drv }

// ListProfiles returns all profile names and the active profile name.
func (s *Service) ListProfiles() (ProfileListSummary, error) {
	names, active, err := s.store.ListProfileNames()
	if err != nil {
		return ProfileListSummary{}, err
	}
	return ProfileListSummary{Names: names, Active: active}, nil
}

// ActiveProfile returns the currently active profile.
func (s *Service) ActiveProfile() (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	s.fillProxyPasswordIndicators(&p)
	return p, nil
}

// SwitchProfile activates the named profile and synchronizes its rules and proxies to the OS driver.
func (s *Service) SwitchProfile(name string) (pbprofile.Profile, error) {
	if err := s.store.SetActiveProfile(name); err != nil {
		return pbprofile.Profile{}, err
	}
	if err := s.syncActiveProfileToDriver(); err != nil {
		return pbprofile.Profile{}, err
	}
	s.logs.LogActivity("INFO", "Switched active profile to "+name)
	p, err := s.ActiveProfile()
	s.emit("profile:changed", p)
	return p, err
}

// CreateProfile creates a new empty profile and makes it active.
func (s *Service) CreateProfile(name string) (pbprofile.Profile, error) {
	name = strings.TrimSpace(name)
	if err := validateProfileName(name); err != nil {
		return pbprofile.Profile{}, err
	}
	if _, err := s.store.GetProfileByName(name); err == nil {
		return pbprofile.Profile{}, fmt.Errorf("profile %q already exists", name)
	}
	def := pbprofile.DefaultProfile(name)
	def.ID = uuid.NewString()
	saved, err := s.store.SaveProfile(def, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.logs.LogActivity("INFO", "Created profile "+saved.Name)
	s.emit("profile:changed", saved)
	return saved, nil
}

// DuplicateProfile clones srcName into newName (including proxy passwords in the vault) and activates it.
func (s *Service) DuplicateProfile(srcName, newName string) (pbprofile.Profile, error) {
	newName = strings.TrimSpace(newName)
	if err := validateProfileName(newName); err != nil {
		return pbprofile.Profile{}, err
	}
	names, _, err := s.store.ListProfileNames()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	for _, name := range names {
		if name == newName {
			return pbprofile.Profile{}, fmt.Errorf("profile %q already exists", newName)
		}
	}
	src, err := s.store.GetProfileByName(srcName)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	s.hydratePasswords(&src)

	src.ID = uuid.NewString()
	src.Name = newName
	saved, err := s.store.SaveProfile(src, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	for i, px := range saved.Proxies {
		if i < len(src.Proxies) && src.Proxies[i].Password != "" && s.vault != nil {
			_ = s.vault.Set(secrets.ServiceProxy, px.ID, []byte(src.Proxies[i].Password))
		}
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// RenameProfile renames oldName to newName.
func (s *Service) RenameProfile(oldName, newName string) (pbprofile.Profile, error) {
	newName = strings.TrimSpace(newName)
	if err := validateProfileName(newName); err != nil {
		return pbprofile.Profile{}, err
	}
	if err := s.store.RenameProfile(oldName, newName); err != nil {
		return pbprofile.Profile{}, err
	}
	p, err := s.ActiveProfile()
	s.emit("profile:changed", p)
	return p, err
}

// DeleteProfile deletes name and returns the newly active profile.
func (s *Service) DeleteProfile(name string) (pbprofile.Profile, error) {
	prof, err := s.store.GetProfileByName(name)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	if err := s.store.DeleteProfile(name); err != nil {
		return pbprofile.Profile{}, err
	}
	if s.vault != nil {
		for _, px := range prof.Proxies {
			_ = s.vault.Delete(secrets.ServiceProxy, px.ID)
		}
	}
	_ = s.syncActiveProfileToDriver()
	p, err := s.ActiveProfile()
	s.emit("profile:changed", p)
	return p, err
}

// ImportProfileJSON parses a .pbprofile JSON document, stores its passwords in the vault, and saves it.
func (s *Service) ImportProfileJSON(raw []byte, makeActive bool) (pbprofile.Profile, error) {
	parsed, err := pbprofile.ParseProfile(bytes.NewReader(raw))
	if err != nil {
		return pbprofile.Profile{}, err
	}
	// Map original proxy index passwords after SaveProfile assigns final UUIDs
	passwordsByIdx := make([]string, len(parsed.Proxies))
	for i := range parsed.Proxies {
		passwordsByIdx[i] = parsed.Proxies[i].Password
		parsed.Proxies[i].Password = ""
	}
	saved, err := s.store.SaveProfile(parsed, makeActive)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	if s.vault != nil {
		for i, px := range saved.Proxies {
			if i < len(passwordsByIdx) && passwordsByIdx[i] != "" {
				_ = s.vault.Set(secrets.ServiceProxy, px.ID, []byte(passwordsByIdx[i]))
			}
		}
	}
	if makeActive {
		_ = s.syncActiveProfileToDriver()
	}
	s.fillProxyPasswordIndicators(&saved)
	s.logs.LogActivity("INFO", fmt.Sprintf("Imported profile %q (%d proxies, %d rules)", saved.Name, len(saved.Proxies), len(saved.Rules)))
	s.emit("profile:changed", saved)
	return saved, nil
}

// ExportProfileJSON serializes the named profile (or active profile if name is empty) as a .pbprofile JSON document.
func (s *Service) ExportProfileJSON(name string, includePasswords bool) ([]byte, error) {
	var p pbprofile.Profile
	var err error
	if strings.TrimSpace(name) == "" {
		p, err = s.store.ActiveProfile()
	} else {
		p, err = s.store.GetProfileByName(name)
	}
	if err != nil {
		return nil, err
	}
	if includePasswords {
		s.hydratePasswords(&p)
	}
	return pbprofile.MarshalProfile(p)
}

// SaveProxy adds or updates a ProxyConfig in the active profile.
func (s *Service) SaveProxy(px pbprofile.ProxyConfig) (pbprofile.Profile, error) {
	px.Type = pbprofile.ProxyType(strings.ToUpper(string(px.Type)))
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}

	rawPassword := px.Password
	px.Password = ""
	isNew := px.ID == ""
	if rawPassword != "" && rawPassword != "••••••••" {
		if s.vault == nil {
			return pbprofile.Profile{}, errors.New("cannot save proxy password: vault unavailable")
		}
		check := pbprofile.DefaultProfile("validation")
		check.Proxies = []pbprofile.ProxyConfig{{ID: "validation", Type: px.Type, Host: px.Host, Port: px.Port, Username: px.Username, Password: rawPassword}}
		if err := pbprofile.ValidateProfile(&check); err != nil {
			return pbprofile.Profile{}, err
		}
	}

	if isNew {
		px.ID = uuid.NewString()
		p.Proxies = append(p.Proxies, px)
	} else {
		found := false
		for i := range p.Proxies {
			if p.Proxies[i].ID == px.ID {
				p.Proxies[i] = px
				found = true
				break
			}
		}
		if !found {
			p.Proxies = append(p.Proxies, px)
		}
	}
	if err := pbprofile.ValidateProfile(&p); err != nil {
		return pbprofile.Profile{}, err
	}

	if p.DefaultProxyID == "" && len(p.Proxies) > 0 {
		p.DefaultProxyID = p.Proxies[0].ID
	}

	if isNew && rawPassword != "" && rawPassword != "••••••••" {
		if err := s.vault.Set(secrets.ServiceProxy, px.ID, []byte(rawPassword)); err != nil {
			return pbprofile.Profile{}, fmt.Errorf("cannot save proxy password: %w", err)
		}
	}
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		if isNew && rawPassword != "" && rawPassword != "••••••••" {
			_ = s.vault.Delete(secrets.ServiceProxy, px.ID)
		}
		return pbprofile.Profile{}, err
	}

	var secretErr error
	if s.vault != nil && !isNew {
		if rawPassword != "" && rawPassword != "••••••••" {
			secretErr = s.vault.Set(secrets.ServiceProxy, px.ID, []byte(rawPassword))
		} else if px.Username == "" && rawPassword == "" {
			secretErr = s.vault.Delete(secrets.ServiceProxy, px.ID)
		}
	}

	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	if secretErr != nil {
		return saved, fmt.Errorf("proxy saved, but credentials could not be updated: %w", secretErr)
	}
	return saved, nil
}

// DeleteProxy removes proxyID from the active profile and deletes its password from the vault.
func (s *Service) DeleteProxy(proxyID string) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	kept := make([]pbprofile.ProxyConfig, 0, len(p.Proxies))
	for _, px := range p.Proxies {
		if px.ID != proxyID {
			kept = append(kept, px)
		}
	}
	p.Proxies = kept
	if p.DefaultProxyID == proxyID {
		p.DefaultProxyID = ""
		if len(p.Proxies) > 0 {
			p.DefaultProxyID = p.Proxies[0].ID
		}
	}
	for i := range p.Rules {
		if p.Rules[i].ProxyConfigID == proxyID {
			p.Rules[i].ProxyConfigID = p.DefaultProxyID
		}
	}
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	if s.vault != nil {
		_ = s.vault.Delete(secrets.ServiceProxy, proxyID)
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// SetDefaultProxy marks proxyID as the default proxy on the active profile.
func (s *Service) SetDefaultProxy(proxyID string) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	p.DefaultProxyID = proxyID
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// TestProxy runs the 5-step Proxy Checker against proxyID (streaming each step via emit and Wails events).
func (s *Service) TestProxy(ctx context.Context, proxyID, targetHost string, targetPort uint16, emit func(netproxy.CheckStep)) (netproxy.CheckReport, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return netproxy.CheckReport{}, err
	}
	s.hydratePasswords(&p)

	var target *pbprofile.ProxyConfig
	for i := range p.Proxies {
		if p.Proxies[i].ID == proxyID {
			target = &p.Proxies[i]
			break
		}
	}
	if target == nil {
		return netproxy.CheckReport{}, fmt.Errorf("proxy %q not found", proxyID)
	}

	up := netproxy.Upstream{
		ID:                target.ID,
		Name:              target.Name,
		Type:              string(target.Type),
		Host:              target.Host,
		Port:              target.Port,
		Username:          target.Username,
		Password:          target.Password,
		SendDomainToProxy: target.SendDomainToProxy,
	}

	report := netproxy.CheckProxyEx(ctx, up, targetHost, targetPort, func(step netproxy.CheckStep) {
		if emit != nil {
			emit(step)
		}
		s.emit("proxy:check-step", step)
	})
	s.logs.LogActivity("INFO", fmt.Sprintf("Proxy check %s (%s): %s", target.Label(), report.Target, report.Summary))
	return report, nil
}

// SaveRule validates and adds or updates a ProxyRule in the active profile.
func (s *Service) SaveRule(r pbprofile.ProxyRule) (pbprofile.Profile, error) {
	r.Protocol = pbprofile.RuleProtocol(strings.ToUpper(string(r.Protocol)))
	r.Action = pbprofile.RuleAction(strings.ToUpper(string(r.Action)))
	if r.ExcludeProcess {
		hasApp := false
		for _, tok := range strings.FieldsFunc(r.ProcessNames, func(c rune) bool { return c == ';' || c == ',' }) {
			tok = strings.TrimSpace(tok)
			if tok == "*" {
				return pbprofile.Profile{}, errors.New("exclude mode cannot use '*' wildcard")
			}
			if tok != "" {
				hasApp = true
			}
		}
		if !hasApp {
			return pbprofile.Profile{}, errors.New("exclude mode requires at least one application name")
		}
	}
	if strings.TrimSpace(r.ProcessNames) == "" {
		r.ProcessNames = "*"
	}
	if strings.TrimSpace(r.TargetHosts) == "" {
		r.TargetHosts = "*"
	}
	if strings.TrimSpace(r.TargetPorts) == "" {
		r.TargetPorts = "*"
	}
	if r.Protocol == "" {
		r.Protocol = pbprofile.ProtoBoth
	}
	if r.Action == "" {
		r.Action = pbprofile.ActionProxy
	}
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	if r.Action == pbprofile.ActionProxy && r.ProxyConfigID == "" {
		r.ProxyConfigID = p.DefaultProxyID
		if r.ProxyConfigID == "" && len(p.Proxies) > 0 {
			r.ProxyConfigID = p.Proxies[0].ID
		}
	}

	if r.ID == "" {
		r.ID = uuid.NewString()
		p.Rules = append(p.Rules, r)
	} else {
		found := false
		for i := range p.Rules {
			if p.Rules[i].ID == r.ID {
				p.Rules[i] = r
				found = true
				break
			}
		}
		if !found {
			p.Rules = append(p.Rules, r)
		}
	}
	if err := pbprofile.ValidateProfile(&p); err != nil {
		return pbprofile.Profile{}, err
	}

	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// DeleteRule removes ruleID from the active profile.
func (s *Service) DeleteRule(ruleID string) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	kept := make([]pbprofile.ProxyRule, 0, len(p.Rules))
	for _, rl := range p.Rules {
		if rl.ID != ruleID {
			kept = append(kept, rl)
		}
	}
	p.Rules = kept
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// ToggleRule enables or disables ruleID in the active profile.
func (s *Service) ToggleRule(ruleID string, enabled bool) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	for i := range p.Rules {
		if p.Rules[i].ID == ruleID {
			p.Rules[i].Enabled = enabled
			break
		}
	}
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// MoveRule moves ruleID up (delta < 0) or down (delta > 0) in priority order.
func (s *Service) MoveRule(ruleID string, delta int) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	idx := -1
	for i := range p.Rules {
		if p.Rules[i].ID == ruleID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return pbprofile.Profile{}, errors.New("rule not found")
	}
	target := idx + delta
	if target < 0 || target >= len(p.Rules) {
		s.fillProxyPasswordIndicators(&p)
		return p, nil
	}
	p.Rules[idx], p.Rules[target] = p.Rules[target], p.Rules[idx]
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// CloneRule duplicates ruleID directly below the original rule.
func (s *Service) CloneRule(ruleID string) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	for i, rl := range p.Rules {
		if rl.ID == ruleID {
			clone := rl
			clone.ID = uuid.NewString()
			if clone.Name != "" {
				clone.Name = clone.Name + " (Copy)"
			}
			out := make([]pbprofile.ProxyRule, 0, len(p.Rules)+1)
			out = append(out, p.Rules[:i+1]...)
			out = append(out, clone)
			out = append(out, p.Rules[i+1:]...)
			p.Rules = out
			saved, err := s.store.SaveProfile(p, true)
			if err != nil {
				return pbprofile.Profile{}, err
			}
			_ = s.syncActiveProfileToDriver()
			s.fillProxyPasswordIndicators(&saved)
			s.emit("profile:changed", saved)
			return saved, nil
		}
	}
	return pbprofile.Profile{}, errors.New("rule not found")
}

// ImportRulesJSON parses a standalone rules JSON array and appends the rules to the active profile.
func (s *Service) ImportRulesJSON(raw []byte) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	imported, err := pbprofile.ParseRulesJSON(bytes.NewReader(raw), p.DefaultProxyID)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	for _, r := range imported {
		r.ID = uuid.NewString()
		p.Rules = append(p.Rules, r)
	}
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.logs.LogActivity("INFO", fmt.Sprintf("Imported %d rule(s) into profile %q", len(imported), saved.Name))
	s.emit("profile:changed", saved)
	return saved, nil
}

// ExportRulesJSON serializes the selected ruleIDs (or all rules in the active profile if empty).
func (s *Service) ExportRulesJSON(ruleIDs []string) ([]byte, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return nil, err
	}
	if len(ruleIDs) == 0 {
		return pbprofile.MarshalRulesJSON(p.Rules)
	}
	want := make(map[string]struct{}, len(ruleIDs))
	for _, id := range ruleIDs {
		want[id] = struct{}{}
	}
	selected := make([]pbprofile.ProxyRule, 0, len(ruleIDs))
	for _, rl := range p.Rules {
		if _, ok := want[rl.ID]; ok {
			selected = append(selected, rl)
		}
	}
	return pbprofile.MarshalRulesJSON(selected)
}

// StartEngine activates the OS interception driver and syncs the active profile.
func (s *Service) StartEngine(ctx context.Context) (driver.EngineStatus, error) {
	if s.inboundErr != nil {
		return s.Status(), s.inboundErr
	}
	if err := s.syncActiveProfileToDriver(); err != nil {
		return s.drv.Status(), err
	}
	if err := s.drv.Start(ctx); err != nil {
		return s.drv.Status(), err
	}
	if s.store != nil {
		_ = s.store.SetSetting(settingSysProxyOn, "1")
	}
	st := s.drv.Status()
	s.emit("engine:status", st)
	return st, nil
}

// StopEngine stops interception and wipes in-memory proxy credentials in the driver.
func (s *Service) StopEngine() (driver.EngineStatus, error) {
	err := s.drv.Stop()
	if s.store != nil {
		_ = s.store.SetSetting(settingSysProxyOn, "0")
	}
	st := s.drv.Status()
	s.emit("engine:status", st)
	return st, err
}

// SetEngineEnabled toggles the master pause/resume switch.
func (s *Service) SetEngineEnabled(enabled bool) (driver.EngineStatus, error) {
	err := s.drv.SetEnabled(enabled)
	st := s.drv.Status()
	s.emit("engine:status", st)
	return st, err
}

// Status returns the current OS driver status.
func (s *Service) Status() driver.EngineStatus {
	st := s.drv.Status()
	if s.inboundErr != nil {
		st.State = driver.StateError
		st.Message = s.inboundErr.Error()
	}
	return st
}

// UpdateOptions updates profile-level runtime options (traffic logging, DNS via proxy, log every connection, auto-clear).
func (s *Service) UpdateOptions(trafficLogging, dnsViaProxy, logEveryConn, autoClearLogs bool) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	p.TrafficLoggingEnabled = trafficLogging
	p.DnsViaProxy = dnsViaProxy
	p.LogEveryConnection = logEveryConn
	p.AutoClearConnectionLogs = autoClearLogs

	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// SaveLogFilters replaces the active profile's LogFilters and syncs them to the log manager.
func (s *Service) SaveLogFilters(filters []pbprofile.LogFilter) (pbprofile.Profile, error) {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return pbprofile.Profile{}, err
	}
	p.LogFilters = filters
	saved, err := s.store.SaveProfile(p, true)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	_ = s.syncActiveProfileToDriver()
	s.fillProxyPasswordIndicators(&saved)
	s.emit("profile:changed", saved)
	return saved, nil
}

// GetFlows returns up to limit live + recently closed flows.
func (s *Service) GetFlows(limit int) ([]flows.Flow, error) {
	return s.drv.GetFlows(limit)
}

// ClearClosedFlows removes closed flows from the live flow table.
func (s *Service) ClearClosedFlows() int {
	return s.drv.ClearClosedFlows()
}

// GetConnectionLogs returns up to limit connection log entries.
func (s *Service) GetConnectionLogs(limit int) ([]flows.ConnectionLogEntry, error) {
	return s.logs.ConnectionLogs(limit), nil
}

// ClearConnectionLogs clears the connection log buffer.
func (s *Service) ClearConnectionLogs() {
	s.logs.ClearConnectionLogs()
}

// GetActivityLogs returns up to limit activity log entries.
func (s *Service) GetActivityLogs(limit int) ([]flows.ActivityLogEntry, error) {
	return s.logs.ActivityLogs(limit), nil
}

// ClearActivityLogs clears the activity log buffer.
func (s *Service) ClearActivityLogs() {
	s.logs.ClearActivityLogs()
}

func (s *Service) syncActiveProfileToDriver() error {
	p, err := s.store.ActiveProfile()
	if err != nil {
		return err
	}
	s.hydratePasswords(&p)
	if s.inbound != nil {
		s.inbound.SyncProfile(p.Proxies, p.Rules, p.DefaultProxyID)
	}
	return s.drv.SyncProfile(p.Proxies, p.Rules, driver.RuntimeOptions{
		TrafficLoggingEnabled:   p.TrafficLoggingEnabled,
		DnsViaProxy:             p.DnsViaProxy,
		LogEveryConnection:      p.LogEveryConnection,
		AutoClearConnectionLogs: p.AutoClearConnectionLogs,
		DefaultProxyID:          p.DefaultProxyID,
		LogFilters:              p.LogFilters,
	})
}

func (s *Service) hydratePasswords(p *pbprofile.Profile) {
	if s.vault == nil {
		return
	}
	for i := range p.Proxies {
		if pw, err := s.vault.Get(secrets.ServiceProxy, p.Proxies[i].ID); err == nil && len(pw) > 0 {
			p.Proxies[i].Password = string(pw)
		}
	}
}

func (s *Service) fillProxyPasswordIndicators(p *pbprofile.Profile) {
	if s.vault == nil {
		return
	}
	for i := range p.Proxies {
		if pw, err := s.vault.Get(secrets.ServiceProxy, p.Proxies[i].ID); err == nil && len(pw) > 0 {
			p.Proxies[i].Password = "••••••••"
		}
	}
}
