package launch

import (
	"bytes"
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const steamHelper = `C:\windows\system32\steam.exe`

var steamBridgeFiles = map[string]string{
	"Program Files (x86)/Steam/steamclient.dll":   "lib/wine/i386-windows/lsteamclient.dll",
	"Program Files (x86)/Steam/steamclient64.dll": "lib/wine/x86_64-windows/lsteamclient.dll",
	"Program Files (x86)/Steam/steam.exe":         "lib/wine/x86_64-windows/steam.exe",
	"windows/system32/steam.exe":                  "lib/wine/x86_64-windows/steam.exe",
}

const steamBridgeUnixLib = "lib/wine/x86_64-unix/lsteamclient.so"

var steamBridgeRegistry = []fixes.RegistryValue{
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam`, Name: "SteamPath", Type: "sz", Value: `C:\Program Files (x86)\Steam`},
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam`, Name: "SteamExe", Type: "sz", Value: `C:\Program Files (x86)\Steam\steam.exe`},
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam\ActiveProcess`, Name: "SteamClientDll", Type: "sz", Value: `C:\Program Files (x86)\Steam\steamclient.dll`},
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam\ActiveProcess`, Name: "SteamClientDll64", Type: "sz", Value: `C:\Program Files (x86)\Steam\steamclient64.dll`},
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam\ActiveProcess`, Name: "SteamPath", Type: "sz", Value: `C:\Program Files (x86)\Steam`},
	{Key: `HKEY_CURRENT_USER\Software\Valve\Steam\ActiveProcess`, Name: "Universe", Type: "sz", Value: "Public"},
	{Key: `HKEY_LOCAL_MACHINE\Software\Valve\Steam`, Name: "InstallPath", Type: "sz", Value: `C:\Program Files (x86)\Steam`},
	{Key: `HKEY_LOCAL_MACHINE\Software\Wow6432Node\Valve\Steam`, Name: "InstallPath", Type: "sz", Value: `C:\Program Files (x86)\Steam`},
}

func withSteamBridge(ctx context.Context, in Input, tree wineruntime.Tree, env, command []string) ([]string, []string, error) {
	root := in.Runtime.Root(tree)
	if !hasSteamBridge(root) {
		in.Progress("this runtime has no Steam bridge, so the game cannot reach Steam")
		return env, command, nil
	}
	for target, source := range steamBridgeFiles {
		if err := copyIfChanged(filepath.Join(root, source), filepath.Join(in.Prefix, "drive_c", target)); err != nil {
			return nil, nil, failure.Errorf(failure.CodePrefixFailed, "putting the Steam bridge's %s into the prefix: %s", target, err)
		}
	}
	err := importRegistry(ctx, in, env, registryImport{
		Values:   steamBridgeRegistry,
		Marker:   ".macron-steam-bridge.json",
		FileName: "macron-steam-bridge.reg",
		Message:  "writing the registry keys games read to find Steam",
	})
	if err != nil {
		return nil, nil, err
	}
	in.Progress("starting it through steam.exe, which connects it to the Mac Steam client (Steam bridge)")
	return slices.Concat(env, steamBridgeEnv(env)), slices.Concat([]string{steamHelper}, command), nil
}

func steamBridgeEnv(env []string) []string {
	var added []string
	appID := firstValue(env, "SteamGameId", "SteamAppId", "STEAM_COMPAT_APP_ID")
	if appID != "" && envValue(env, "SteamGameId") == "" {
		added = append(added, "SteamGameId="+appID)
	}
	if appID != "" && envValue(env, "SteamAppId") == "" {
		added = append(added, "SteamAppId="+appID)
	}
	if envValue(env, "STEAM_COMPAT_CLIENT_INSTALL_PATH") == "" {
		added = append(added, "STEAM_COMPAT_CLIENT_INSTALL_PATH="+filepath.Join(envValue(env, "HOME"), "Library", "Application Support", "Steam"))
	}
	return added
}

func hasSteamBridge(root string) bool {
	sources := append(slices.Collect(maps.Values(steamBridgeFiles)), steamBridgeUnixLib)
	for _, source := range sources {
		if _, err := os.Stat(filepath.Join(root, source)); err != nil {
			return false
		}
	}
	return true
}

func copyIfChanged(source, target string) error {
	want, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if have, err := os.ReadFile(target); err == nil && bytes.Equal(have, want) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, want, 0o644)
}

func firstValue(env []string, names ...string) string {
	for _, name := range names {
		if value := envValue(env, name); value != "" {
			return value
		}
	}
	return ""
}

func envValue(env []string, name string) string {
	for _, entry := range slices.Backward(env) {
		if value, ok := strings.CutPrefix(entry, name+"="); ok {
			return value
		}
	}
	return ""
}
