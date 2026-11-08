//go:build darwin

// SPDX-License-Identifier: MIT

package main

import "github.com/wailsapp/wails/v2/pkg/menu"

func appMenu(_ *App) *menu.Menu {
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
	m.Append(menu.EditMenu())
	m.Append(menu.WindowMenu())
	return m
}

