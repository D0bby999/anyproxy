// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/D0bby999/anyproxy/internal/pbprofile"
)

func TestStoreSaveAndSwitchProfiles(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "anyproxyxa.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	names, active, err := st.ListProfileNames()
	if err != nil || len(names) != 1 || active != "Default" {
		t.Fatalf("initial profiles = %v, active = %q, err = %v", names, active, err)
	}

	f, err := os.Open("/tmp/ProxyBridge/MacOS/tests/pbtest.pbprofile")
	if err != nil {
		t.Skipf("pbtest.pbprofile not found: %v", err)
	}
	defer f.Close()
	imported, err := pbprofile.ParseProfile(f)
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	saved, err := st.SaveProfile(imported, true)
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	got, err := st.ActiveProfile()
	if err != nil {
		t.Fatalf("ActiveProfile: %v", err)
	}
	if got.Name != "PBTest" || len(got.Proxies) != 5 || len(got.Rules) != 10 {
		t.Fatalf("ActiveProfile = %+v", got)
	}
	if got.Rules[0].ProxyConfigID != saved.Proxies[0].ID {
		t.Fatalf("rule 0 proxy id = %q, want %q", got.Rules[0].ProxyConfigID, saved.Proxies[0].ID)
	}
	if err := st.RenameProfile("PBTest", "Work"); err != nil {
		t.Fatalf("RenameProfile: %v", err)
	}
	if err := st.SetActiveProfile("Default"); err != nil {
		t.Fatalf("SetActiveProfile: %v", err)
	}
	if err := st.DeleteProfile("Work"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
}

func TestStoreSettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "settings.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := st.GetSetting("compact_pos"); err != ErrNotFound {
		t.Fatalf("GetSetting missing = %v, want ErrNotFound", err)
	}
	if err := st.SetSetting("compact_pos", "920,60"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, err := st.GetSetting("compact_pos")
	if err != nil || val != "920,60" {
		t.Fatalf("GetSetting = %q, err = %v", val, err)
	}
}


