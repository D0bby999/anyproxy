//go:build !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/D0bby999/anyproxy/internal/driver"

func initTray(_ *App) {}

func updateTrayStatus(_ driver.EngineStatus) {}

func stopTray() {}

func wakeTrayRunLoop() {}
