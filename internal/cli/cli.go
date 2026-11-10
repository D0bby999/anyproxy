// SPDX-License-Identifier: MIT

// Package cli implements the headless command-line runner for AnyProxy,
// compatible with upstream ProxyBridge CLI flags (--profile, --list-profiles,
// --verbose 0..3, --test-proxy, --version, --help).
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/D0bby999/anyproxy/internal/netproxy"
	"github.com/D0bby999/anyproxy/internal/service"
	"github.com/D0bby999/anyproxy/internal/update"
)

const Version = "1.0.0"

// Options holds parsed CLI flags.
type Options struct {
	ProfileSpec  string
	ListProfiles bool
	Verbose      int // 0=silent, 1=activity, 2=connections, 3=both
	TestProxyID  string
	TestTarget   string
	ShowVersion  bool
	ShowHelp     bool
	CheckUpdate  bool
}

// ShouldRunCLI reports whether the process arguments request headless CLI mode
// rather than launching the Wails desktop GUI.
func ShouldRunCLI(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-psn_") {
			continue
		}
		switch a {
		case "-h", "--help", "--version", "-v", "--list-profiles", "--profile", "-p", "--verbose", "--test-proxy", "--update":
			return true
		}
		if strings.HasPrefix(a, "--profile=") || strings.HasPrefix(a, "--verbose=") || strings.HasPrefix(a, "--test-proxy=") {
			return true
		}
	}
	return false
}

// ParseArgs parses command-line arguments into Options.
func ParseArgs(args []string) (Options, error) {
	opts := Options{
		Verbose:    1,
		TestTarget: "www.google.com:80",
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-psn_") {
			continue
		}
		switch {
		case arg == "-h" || arg == "--help":
			opts.ShowHelp = true
		case arg == "--version":
			opts.ShowVersion = true
		case arg == "--list-profiles":
			opts.ListProfiles = true
		case arg == "-p" || arg == "--profile":
			if i+1 >= len(args) {
				return opts, errors.New("--profile requires a profile name, file path, or '-' for stdin")
			}
			i++
			opts.ProfileSpec = args[i]
		case strings.HasPrefix(arg, "--profile="):
			opts.ProfileSpec = strings.TrimPrefix(arg, "--profile=")
		case arg == "-v" || arg == "--verbose":
			if i+1 >= len(args) {
				return opts, errors.New("--verbose requires a level between 0 and 3")
			}
			i++
			v, err := strconv.Atoi(args[i])
			if err != nil || v < 0 || v > 3 {
				return opts, fmt.Errorf("invalid --verbose level %q (want 0..3)", args[i])
			}
			opts.Verbose = v
		case strings.HasPrefix(arg, "--verbose="):
			val := strings.TrimPrefix(arg, "--verbose=")
			v, err := strconv.Atoi(val)
			if err != nil || v < 0 || v > 3 {
				return opts, fmt.Errorf("invalid --verbose level %q (want 0..3)", val)
			}
			opts.Verbose = v
		case arg == "--test-proxy":
			if i+1 >= len(args) {
				return opts, errors.New("--test-proxy requires a proxy ID or name")
			}
			i++
			opts.TestProxyID = args[i]
		case strings.HasPrefix(arg, "--test-proxy="):
			opts.TestProxyID = strings.TrimPrefix(arg, "--test-proxy=")
		case arg == "--update":
			opts.CheckUpdate = true
		default:
			return opts, fmt.Errorf("unknown flag %q (run with --help for usage)", arg)
		}
	}
	return opts, nil
}

// Run executes the CLI command against svc and writes output to out.
func Run(ctx context.Context, svc *service.Service, args []string, in io.Reader, out io.Writer) error {
	opts, err := ParseArgs(args)
	if err != nil {
		return err
	}

	if opts.ShowHelp {
		printUsage(out)
		return nil
	}
	if opts.ShowVersion {
		_, _ = fmt.Fprintf(out, "AnyProxy v%s\n", Version)
		return nil
	}
	if opts.CheckUpdate {
		_, _ = fmt.Fprintln(out, "Checking for AnyProxy updates...")
		info, err := update.CheckForUpdates(ctx, "")
		if err != nil {
			_, _ = fmt.Fprintf(out, "Update check failed: %v\n", err)
			return err
		}
		_, _ = fmt.Fprintf(out, "Current version: v%s\n", info.CurrentVersion)
		_, _ = fmt.Fprintf(out, "Latest version:  v%s\n", info.LatestVersion)
		if info.IsUpdateAvailable {
			_, _ = fmt.Fprintf(out, "\nNew version available! (Released: %s)\n", info.ReleaseDate)
			if info.ReleaseNotes != "" {
				_, _ = fmt.Fprintf(out, "Notes: %s\n", info.ReleaseNotes)
			}
			if info.DownloadURL != "" {
				_, _ = fmt.Fprintf(out, "Download URL: %s\n", info.DownloadURL)
			}
		} else {
			_, _ = fmt.Fprintln(out, "You are running the latest version.")
		}
		return nil
	}
	if opts.ListProfiles {
		summary, err := svc.ListProfiles()
		if err != nil {
			return err
		}
		for _, name := range summary.Names {
			prefix := "  "
			if name == summary.Active {
				prefix = "* "
			}
			_, _ = fmt.Fprintf(out, "%s%s\n", prefix, name)
		}
		return nil
	}

	if opts.ProfileSpec != "" {
		if err := loadProfileSpec(svc, opts.ProfileSpec, in); err != nil {
			return err
		}
	}

	if opts.TestProxyID != "" {
		prof, err := svc.ActiveProfile()
		if err != nil {
			return err
		}
		proxyID := opts.TestProxyID
		for _, px := range prof.Proxies {
			if px.ID == opts.TestProxyID || strings.EqualFold(px.Name, opts.TestProxyID) {
				proxyID = px.ID
				break
			}
		}
		_, err = svc.TestProxy(ctx, proxyID, "www.google.com", 80, func(step netproxy.CheckStep) {
			_, _ = fmt.Fprintf(out, "[Step %d/5] %-24s %-5s %s\n", step.Step, step.Name, strings.ToUpper(string(step.Status)), step.Message)
		})
		return err
	}

	prof, err := svc.ActiveProfile()
	if err != nil {
		return err
	}
	if _, err := svc.StartEngine(ctx); err != nil {
		return err
	}
	defer func() {
		_, _ = svc.StopEngine()
	}()

	if opts.Verbose > 0 {
		_, _ = fmt.Fprintf(out, "AnyProxy active with profile %q (%d proxies, %d rules). Press Ctrl+C to stop.\n",
			prof.Name, len(prof.Proxies), len(prof.Rules))
	}

	var lastConnID, lastActID int64
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if opts.Verbose > 0 {
				_, _ = fmt.Fprintln(out, "Stopping engine and clearing rules...")
			}
			return nil
		case <-ticker.C:
			if (opts.Verbose & 1) != 0 {
				acts, _ := svc.GetActivityLogs(100)
				for i := len(acts) - 1; i >= 0; i-- {
					if acts[i].ID > lastActID {
						lastActID = acts[i].ID
						_, _ = fmt.Fprintf(out, "[%s] [%s] %s\n", acts[i].Timestamp, acts[i].Level, acts[i].Message)
					}
				}
			}
			if (opts.Verbose & 2) != 0 {
				conns, _ := svc.GetConnectionLogs(100)
				for i := len(conns) - 1; i >= 0; i-- {
					if conns[i].ID > lastConnID {
						lastConnID = conns[i].ID
						target := conns[i].Host
						if target == "" {
							target = conns[i].DestIP
						}
						_, _ = fmt.Fprintf(out, "[%s] %-3s %-18s -> %s:%d [%s]\n",
							conns[i].Timestamp, conns[i].Protocol, conns[i].ProcessName, target, conns[i].Port, conns[i].ProxyName)
					}
				}
			}
		}
	}
}

func loadProfileSpec(svc *service.Service, spec string, in io.Reader) error {
	if spec == "-" {
		raw, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("read profile from stdin: %w", err)
		}
		_, err = svc.ImportProfileJSON(raw, true)
		return err
	}

	// Check if spec matches a saved profile name first
	summary, err := svc.ListProfiles()
	if err == nil {
		for _, name := range summary.Names {
			if strings.EqualFold(name, spec) {
				_, err := svc.SwitchProfile(name)
				return err
			}
		}
	}

	// Otherwise load as a .pbprofile file path
	raw, err := os.ReadFile(spec)
	if err != nil {
		return fmt.Errorf("profile %q is neither a saved profile nor a readable file: %w", spec, err)
	}
	_, err = svc.ImportProfileJSON(raw, true)
	return err
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, `Usage: anyproxy [options]

When launched without flags, AnyProxy opens the desktop GUI.
When launched with any flag below, AnyProxy runs in headless CLI mode:

  -p, --profile <name|path|->   Load saved profile name, .pbprofile file, or '-' (stdin)
      --list-profiles           List saved profiles (* marks active) and exit
  -v, --verbose <0-3>           Log output: 0=silent, 1=activity, 2=connections, 3=both (default: 1)
      --test-proxy <id|name>    Run 5-step Proxy Checker on the specified proxy and exit
      --update                  Check for software updates
      --version                 Print version and exit
  -h, --help                    Show this help message`)
}
