package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ThallesP/macron/tools/sandbox/internal/cli"
	"github.com/ThallesP/macron/tools/sandbox/internal/cloudflare"
	"github.com/ThallesP/macron/tools/sandbox/internal/githubactions"
	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sbx: cannot find its own path: %s\n", err)
		os.Exit(125)
	}
	stateHome, err := stateHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sbx: cannot find the home directory: %s\n", err)
		os.Exit(125)
	}

	ctx, release := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		release()
	}()

	stdin, err := os.Stdin.Stat()
	program := cli.CLI{
		App: &sandbox.App{
			Backend: &githubactions.Backend{
				Repo: envOr("SBX_REPO", "ThallesP/macron"),
				Ref:  envOr("SBX_REF", "main"),
				API:  githubactions.GH,
			},
			Transport: cloudflare.QuickTunnel{Executable: executable},
			Store:     sandbox.Store{Dir: filepath.Join(stateHome, "sbx")},
		},
		Proxies:         map[string]cli.Proxy{cloudflare.QuickTunnelName: cloudflare.ProxyQuickTunnel},
		Stdin:           os.Stdin,
		Stdout:          os.Stdout,
		Stderr:          os.Stderr,
		StdinIsTerminal: err == nil && stdin.Mode()&os.ModeCharDevice != 0,
	}
	os.Exit(program.Run(ctx, os.Args[1:]))
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func stateHome() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}
