// SPDX-License-Identifier: AGPL-3.0-or-later

// Package store persists profiles, proxy configs, ordered rules, log filters,
// and app settings in SQLite. Passwords never touch SQLite; they live in
// package secrets.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS profiles (
	id                        TEXT PRIMARY KEY,
	name                      TEXT NOT NULL UNIQUE,
	is_active                 INTEGER NOT NULL DEFAULT 0,
	traffic_logging_enabled   INTEGER NOT NULL DEFAULT 1,
	dns_via_proxy             INTEGER NOT NULL DEFAULT 1,
	log_every_connection      INTEGER NOT NULL DEFAULT 0,
	auto_clear_logs           INTEGER NOT NULL DEFAULT 1,
	default_proxy_id          TEXT NOT NULL DEFAULT '',
	font_zoom                 INTEGER NOT NULL DEFAULT 100,
	close_to_menu_bar         INTEGER NOT NULL DEFAULT 0,
	run_at_startup            INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS proxy_configs (
	id                   TEXT PRIMARY KEY,
	profile_id           TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	legacy_id            INTEGER NOT NULL DEFAULT 0,
	name                 TEXT NOT NULL DEFAULT '',
	type                 TEXT NOT NULL DEFAULT 'SOCKS5',
	host                 TEXT NOT NULL,
	port                 INTEGER NOT NULL,
	username             TEXT NOT NULL DEFAULT '',
	send_domain_to_proxy INTEGER NOT NULL DEFAULT 1,
	position             INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS proxy_rules (
	id                  TEXT PRIMARY KEY,
	profile_id          TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	name                TEXT NOT NULL DEFAULT '',
	process_names       TEXT NOT NULL DEFAULT '*',
	target_hosts        TEXT NOT NULL DEFAULT '*',
	target_domains      TEXT NOT NULL DEFAULT '',
	target_ports        TEXT NOT NULL DEFAULT '*',
	protocol            TEXT NOT NULL DEFAULT 'BOTH',
	action              TEXT NOT NULL DEFAULT 'PROXY',
	proxy_config_id     TEXT NOT NULL DEFAULT '',
	enabled             INTEGER NOT NULL DEFAULT 1,
	localhost_via_proxy INTEGER NOT NULL DEFAULT 0,
	position            INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS log_filters (
	id         TEXT PRIMARY KEY,
	profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	mode       TEXT NOT NULL DEFAULT 'INCLUDE',
	process    TEXT NOT NULL DEFAULT '',
	host       TEXT NOT NULL DEFAULT '',
	port       TEXT NOT NULL DEFAULT '',
	protocol   TEXT NOT NULL DEFAULT 'ALL',
	action     TEXT NOT NULL DEFAULT 'ALL',
	position   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !os.IsNotExist(err) {
			db.Close()
			return nil, err
		}
	}
	s := &Store{db: db}
	if err := s.ensureDefaultProfile(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) ensureDefaultProfile() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM profiles`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	def := pbprofile.DefaultProfile("Default")
	def.ID = uuid.NewString()
	_, err := s.SaveProfile(def, true)
	return err
}

func (s *Store) ListProfileNames() (names []string, active string, err error) {
	rows, err := s.db.Query(`SELECT name, is_active FROM profiles ORDER BY rowid ASC`)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var isAct int
		if err := rows.Scan(&name, &isAct); err != nil {
			return nil, "", err
		}
		names = append(names, name)
		if isAct != 0 {
			active = name
		}
	}
	return names, active, rows.Err()
}

func (s *Store) ActiveProfile() (pbprofile.Profile, error) {
	var name string
	err := s.db.QueryRow(`SELECT name FROM profiles WHERE is_active = 1 LIMIT 1`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRow(`SELECT name FROM profiles ORDER BY rowid ASC LIMIT 1`).Scan(&name)
	}
	if err != nil {
		return pbprofile.Profile{}, err
	}
	return s.GetProfileByName(name)
}

func (s *Store) GetProfileByName(name string) (pbprofile.Profile, error) {
	var p pbprofile.Profile
	var tl, dns, lec, acl, ctm, ras int
	err := s.db.QueryRow(`
		SELECT id, name, traffic_logging_enabled, dns_via_proxy, log_every_connection,
		       auto_clear_logs, default_proxy_id, font_zoom, close_to_menu_bar, run_at_startup
		FROM profiles WHERE name = ?`, name).Scan(
		&p.ID, &p.Name, &tl, &dns, &lec, &acl, &p.DefaultProxyID, &p.FontZoom, &ctm, &ras,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return pbprofile.Profile{}, ErrNotFound
	}
	if err != nil {
		return pbprofile.Profile{}, err
	}
	p.TrafficLoggingEnabled = tl != 0
	p.DnsViaProxy = dns != 0
	p.LogEveryConnection = lec != 0
	p.AutoClearConnectionLogs = acl != 0
	p.CloseToMenuBar = ctm != 0
	p.RunAtStartup = ras != 0

	pxRows, err := s.db.Query(`
		SELECT id, legacy_id, name, type, host, port, username, send_domain_to_proxy
		FROM proxy_configs WHERE profile_id = ? ORDER BY position ASC, rowid ASC`, p.ID)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	defer pxRows.Close()
	p.Proxies = []pbprofile.ProxyConfig{}
	for pxRows.Next() {
		var px pbprofile.ProxyConfig
		var typ string
		var sd int
		if err := pxRows.Scan(&px.ID, &px.LegacyID, &px.Name, &typ, &px.Host, &px.Port, &px.Username, &sd); err != nil {
			return pbprofile.Profile{}, err
		}
		px.Type = pbprofile.ProxyType(typ)
		px.SendDomainToProxy = sd != 0
		p.Proxies = append(p.Proxies, px)
	}

	rlRows, err := s.db.Query(`
		SELECT id, name, process_names, target_hosts, target_domains, target_ports,
		       protocol, action, proxy_config_id, enabled, localhost_via_proxy
		FROM proxy_rules WHERE profile_id = ? ORDER BY position ASC, rowid ASC`, p.ID)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	defer rlRows.Close()
	p.Rules = []pbprofile.ProxyRule{}
	for rlRows.Next() {
		var rl pbprofile.ProxyRule
		var proto, act string
		var en, lh int
		if err := rlRows.Scan(&rl.ID, &rl.Name, &rl.ProcessNames, &rl.TargetHosts, &rl.TargetDomains,
			&rl.TargetPorts, &proto, &act, &rl.ProxyConfigID, &en, &lh); err != nil {
			return pbprofile.Profile{}, err
		}
		rl.Protocol = pbprofile.RuleProtocol(proto)
		rl.Action = pbprofile.RuleAction(act)
		rl.Enabled = en != 0
		rl.LocalhostViaProxy = lh != 0
		p.Rules = append(p.Rules, rl)
	}

	lfRows, err := s.db.Query(`
		SELECT id, mode, process, host, port, protocol, action
		FROM log_filters WHERE profile_id = ? ORDER BY position ASC, rowid ASC`, p.ID)
	if err != nil {
		return pbprofile.Profile{}, err
	}
	defer lfRows.Close()
	for lfRows.Next() {
		var lf pbprofile.LogFilter
		if err := lfRows.Scan(&lf.ID, &lf.Mode, &lf.Process, &lf.Host, &lf.Port, &lf.Protocol, &lf.Action); err != nil {
			return pbprofile.Profile{}, err
		}
		p.LogFilters = append(p.LogFilters, lf)
	}
	return p, nil
}

// SaveProfile inserts or replaces a profile and its proxies, rules, and filters in one transaction.
func (s *Store) SaveProfile(p pbprofile.Profile, makeActive bool) (pbprofile.Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return p, errors.New("profile name is required")
	}
	if p.FontZoom == 0 {
		p.FontZoom = 100
	}
	tx, err := s.db.Begin()
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	var existingID string
	err = tx.QueryRow(`SELECT id FROM profiles WHERE name = ?`, p.Name).Scan(&existingID)
	if err == nil {
		p.ID = existingID
	} else if p.ID == "" {
		p.ID = uuid.NewString()
	}

	if makeActive {
		if _, err := tx.Exec(`UPDATE profiles SET is_active = 0`); err != nil {
			return p, err
		}
	}

	// Ensure globally unique IDs across profiles in SQLite tables while preserving rule->proxy links.
	proxyIDMap := map[string]string{}
	for i := range p.Proxies {
		oldID := p.Proxies[i].ID
		var ownerProfile string
		err := tx.QueryRow(`SELECT profile_id FROM proxy_configs WHERE id = ?`, oldID).Scan(&ownerProfile)
		if oldID == "" || (err == nil && ownerProfile != p.ID) {
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
		var ownerProfile string
		err := tx.QueryRow(`SELECT profile_id FROM proxy_rules WHERE id = ?`, p.Rules[i].ID).Scan(&ownerProfile)
		if p.Rules[i].ID == "" || (err == nil && ownerProfile != p.ID) {
			p.Rules[i].ID = uuid.NewString()
		}
	}

	isActive := 0
	if makeActive {
		isActive = 1
	} else {
		_ = tx.QueryRow(`SELECT is_active FROM profiles WHERE id = ?`, p.ID).Scan(&isActive)
	}

	_, err = tx.Exec(`
		INSERT INTO profiles (id, name, is_active, traffic_logging_enabled, dns_via_proxy,
		                      log_every_connection, auto_clear_logs, default_proxy_id,
		                      font_zoom, close_to_menu_bar, run_at_startup)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			is_active = excluded.is_active,
			traffic_logging_enabled = excluded.traffic_logging_enabled,
			dns_via_proxy = excluded.dns_via_proxy,
			log_every_connection = excluded.log_every_connection,
			auto_clear_logs = excluded.auto_clear_logs,
			default_proxy_id = excluded.default_proxy_id,
			font_zoom = excluded.font_zoom,
			close_to_menu_bar = excluded.close_to_menu_bar,
			run_at_startup = excluded.run_at_startup
	`, p.ID, p.Name, isActive, boolInt(p.TrafficLoggingEnabled), boolInt(p.DnsViaProxy),
		boolInt(p.LogEveryConnection), boolInt(p.AutoClearConnectionLogs), p.DefaultProxyID,
		p.FontZoom, boolInt(p.CloseToMenuBar), boolInt(p.RunAtStartup))
	if err != nil {
		return p, err
	}

	if _, err := tx.Exec(`DELETE FROM proxy_configs WHERE profile_id = ?`, p.ID); err != nil {
		return p, err
	}
	for i, px := range p.Proxies {
		if _, err := tx.Exec(`
			INSERT INTO proxy_configs (id, profile_id, legacy_id, name, type, host, port, username, send_domain_to_proxy, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			px.ID, p.ID, px.LegacyID, px.Name, string(px.Type), px.Host, px.Port, px.Username, boolInt(px.SendDomainToProxy), i,
		); err != nil {
			return p, err
		}
	}

	if _, err := tx.Exec(`DELETE FROM proxy_rules WHERE profile_id = ?`, p.ID); err != nil {
		return p, err
	}
	for i, rl := range p.Rules {
		if _, err := tx.Exec(`
			INSERT INTO proxy_rules (id, profile_id, name, process_names, target_hosts, target_domains,
			                         target_ports, protocol, action, proxy_config_id, enabled, localhost_via_proxy, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rl.ID, p.ID, rl.Name, rl.ProcessNames, rl.TargetHosts, rl.TargetDomains,
			rl.TargetPorts, string(rl.Protocol), string(rl.Action), rl.ProxyConfigID,
			boolInt(rl.Enabled), boolInt(rl.LocalhostViaProxy), i,
		); err != nil {
			return p, err
		}
	}

	if _, err := tx.Exec(`DELETE FROM log_filters WHERE profile_id = ?`, p.ID); err != nil {
		return p, err
	}
	for i, lf := range p.LogFilters {
		if lf.ID == "" {
			lf.ID = uuid.NewString()
		}
		if _, err := tx.Exec(`
			INSERT INTO log_filters (id, profile_id, mode, process, host, port, protocol, action, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lf.ID, p.ID, lf.Mode, lf.Process, lf.Host, lf.Port, lf.Protocol, lf.Action, i,
		); err != nil {
			return p, err
		}
	}

	return p, tx.Commit()
}

func (s *Store) SetActiveProfile(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRow(`SELECT id FROM profiles WHERE name = ?`, name).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if _, err := tx.Exec(`UPDATE profiles SET is_active = CASE WHEN id = ? THEN 1 ELSE 0 END`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RenameProfile(oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("new profile name is required")
	}
	res, err := s.db.Exec(`UPDATE profiles SET name = ? WHERE name = ?`, newName, oldName)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteProfile(name string) error {
	names, active, err := s.ListProfileNames()
	if err != nil {
		return err
	}
	if len(names) <= 1 {
		return errors.New("cannot delete the last remaining profile")
	}
	if _, err := s.db.Exec(`DELETE FROM profiles WHERE name = ?`, name); err != nil {
		return err
	}
	if active == name {
		for _, cand := range names {
			if cand != name {
				return s.SetActiveProfile(cand)
			}
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

