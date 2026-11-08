// SPDX-License-Identifier: AGPL-3.0-or-later

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAndEnsure(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(EnvDataDir, tmp)
	d, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if d.Root != filepath.Clean(tmp) {
		t.Fatalf("Root = %q, want %q", d.Root, tmp)
	}
	if err := d.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for _, p := range []string{d.Root, d.Profiles(), d.Logs(), d.Backups()} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s: %v", p, err)
		}
		if perm := fi.Mode().Perm(); perm != 0o700 {
			t.Fatalf("perm %s = %o, want 0700", p, perm)
		}
	}
}

