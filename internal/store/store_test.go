// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"strings"
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

func TestStorePersistsAcrossReopenAndStripsPasswords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	p := pbprofile.DefaultProfile("Work")
	p.Proxies = []pbprofile.ProxyConfig{{
		Name: "P1", Type: pbprofile.ProxySOCKS5, Host: "127.0.0.1", Port: 1080,
		Username: "u", Password: "never-on-disk-secret",
	}}
	p.Rules = []pbprofile.ProxyRule{{
		Name: "R1", ProcessNames: "curl", TargetHosts: "*", TargetPorts: "*",
		Protocol: pbprofile.ProtoTCP, Action: pbprofile.ActionProxy, Enabled: true,
	}}
	saved, err := st.SaveProfile(p, true)
	if err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	if err := st.SetSetting("k", "v"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	_ = st.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "never-on-disk-secret") {
		t.Fatal("password leaked to disk state file")
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer st2.Close()

	act, err := st2.ActiveProfile()
	if err != nil || act.Name != "Work" || len(act.Proxies) != 1 || act.Proxies[0].ID != saved.Proxies[0].ID {
		t.Fatalf("reopened ActiveProfile = %+v, err = %v", act, err)
	}
	if v, err := st2.GetSetting("k"); err != nil || v != "v" {
		t.Fatalf("reopened GetSetting = %q, err = %v", v, err)
	}
}
