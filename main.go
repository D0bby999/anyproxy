// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/D0bby999/anyproxy/internal/cli"
	"github.com/D0bby999/anyproxy/internal/paths"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	ownerOnlyFiles()

	if cli.ShouldRunCLI(os.Args[1:]) {
		os.Exit(runCLI(os.Args[1:]))
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "AnyProxy",
		Width:     1080,
		Height:    680,
		MinWidth:  320,
		MinHeight: 420,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 236, G: 235, B: 231, A: 1},
		Menu:             appMenu(app),
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind: []any{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "AnyProxy",
				Message: "Per-application proxy manager for macOS & Windows\nAnyTools Ecosystem",
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.Light,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func runCLI(args []string) int {
	dirs, err := paths.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "anyproxy: paths: %v\n", err)
		return 1
	}
	if err := dirs.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "anyproxy: ensure dirs: %v\n", err)
		return 1
	}
	st, err := store.Open(dirs.DB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "anyproxy: open store: %v\n", err)
		return 1
	}
	vault := secrets.NewVault(dirs.Vault(), secrets.Keychain{})
	svc := service.New(st, vault, nil)
	defer func() { _ = svc.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, svc, args, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "anyproxy: %v\n", err)
		return 1
	}
	return 0
}
