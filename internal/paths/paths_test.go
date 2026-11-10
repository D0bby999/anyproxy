// SPDX-License-Identifier: AGPL-3.0-or-later

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyDataFallbackAndConflict(t *testing.T) {
	base := t.TempDir()
	legacy, current := filepath.Join(base, "AnyProxyXa"), filepath.Join(base, "AnyProxy")
	if err := os.Mkdir(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "anyproxyxa.db"), []byte("existing data"), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := defaultForBase(base)
	if err != nil || d.Root != legacy {
		t.Fatalf("legacy root = %q, %v", d.Root, err)
	}
	if got, err := os.ReadFile(d.DB()); err != nil || string(got) != "existing data" {
		t.Fatalf("legacy data missing: %q, %v", got, err)
	}
	if err := os.Mkdir(current, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := defaultForBase(base); err == nil {
		t.Fatal("expected conflict when both data directories exist")
	}
	if got, err := os.ReadFile(d.DB()); err != nil || string(got) != "existing data" {
		t.Fatalf("conflict modified data: %q, %v", got, err)
	}
}

func TestRenamedDirectoryKeepsLegacyDatabase(t *testing.T) {
	base := t.TempDir()
	legacy, current := filepath.Join(base, "AnyProxyXa"), filepath.Join(base, "AnyProxy")
	if err := os.Mkdir(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "anyproxyxa.db"), []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(legacy, current); err != nil {
		t.Fatal(err)
	}
	d, err := defaultForBase(base)
	if err != nil || d.Root != current {
		t.Fatalf("current root = %q, %v", d.Root, err)
	}
	if got, err := os.ReadFile(d.DB()); err != nil || string(got) != "profile" {
		t.Fatalf("legacy DB after rename: %q, %v", got, err)
	}
}

func TestDatabaseConflict(t *testing.T) {
	d := Dirs{Root: t.TempDir()}
	for _, name := range []string{"anyproxyxa.db", "anyproxy.db"} {
		if err := os.WriteFile(filepath.Join(d.Root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Ensure(); err == nil {
		t.Fatal("expected error for conflicting databases")
	}
}

func TestFreshDatabaseName(t *testing.T) {
	d := Dirs{Root: t.TempDir()}
	if got := filepath.Base(d.DB()); got != "anyproxy.db" {
		t.Fatalf("DB = %q", got)
	}
}

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
