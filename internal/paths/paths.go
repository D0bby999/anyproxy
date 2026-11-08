// SPDX-License-Identifier: AGPL-3.0-or-later

// Package paths owns the on-disk layout of AnyProxyXa's data directory.
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

// Default is ~/Library/Application Support/AnyProxyXa on macOS,
// %APPDATA%\AnyProxyXa on Windows, or the absolute path in ANYPROXY_DATA_DIR.
func Default() (Dirs, error) {
	if v := os.Getenv(EnvDataDir); v != "" && filepath.IsAbs(v) {
		return Dirs{Root: filepath.Clean(v)}, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	return Dirs{Root: filepath.Join(base, "AnyProxyXa")}, nil
}

func (d Dirs) DB() string       { return filepath.Join(d.Root, "anyproxyxa.db") }
func (d Dirs) Vault() string    { return filepath.Join(d.Root, "secrets.vault") }
func (d Dirs) Profiles() string { return filepath.Join(d.Root, "profiles") }
func (d Dirs) Logs() string     { return filepath.Join(d.Root, "logs") }
func (d Dirs) Backups() string  { return filepath.Join(d.Root, "backups") }

// Ensure creates every directory with owner-only permissions (0700).
func (d Dirs) Ensure() error {
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

