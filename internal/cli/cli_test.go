// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyproxy/internal/driver"
	"github.com/D0bby999/anyproxy/internal/secrets"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/store"
)

func TestCLIListProfilesAndStdinProfile(t *testing.T) {
	if ShouldRunCLI([]string{"-psn_0_12345"}) {
		t.Fatalf("Finder -psn argument must not trigger CLI mode")
	}
	if !ShouldRunCLI([]string{"--list-profiles"}) {
		t.Fatalf("--list-profiles must trigger CLI mode")
	}

	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "cli.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	vault := secrets.NewVault(filepath.Join(dir, "secrets.enc"), secrets.NewMemory())
	drv := driver.NewMockDriver("cli-test", nil, nil, nil)
	svc := service.New(st, vault, drv)
	t.Cleanup(func() { _ = svc.Close() })

	_, _ = svc.CreateProfile("Pentest")

	var out bytes.Buffer
	if err := Run(context.Background(), svc, []string{"--list-profiles"}, nil, &out); err != nil {
		t.Fatalf("Run --list-profiles: %v", err)
	}
	if !strings.Contains(out.String(), "* Pentest") || !strings.Contains(out.String(), "  Default") {
		t.Fatalf("unexpected --list-profiles output:\n%s", out.String())
	}

	// Test --profile - (stdin) with immediate context cancellation
	stdinProfile := `{"name":"CLI-Stdin","proxyConfigs":[{"id":"p1","type":"SOCKS5","host":"127.0.0.1","port":1080}],"proxyRules":[]}`
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	out.Reset()
	err = Run(ctx, svc, []string{"--profile", "-", "--verbose", "1"}, strings.NewReader(stdinProfile), &out)
	if err != nil {
		t.Fatalf("Run --profile -: %v", err)
	}
	if !strings.Contains(out.String(), `AnyProxy active with profile "CLI-Stdin"`) {
		t.Fatalf("expected active banner in CLI output, got:\n%s", out.String())
	}
}

