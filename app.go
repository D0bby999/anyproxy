// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyproxy/internal/paths"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

// App is the Wails application host bound to the frontend.
type App struct {
	ctx       context.Context
	store     *store.Store
	svc       *service.Service
	startErr  error
	posMu     sync.Mutex
	isCompact bool
	compactX  int
	compactY  int
	fullX     int
	fullY     int
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
	a.svc.SetEventEmitter(func(topic string, payload any) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, topic, payload)
		}
	})

	go a.trackWindowPosition(ctx)
}

func (a *App) trackWindowPosition(ctx context.Context) {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.saveCurrentWindowPosition()
		}
	}
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
	if a.svc != nil {
		_ = a.svc.Close()
	}
	return false
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
