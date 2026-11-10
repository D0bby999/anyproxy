// SPDX-License-Identifier: AGPL-3.0-or-later

// Package paths owns the on-disk layout of AnyProxy's data directory.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dirs is the data layout rooted at one directory.
type Dirs struct {
	Root string
}

// EnvDataDir overrides the data directory with an absolute path.
const EnvDataDir = "ANYPROXY_DATA_DIR"

// Default is ~/Library/Application Support/AnyProxy on macOS,
// %APPDATA%\AnyProxy on Windows, or the absolute path in ANYPROXY_DATA_DIR.
func Default() (Dirs, error) {
	if v := os.Getenv(EnvDataDir); v != "" && filepath.IsAbs(v) {
		return Dirs{Root: filepath.Clean(v)}, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	return defaultForBase(base)
}

func defaultForBase(base string) (Dirs, error) {
	current, legacy := filepath.Join(base, "AnyProxy"), filepath.Join(base, "AnyProxyXa")
	_, oldErr := os.Lstat(legacy)
	_, newErr := os.Lstat(current)
	if oldErr != nil && !os.IsNotExist(oldErr) {
		return Dirs{}, fmt.Errorf("inspect legacy data directory: %w", oldErr)
	}
	if newErr != nil && !os.IsNotExist(newErr) {
		return Dirs{}, fmt.Errorf("inspect data directory: %w", newErr)
	}
	if oldErr == nil && newErr == nil {
		return Dirs{}, fmt.Errorf("both AnyProxy and legacy data directories exist; resolve manually before starting")
	}
	if oldErr == nil {
		return Dirs{Root: legacy}, nil
	}
	return Dirs{Root: current}, nil
}

func (d Dirs) DB() string {
	legacy := filepath.Join(d.Root, "anyproxyxa.db")
	if _, err := os.Lstat(legacy); err == nil || !os.IsNotExist(err) {
		return legacy
	}
	return filepath.Join(d.Root, "anyproxy.db")
}
func (d Dirs) Vault() string    { return filepath.Join(d.Root, "secrets.vault") }
func (d Dirs) Profiles() string { return filepath.Join(d.Root, "profiles") }
func (d Dirs) Logs() string     { return filepath.Join(d.Root, "logs") }
func (d Dirs) Backups() string  { return filepath.Join(d.Root, "backups") }

// Ensure creates every directory with owner-only permissions (0700).
func (d Dirs) Ensure() error {
	_, oldErr := os.Lstat(filepath.Join(d.Root, "anyproxyxa.db"))
	_, newErr := os.Lstat(filepath.Join(d.Root, "anyproxy.db"))
	if oldErr != nil && !os.IsNotExist(oldErr) {
		return fmt.Errorf("inspect legacy database: %w", oldErr)
	}
	if newErr != nil && !os.IsNotExist(newErr) {
		return fmt.Errorf("inspect database: %w", newErr)
	}
	if oldErr == nil && newErr == nil {
		return fmt.Errorf("both AnyProxy and legacy databases exist; resolve manually before starting")
	}
	for _, p := range []string{d.Root, d.Profiles(), d.Logs(), d.Backups()} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", p, err)
		}
		if err := os.Chmod(p, 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", p, err)
		}
	}
	return nil
}
