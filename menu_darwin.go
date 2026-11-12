//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func appMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	appSub := m.AddSubmenu("AnyProxy")
	appSub.AddText("About AnyProxy", nil, func(_ *menu.CallbackData) {
		showAboutPanel()
	})
	appSub.AddSeparator()
	appSub.AddText("Check for Updates…", nil, func(_ *menu.CallbackData) {
		if app != nil {
			app.openMenuDestination("check-update")
		}
	})
	appSub.AddText("Settings…", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		if app != nil {
			app.openMenuDestination("settings")
		}
	})
	appSub.AddSeparator()
	appSub.AddText("Hide AnyProxy", keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) {
		if app != nil && app.ctx != nil {
			runtime.Hide(app.ctx)
		}
	})
	appSub.AddText("Quit AnyProxy", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		if app != nil {
			app.requestQuit()
		}
	})

	fileMenu := m.AddSubmenu("File")
	fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
		if app != nil {
			app.hideMainWindow()
		}
	})
	m.Append(menu.EditMenu())
	m.Append(menu.WindowMenu())
	return m
}
