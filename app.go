// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/paths"
	"github.com/D0bby999/anyproxy/internal/pbprofile"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

// App is the Wails application host bound to the frontend.
type App struct {
	ctx          context.Context
	store        *store.Store
	svc          *service.Service
	startErr     error
	shutdownOnce sync.Once
	posMu        sync.Mutex
	isCompact    bool
	compactX     int
	compactY     int
	fullX        int
	fullY        int
}

// NewApp creates an uninitialized App instance for Wails binding.
func NewApp() *App {
	return &App{
		compactX: 180,
		compactY: 70,
		fullX:    180,
		fullY:    70,
	}
}

func parseWindowPos(raw string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(parts[0])
	y, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || x < 0 || y < 0 || x > 3800 || y > 2400 {
		return 0, 0, false
	}
	return x, y, true
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dirs, err := paths.Default()
	if err != nil {
		a.startErr = err
		return
	}
	if err := dirs.Ensure(); err != nil {
		a.startErr = err
		return
	}
	st, err := store.Open(dirs.DB())
	if err != nil {
		a.startErr = err
		return
	}
	a.store = st
	if raw, err := st.GetSetting("compact_pos"); err == nil {
		if cx, cy, ok := parseWindowPos(raw); ok {
			a.compactX, a.compactY = cx, cy
		}
	}
	if raw, err := st.GetSetting("full_pos"); err == nil {
		if fx, fy, ok := parseWindowPos(raw); ok {
			a.fullX, a.fullY = fx, fy
		}
	}
	runtime.WindowSetPosition(ctx, a.fullX, a.fullY)

	vault := secrets.NewVault(dirs.Vault(), secrets.Keychain{})

	a.svc = service.New(st, vault, nil)
	a.svc.RecoverSystemProxy()
	a.svc.SetEventEmitter(func(topic string, payload any) {
		if topic == "engine:status" || topic == "profile:changed" {
			a.refreshTray()
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, topic, payload)
		}
	})
	initTray(a)
	a.refreshTray()
}

func (a *App) saveCurrentWindowPosition() {
	if a.ctx == nil {
		return
	}
	x, y := runtime.WindowGetPosition(a.ctx)
	if x < 0 || y < 0 || x > 3800 || y > 2400 {
		return
	}
	a.posMu.Lock()
	defer a.posMu.Unlock()
	if a.isCompact {
		if x != a.compactX || y != a.compactY {
			a.compactX, a.compactY = x, y
			if a.store != nil {
				_ = a.store.SetSetting("compact_pos", fmt.Sprintf("%d,%d", x, y))
			}
		}
	} else {
		if x != a.fullX || y != a.fullY {
			a.fullX, a.fullY = x, y
			if a.store != nil {
				_ = a.store.SetSetting("full_pos", fmt.Sprintf("%d,%d", x, y))
			}
		}
	}
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.saveCurrentWindowPosition()
	if goruntime.GOOS == "darwin" {
		stopTray()
		if a.ctx != nil {
			runtime.Hide(a.ctx)
		}
		// Return before Wails stops the Cocoa runloop, then wake it in case Quit came from the menu bar while hidden.
		time.AfterFunc(25*time.Millisecond, wakeTrayRunLoop)
		return false
	}
	a.shutdown(ctx)
	return false
}

func (a *App) shutdown(_ context.Context) {
	a.shutdownOnce.Do(func() {
		stopTray()
		if a.svc != nil {
			_ = a.svc.Close()
		}
	})
}

func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	activateMainApp()
	runtime.Show(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

func (a *App) hideMainWindow() {
	if a.ctx == nil {
		return
	}
	a.saveCurrentWindowPosition()
	runtime.Hide(a.ctx)
}

func (a *App) currentTraySnapshot() traySnapshot {
	a.posMu.Lock()
	compact := a.isCompact
	a.posMu.Unlock()

	if a.svc == nil {
		return buildTraySnapshot(driver.EngineStatus{State: driver.StateStopped}, "", nil, nil, compact)
	}
	var profPtr *pbprofile.Profile
	if p, err := a.svc.ActiveProfile(); err == nil {
		profPtr = &p
	}
	list, _ := a.svc.ListProfiles()
	return buildTraySnapshot(a.svc.Status(), a.svc.InboundAddr(), profPtr, list.Names, compact)
}

func (a *App) refreshTray() {
	updateTraySnapshot(a.currentTraySnapshot())
}

func (a *App) toggleEngineFromTray() {
	if a.ready() != nil {
		return
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	st := a.svc.Status()
	var err error
	if st.State == driver.StateRunning || st.State == driver.StateStarting {
		_, err = a.svc.StopEngine()
	} else {
		_, err = a.svc.StartEngine(ctx)
	}
	a.refreshTray()
	if err != nil {
		a.showMainWindow()
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "AnyProxy",
			Message: err.Error(),
		})
	}
}

func (a *App) selectProxyFromTray(proxyID string) {
	if a.ready() != nil {
		return
	}
	_, err := a.svc.SetDefaultProxy(proxyID)
	a.refreshTray()
	if err != nil && a.ctx != nil {
		a.showMainWindow()
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "AnyProxy",
			Message: err.Error(),
		})
	}
}

func (a *App) selectProfileFromTray(name string) {
	if a.ready() != nil || name == "" {
		return
	}
	_, err := a.svc.SwitchProfile(name)
	a.refreshTray()
	if err != nil && a.ctx != nil {
		a.showMainWindow()
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "AnyProxy",
			Message: err.Error(),
		})
	}
}

func (a *App) toggleCompactFromTray() {
	a.posMu.Lock()
	nextCompact := !a.isCompact
	a.posMu.Unlock()
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "ui:compact", nextCompact)
	}
	a.applyCompactMode(nextCompact, false)
	a.showMainWindow()
}

func (a *App) openMenuDestination(what string) {
	if a.ctx == nil {
		return
	}
	a.showMainWindow()
	runtime.EventsEmit(a.ctx, "menu", what)
}

func (a *App) requestQuit() {
	if a.ctx == nil {
		return
	}
	runtime.Quit(a.ctx)
}

func (a *App) ready() error {
	if a.startErr != nil {
		return a.startErr
	}
	if a.svc == nil {
		return errors.New("service is not initialized")
	}
	return nil
}
