//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo darwin CFLAGS: -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>

void anyproxyTrayInit(int active, const char *statusLabel, const char *toggleLabel, const char *tooltip);
void anyproxyTrayUpdate(int active, const char *statusLabel, const char *toggleLabel, const char *tooltip);
void anyproxyTrayDestroy(void);
void anyproxyTrayWakeRunLoop(void);
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/D0bby999/anyproxy/internal/driver"
)

const (
	trayActionToggle = 1
	trayActionOpen   = 2
	trayActionQuit   = 3
)

var (
	trayMu  sync.RWMutex
	trayApp *App
)

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func initTray(app *App) {
	trayMu.Lock()
	trayApp = app
	trayMu.Unlock()

	ts := formatTrayState(driver.EngineStatus{State: driver.StateStopped})
	cStatus := C.CString(ts.StatusLabel)
	cToggle := C.CString(ts.ToggleLabel)
	cTip := C.CString(ts.Tooltip)
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cToggle))
	defer C.free(unsafe.Pointer(cTip))

	C.anyproxyTrayInit(boolToCInt(ts.Active), cStatus, cToggle, cTip)
}

func updateTrayStatus(st driver.EngineStatus) {
	trayMu.RLock()
	activeApp := trayApp
	trayMu.RUnlock()
	if activeApp == nil {
		return
	}

	ts := formatTrayState(st)
	cStatus := C.CString(ts.StatusLabel)
	cToggle := C.CString(ts.ToggleLabel)
	cTip := C.CString(ts.Tooltip)
	defer C.free(unsafe.Pointer(cStatus))
	defer C.free(unsafe.Pointer(cToggle))
	defer C.free(unsafe.Pointer(cTip))

	C.anyproxyTrayUpdate(boolToCInt(ts.Active), cStatus, cToggle, cTip)
}

func stopTray() {
	trayMu.Lock()
	wasInit := trayApp != nil
	trayApp = nil
	trayMu.Unlock()
	if wasInit {
		C.anyproxyTrayDestroy()
	}
}

func wakeTrayRunLoop() {
	C.anyproxyTrayWakeRunLoop()
}

//export anyproxyTrayMenuAction
func anyproxyTrayMenuAction(action C.int) {
	trayMu.RLock()
	app := trayApp
	trayMu.RUnlock()
	if app == nil {
		return
	}

	switch int(action) {
	case trayActionToggle:
		go app.toggleEngineFromTray()
	case trayActionOpen:
		go app.showMainWindow()
	case trayActionQuit:
		go app.requestQuit()
	}
}
