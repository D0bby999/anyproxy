// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store persists profiles, proxy configs, ordered rules, log filters,
// and app settings in an atomic owner-only JSON file. Passwords never touch
// disk here; they live in package secrets.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("not found")

type diskState struct {
	Active   string              `json:"active"`
	Profiles []pbprofile.Profile `json:"profiles"`
	Settings map[string]string   `json:"settings,omitempty"`
}

type Store struct {
	mu    sync.RWMutex
	path  string
	state diskState
}

func Open(path string) (*Store, error) {
	s := &Store{
		path: path,
		state: diskState{
			Settings: make(map[string]string),
		},
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, &s.state); err != nil {
			return nil, err
		}
		if s.state.Settings == nil {
			s.state.Settings = make(map[string]string)
		}
	}
	if len(s.state.Profiles) == 0 {
		def := pbprofile.DefaultProfile("Default")
		def.ID = uuid.NewString()
		if _, err := s.SaveProfile(def, true); err != nil {
			return nil, err
		}
		return s, nil
	}
	if !s.hasProfileLocked(s.state.Active) {
		s.state.Active = s.state.Profiles[0].Name
	}
	if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return nil }

func (s *Store) hasProfileLocked(name string) bool {
	for i := range s.state.Profiles {
		if s.state.Profiles[i].Name == name {
			return true
		}
	}
	return false
}

func cloneProfile(p pbprofile.Profile, stripPasswords bool) pbprofile.Profile {
	out := p
	out.Proxies = make([]pbprofile.ProxyConfig, len(p.Proxies))
	for i, px := range p.Proxies {
		if stripPasswords {
			px.Password = ""
		}
		out.Proxies[i] = px
	}
	out.Rules = make([]pbprofile.ProxyRule, len(p.Rules))
	copy(out.Rules, p.Rules)
	if len(p.LogFilters) > 0 {
		out.LogFilters = make([]pbprofile.LogFilter, len(p.LogFilters))
		copy(out.LogFilters, p.LogFilters)
	} else {
		out.LogFilters = nil
	}
	return out
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) ListProfileNames() (names []string, active string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names = make([]string, len(s.state.Profiles))
	for i, p := range s.state.Profiles {
		names[i] = p.Name
	}
	active = s.state.Active
	if !s.hasProfileLocked(active) && len(names) > 0 {
		active = names[0]
	}
	return names, active, nil
}

func (s *Store) ActiveProfile() (pbprofile.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.state.Profiles) == 0 {
		return pbprofile.Profile{}, ErrNotFound
	}
	for _, p := range s.state.Profiles {
		if p.Name == s.state.Active {
			return cloneProfile(p, true), nil
		}
	}
	return cloneProfile(s.state.Profiles[0], true), nil
}

func (s *Store) GetProfileByName(name string) (pbprofile.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.state.Profiles {
		if p.Name == name {
			return cloneProfile(p, true), nil
		}
	}
	return pbprofile.Profile{}, ErrNotFound
}

// SaveProfile inserts or replaces a profile and its proxies, rules, and filters atomically.
func (s *Store) SaveProfile(p pbprofile.Profile, makeActive bool) (pbprofile.Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return p, errors.New("profile name is required")
	}
	if p.FontZoom == 0 {
		p.FontZoom = 100
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existingIdx := -1
	for i := range s.state.Profiles {
		if s.state.Profiles[i].Name == p.Name {
			existingIdx = i
			p.ID = s.state.Profiles[i].ID
			break
		}
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}

	otherProxies := make(map[string]struct{})
	otherRules := make(map[string]struct{})
	for i, prof := range s.state.Profiles {
		if i == existingIdx {
			continue
		}
		for _, px := range prof.Proxies {
			otherProxies[px.ID] = struct{}{}
		}
		for _, rl := range prof.Rules {
			otherRules[rl.ID] = struct{}{}
		}
	}

	proxyIDMap := make(map[string]string, len(p.Proxies))
	for i := range p.Proxies {
		oldID := p.Proxies[i].ID
		if _, taken := otherProxies[oldID]; oldID == "" || taken {
			p.Proxies[i].ID = uuid.NewString()
		}
		if oldID != "" {
			proxyIDMap[oldID] = p.Proxies[i].ID
		}
		if p.Proxies[i].LegacyID == 0 {
			p.Proxies[i].LegacyID = i + 1
		}
	}
	if mapped, ok := proxyIDMap[p.DefaultProxyID]; ok {
		p.DefaultProxyID = mapped
	} else if p.DefaultProxyID == "" && len(p.Proxies) > 0 {
		p.DefaultProxyID = p.Proxies[0].ID
	}

	for i := range p.Rules {
		if mapped, ok := proxyIDMap[p.Rules[i].ProxyConfigID]; ok {
			p.Rules[i].ProxyConfigID = mapped
		}
		if _, taken := otherRules[p.Rules[i].ID]; p.Rules[i].ID == "" || taken {
			p.Rules[i].ID = uuid.NewString()
		}
	}
	for i := range p.LogFilters {
		if p.LogFilters[i].ID == "" {
			p.LogFilters[i].ID = uuid.NewString()
		}
	}

	clean := cloneProfile(p, true)
	if existingIdx >= 0 {
		s.state.Profiles[existingIdx] = clean
	} else {
		s.state.Profiles = append(s.state.Profiles, clean)
	}
	if makeActive || s.state.Active == "" {
		s.state.Active = p.Name
	}

	if err := s.saveLocked(); err != nil {
		return p, err
	}
	return p, nil
}

func (s *Store) SetActiveProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasProfileLocked(name) {
		return ErrNotFound
	}
	s.state.Active = name
	return s.saveLocked()
}

func (s *Store) RenameProfile(oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("new profile name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if newName != oldName && s.hasProfileLocked(newName) {
		return errors.New("profile name already exists")
	}
	idx := -1
	for i := range s.state.Profiles {
		if s.state.Profiles[i].Name == oldName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	s.state.Profiles[idx].Name = newName
	if s.state.Active == oldName {
		s.state.Active = newName
	}
	return s.saveLocked()
}

func (s *Store) DeleteProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Profiles) <= 1 {
		return errors.New("cannot delete the last remaining profile")
	}
	idx := -1
	for i := range s.state.Profiles {
		if s.state.Profiles[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	s.state.Profiles = append(s.state.Profiles[:idx], s.state.Profiles[idx+1:]...)
	if s.state.Active == name {
		s.state.Active = s.state.Profiles[0].Name
	}
	return s.saveLocked()
}

func (s *Store) GetSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state.Settings[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *Store) SetSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Settings[key] = value
	return s.saveLocked()
}
