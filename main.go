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

	"github.com/D0bby999/anyproxyxa/internal/cli"
	"github.com/D0bby999/anyproxyxa/internal/paths"
	"github.com/D0bby999/anyproxyxa/internal/secrets"
	"github.com/D0bby999/anyproxyxa/internal/service"
	"github.com/D0bby999/anyproxyxa/internal/store"
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
		Title:     "AnyProxyXa",
		Width:     1280,
		Height:    800,
		MinWidth:  980,
		MinHeight: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 249, G: 251, B: 252, A: 1},
		Menu:             appMenu(app),
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		Bind: []any{
			app,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "AnyProxyXa",
				Message: "Transparent Per-Application Proxy Router for macOS & Windows\nAnyTools Ecosystem — Ported from InterceptSuite/ProxyBridge (MIT)",
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
		fmt.Fprintf(os.Stderr, "anyproxyxa: paths: %v\n", err)
		return 1
	}
	if err := dirs.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "anyproxyxa: ensure dirs: %v\n", err)
		return 1
	}
	st, err := store.Open(dirs.DB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "anyproxyxa: open store: %v\n", err)
		return 1
	}
	vault := secrets.NewVault(dirs.Vault(), secrets.Keychain{})
	svc := service.New(st, vault, nil)
	defer func() { _ = svc.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, svc, args, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "anyproxyxa: %v\n", err)
		return 1
	}
	return 0
}

