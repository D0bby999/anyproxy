// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package update

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	bundleName = "AnyProxy.app"
	bundleID   = "dev.anyproxy.app"
	stagingDir = ".AnyProxy-update"
	oldBundle  = ".AnyProxy-old.app"
)

// ErrTranslocated is returned when macOS runs AnyProxy from an AppTranslocation read-only path.
var ErrTranslocated = errors.New("macOS runs AnyProxy from a read-only copy; move AnyProxy.app to Applications, open it from there and update again")

var executable = os.Executable

func installedApp() (string, error) {
	exe, err := executable()
	if err != nil {
		return "", err
	}
	if strings.Contains(exe, "/AppTranslocation/") {
		return "", ErrTranslocated
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe))) // .app/Contents/MacOS/AnyProxy
	if !strings.HasSuffix(app, ".app") {
		return "", fmt.Errorf("AnyProxy is not running from an app bundle (%s)", exe)
	}
	if strings.Contains(app, "/AppTranslocation/") {
		return "", ErrTranslocated
	}
	return app, nil
}

// Apply swaps the installed AnyProxy.app for the one inside the verified pkg (.dmg or .zip),
// then starts a detached shell that waits for pid to exit, deletes the old copy, and opens the new app.
func Apply(pkg string, pid int) error {
	app, err := installedApp()
	if err != nil {
		return err
	}
	if err := swap(app, pkg); err != nil {
		return err
	}
	_ = os.Remove(pkg)
	old := filepath.Join(filepath.Dir(app), oldBundle)
	relaunch := exec.Command("/bin/sh", "-c",
		`while kill -0 "$0" 2>/dev/null; do sleep 0.2; done; rm -rf "$2"; exec /usr/bin/open "$1"`,
		strconv.Itoa(pid), app, old)
	relaunch.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return relaunch.Start()
}

func swap(app, pkg string) error {
	parent := filepath.Dir(app)
	staging := filepath.Join(parent, stagingDir)
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)

	if err := os.MkdirAll(staging, 0o700); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("cannot write in %s; move AnyProxy.app to a folder you own, such as Applications", parent)
		}
		return err
	}

	if err := unpackBundle(pkg, staging); err != nil {
		return err
	}
	next := filepath.Join(staging, bundleName)
	if err := checkBundle(next); err != nil {
		return err
	}

	old := filepath.Join(parent, oldBundle)
	_ = os.RemoveAll(old)
	if err := os.Rename(app, old); err != nil {
		return fmt.Errorf("move the old app aside: %w", err)
	}
	if err := os.Rename(next, app); err != nil {
		_ = os.Rename(old, app)
		return fmt.Errorf("put the new app in place: %w", err)
	}
	return nil
}

func unpackBundle(pkg, staging string) error {
	if strings.EqualFold(filepath.Ext(pkg), ".dmg") {
		mnt := filepath.Join(staging, "mnt")
		if err := os.MkdirAll(mnt, 0o700); err != nil {
			return err
		}
		if out, err := exec.Command("/usr/bin/hdiutil", "attach", "-nobrowse", "-readonly", "-mountpoint", mnt, pkg).CombinedOutput(); err != nil {
			return fmt.Errorf("mount update image: %s", strings.TrimSpace(string(out)))
		}
		defer func() {
			if err := exec.Command("/usr/bin/hdiutil", "detach", mnt).Run(); err != nil {
				_ = exec.Command("/usr/bin/hdiutil", "detach", mnt, "-force").Run()
			}
		}()
		src := filepath.Join(mnt, bundleName)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("update image missing %s", bundleName)
		}
		if out, err := exec.Command("/usr/bin/ditto", src, filepath.Join(staging, bundleName)).CombinedOutput(); err != nil {
			return fmt.Errorf("copy update app: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	if out, err := exec.Command("/usr/bin/ditto", "-x", "-k", pkg, staging).CombinedOutput(); err != nil {
		return fmt.Errorf("unpack update: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func checkBundle(app string) error {
	if out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict", app).CombinedOutput(); err != nil {
		return fmt.Errorf("the update's code signature is broken: %s", strings.TrimSpace(string(out)))
	}
	out, _ := exec.Command("/usr/bin/codesign", "-dv", app).CombinedOutput()
	if !strings.Contains(string(out), "Identifier="+bundleID+"\n") {
		return errors.New("the update is not AnyProxy")
	}
	return nil
}

// CleanStale removes leftover staging or backup bundles from an interrupted update.
func CleanStale() {
	if app, err := installedApp(); err == nil {
		_ = os.RemoveAll(filepath.Join(filepath.Dir(app), stagingDir))
		_ = os.RemoveAll(filepath.Join(filepath.Dir(app), oldBundle))
	}
}
