// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwapRealBundle(t *testing.T) {
	built, _ := filepath.Abs("../../build/bin/AnyProxy.app")
	if _, err := os.Stat(built); err != nil {
		t.Skip("needs make build")
	}
	root, _ := filepath.EvalSymlinks(t.TempDir())
	app := filepath.Join(root, "Applications", bundleName)
	if err := os.MkdirAll(filepath.Dir(app), 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/ditto", built, app).CombinedOutput(); err != nil {
		t.Fatalf("copy: %s", out)
	}
	marker := filepath.Join(app, "Contents", "old-marker")
	_ = os.WriteFile(marker, nil, 0o600)

	pkg := filepath.Join(root, "AnyProxy-darwin-arm64.zip")
	if out, err := exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", built, pkg).CombinedOutput(); err != nil {
		t.Fatalf("zip: %s", out)
	}

	old := executable
	executable = func() (string, error) { return filepath.Join(app, "Contents", "MacOS", "AnyProxy"), nil }
	t.Cleanup(func() { executable = old })

	got, err := installedApp()
	if err != nil || got != app {
		t.Fatalf("installedApp = %q, %v", got, err)
	}
	if err := swap(app, pkg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the old app is still in place")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(app), oldBundle, "Contents", "old-marker")); err != nil {
		t.Fatal("the old app was not kept aside")
	}
	if err := checkBundle(app); err != nil {
		t.Fatalf("new app: %v", err)
	}
	CleanStale()
	if _, err := os.Stat(filepath.Join(filepath.Dir(app), oldBundle)); err == nil {
		t.Fatal("CleanStale left the old app")
	}

	// A tampered bundle is refused and the installed app remains intact.
	bad := filepath.Join(root, "bad")
	_ = exec.Command("/usr/bin/ditto", built, filepath.Join(bad, bundleName)).Run()
	_ = os.WriteFile(filepath.Join(bad, bundleName, "Contents", "MacOS", "AnyProxy"), []byte("evil"), 0o700)
	badPkg := filepath.Join(root, "bad.zip")
	_ = exec.Command("/usr/bin/ditto", "-c", "-k", "--keepParent", filepath.Join(bad, bundleName), badPkg).Run()
	if err := swap(app, badPkg); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("tampered bundle: %v", err)
	}
	if err := checkBundle(app); err != nil {
		t.Fatalf("installed app damaged by a refused update: %v", err)
	}
}

func TestTranslocatedAppRefused(t *testing.T) {
	old := executable
	executable = func() (string, error) {
		return "/private/var/folders/x/AppTranslocation/ABC/d/AnyProxy.app/Contents/MacOS/AnyProxy", nil
	}
	t.Cleanup(func() { executable = old })
	if _, err := installedApp(); err != ErrTranslocated {
		t.Fatalf("err = %v", err)
	}
}
