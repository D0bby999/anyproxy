//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo darwin CFLAGS: -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>

void anyproxyTrayInitJSON(const char *json);
void anyproxyTrayUpdateJSON(const char *json);
void anyproxyTrayActivateApp(void);
void anyproxyShowAboutPanel(void);
void anyproxyTrayDestroy(void);
void anyproxyTrayWakeRunLoop(void);
*/
import "C"

import (
	"encoding/json"
	"sync"
	"unsafe"
)

const (
	trayActionToggle  = 1
	trayActionOpen    = 2
	trayActionCompact = 3
	trayActionQuit    = 4
)

var (
	trayMu  sync.RWMutex
	trayApp *App
)

func initTray(app *App) {
	trayMu.Lock()
	trayApp = app
	trayMu.Unlock()

	raw, err := json.Marshal(app.currentTraySnapshot())
	if err != nil {
		return
	}
	cJSON := C.CString(string(raw))
	defer C.free(unsafe.Pointer(cJSON))
	C.anyproxyTrayInitJSON(cJSON)
}

func updateTraySnapshot(snap traySnapshot) {
	trayMu.RLock()
	activeApp := trayApp
	trayMu.RUnlock()
	if activeApp == nil {
		return
	}

	raw, err := json.Marshal(snap)
	if err != nil {
		return
	}
	cJSON := C.CString(string(raw))
	defer C.free(unsafe.Pointer(cJSON))
	C.anyproxyTrayUpdateJSON(cJSON)
}

func activateMainApp() {
	C.anyproxyTrayActivateApp()
}

func showAboutPanel() {
	C.anyproxyShowAboutPanel()
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
	case trayActionCompact:
		go app.toggleCompactFromTray()
	case trayActionQuit:
		go app.requestQuit()
	}
}

//export anyproxyTraySelectProxy
func anyproxyTraySelectProxy(cID *C.char) {
	trayMu.RLock()
	app := trayApp
	trayMu.RUnlock()
	if app == nil {
		return
	}
	var id string
	if cID != nil {
		id = C.GoString(cID)
	}
	go app.selectProxyFromTray(id)
}

//export anyproxyTraySelectProfile
func anyproxyTraySelectProfile(cName *C.char) {
	trayMu.RLock()
	app := trayApp
	trayMu.RUnlock()
	if app == nil {
		return
	}
	var name string
	if cName != nil {
		name = C.GoString(cName)
	}
	go app.selectProfileFromTray(name)
}
