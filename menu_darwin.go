//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

func appMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
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
