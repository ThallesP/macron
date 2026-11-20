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
		{"uninstall", "now"},
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

func TestAppleSoftwareRendererAddsTheMacDriverSetting(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Registry = []fixes.RegistryValue{{Key: `HKEY_CURRENT_USER\Software\Game`, Name: "Mode", Type: "sz", Value: "a"}}

	got := withAppleSoftwareRenderer(profile)

	if len(profile.Registry) != 1 {
		t.Fatalf("the profile's own registry list changed: %v", profile.Registry)
	}
	want := fixes.RegistryValue{Key: `HKEY_CURRENT_USER\Software\Wine\Mac Driver`, Name: "AllowSoftwareRendering", Type: "sz", Value: "y"}
	if len(got.Registry) != 2 || got.Registry[1] != want {
		t.Errorf("registry = %v", got.Registry)
	}
}

func TestDLSSSwitchTurnsOnDXMTsNVIDIAExtensionWithoutTouchingTheProfile(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Env = map[string]string{"DXMT_LOG_LEVEL": "info"}

	got := withDLSS(profile)

	if len(profile.Env) != 1 {
		t.Fatalf("the profile's own env changed: %v", profile.Env)
	}
	if got.Env["DXMT_ENABLE_NVEXT"] != "1" || got.Env["DXMT_LOG_LEVEL"] != "info" {
		t.Errorf("env = %v, want DXMT_ENABLE_NVEXT=1 and the profile's own", got.Env)
	}
	if got.Env["WINE_D3D_CONFIG"] != "VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684" {
		t.Errorf("WINE_D3D_CONFIG = %q, want the card DXMT answers as, so that wined3d's driver version is that card's", got.Env["WINE_D3D_CONFIG"])
	}
}

func TestDLSSSwitchKeepsAProfilesOwnWined3dOptions(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Env = map[string]string{"WINE_D3D_CONFIG": "csmt=0"}

	got := withDLSS(profile)

	if got.Env["WINE_D3D_CONFIG"] != "csmt=0,VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684" {
		t.Errorf("WINE_D3D_CONFIG = %q", got.Env["WINE_D3D_CONFIG"])
	}
}

func TestFrameCounterLogsTheFrameRateWithoutTouchingTheProfile(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Env = map[string]string{"DXMT_LOG_LEVEL": "info"}

	got := withFrameCounter(profile)

	if len(profile.Env) != 1 {
		t.Fatalf("the profile's own env changed: %v", profile.Env)
	}
	for name, want := range map[string]string{"WINEDEBUG": "-all,+fps", "DXVK_HUD": "fps", "MTL_HUD_ENABLED": "1", "MTL_HUD_LOG_ENABLED": "1", "DXMT_LOG_LEVEL": "info", "DXMT_CONFIG": "dxmt.logLatency=True"} {
		if got.Env[name] != want {
			t.Errorf("%s = %q, want %q", name, got.Env[name], want)
		}
	}
}

func TestTheFrameCounterKeepsTheProfilesDXMTOptions(t *testing.T) {
	profile := fixes.Default("steam-1")
	profile.Env = map[string]string{"DXMT_CONFIG": "dxgi.maxFrameLatency=1"}

	got := withFrameCounter(profile)

	if want := "dxgi.maxFrameLatency=1;dxmt.logLatency=True"; got.Env["DXMT_CONFIG"] != want {
		t.Errorf("DXMT_CONFIG = %q, want %q", got.Env["DXMT_CONFIG"], want)
	}
}
