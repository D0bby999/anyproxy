// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/D0bby999/anyproxy/internal/paths"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

// App is the Wails application host bound to the frontend.
type App struct {
	ctx      context.Context
	svc      *service.Service
	startErr error
}

// NewApp creates an uninitialized App instance for Wails binding.
func NewApp() *App {
	return &App{}
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
	vault := secrets.NewVault(dirs.Vault(), secrets.Keychain{})

	a.svc = service.New(st, vault, nil)
	a.svc.SetEventEmitter(func(topic string, payload any) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, topic, payload)
		}
	})
	_, _ = a.svc.StartEngine(ctx)
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
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

