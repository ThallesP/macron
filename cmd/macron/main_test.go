package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
)

func TestUsageErrorsExitTwo(t *testing.T) {
	cases := [][]string{
		{"frobnicate"},
		{"launch"},
		{"launch", "--game", "d3d11-probe"},
		{"kill"},
		{"runtime"},
		{"waitforexitandrun"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer

		if code := run(context.Background(), args, &stdout, &stderr); code != exitUsage {
			t.Errorf("%v exited %d, want %d (stderr %q)", args, code, exitUsage, stderr.String())
		}
	}
}

func TestSteamVerbNeedsTheAppID(t *testing.T) {
	t.Setenv("SteamAppId", "")
	t.Setenv("STEAM_COMPAT_APP_ID", "")
	var stdout, stderr bytes.Buffer

	if code := run(context.Background(), []string{"waitforexitandrun", "game.exe"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("exited %d", code)
	}
}

func TestLaunchUnknownGameFails(t *testing.T) {
	t.Setenv("MACRON_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"launch", "--game", "no-such-game", "--", "x.exe"}, &stdout, &stderr)

	if code != exitFailed || !bytes.Contains(stderr.Bytes(), []byte("UNKNOWN_GAME")) {
		t.Fatalf("exited %d: %s", code, stderr.String())
	}
}

func TestMacronBackendOverridesTheProfile(t *testing.T) {
	profile := fixes.Default("steam-1")

	got, err := withBackendFromEnv(profile, "wined3d")
	if err != nil || got.Backend != fixes.BackendWineD3D {
		t.Errorf("got %v, %v", got.Backend, err)
	}
	if got, _ := withBackendFromEnv(profile, ""); got.Backend != fixes.BackendDXMT {
		t.Errorf("empty MACRON_BACKEND changed the backend to %s", got.Backend)
	}
	if _, err := withBackendFromEnv(profile, "metal"); failure.CodeOf(err) != failure.CodeInvalidInput {
		t.Errorf("unknown backend: %v", err)
	}
}
