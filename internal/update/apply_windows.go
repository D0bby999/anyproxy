// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Apply runs the verified NSIS installer pkg silently into the directory where this
// app is installed once pid exits, removes the installer, and relaunches the app.
func Apply(pkg string, pid int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	script := `Wait-Process -Id $env:AP_PID -ErrorAction SilentlyContinue
Start-Process -Wait -FilePath $env:AP_INSTALLER -ArgumentList ('/S /D=' + $env:AP_DIR)
Remove-Item -LiteralPath $env:AP_INSTALLER -ErrorAction SilentlyContinue
Start-Process -FilePath $env:AP_EXE`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	cmd.Env = append(os.Environ(),
		"AP_PID="+strconv.Itoa(pid),
		"AP_INSTALLER="+pkg,
		"AP_DIR="+filepath.Dir(exe),
		"AP_EXE="+exe,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	return cmd.Start()
}

// CleanStale is a no-op on Windows because the installer updates files in place.
func CleanStale() {}
