//go:build !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

func initTray(_ *App) {}

func updateTraySnapshot(_ traySnapshot) {}

func activateMainApp() {}

func showAboutPanel() {}

func stopTray() {}

func wakeTrayRunLoop() {}
