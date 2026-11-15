package wineruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

func withSteamBridge(t *testing.T, f *fixture, files map[string]string) {
	t.Helper()
	bridge := tarball(t, files)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		w.Write(bridge)
	}))
	t.Cleanup(server.Close)
	f.manifest.SteamBridge = Archive{Name: "bridge.tar.gz", URL: server.URL + "/bridge.tar.gz", SHA256: sum(bridge), SizeBytes: int64(len(bridge))}
}

func TestInstallPutsTheSteamBridgeInBothTrees(t *testing.T) {
	f := serve(t)
	withSteamBridge(t, f, map[string]string{
		"lib/wine/i386-windows/lsteamclient.dll":                "bridge 32",
		"lib/wine/x86_64-windows/lsteamclient.dll":              "bridge 64",
		"lib/wine/x86_64-windows/steam.exe":                     "steam helper",
		"lib/wine/x86_64-unix/lsteamclient.so":                  "bridge unix",
		"share/doc/macron-steam-bridge/LICENSE.lsteamclient":    "steamworks sdk licence",
		"share/doc/macron-steam-bridge/LICENSE.steam_helper":    "bsd",
		"share/doc/macron-steam-bridge/bridge-check.txt":        "checked",
		"share/doc/macron-steam-bridge/0001-lsteamclient.patch": "patch",
	})
	layout := Layout{Home: t.TempDir()}

	installed, err := Install(context.Background(), layout, f.manifest, func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	for _, tree := range []Tree{TreePlain, TreeDXMT} {
		for path, want := range map[string]string{
			"lib/wine/i386-windows/lsteamclient.dll":             "bridge 32",
			"lib/wine/x86_64-windows/lsteamclient.dll":           "bridge 64",
			"lib/wine/x86_64-windows/steam.exe":                  "steam helper",
			"lib/wine/x86_64-unix/lsteamclient.so":               "bridge unix",
			"share/doc/macron-steam-bridge/LICENSE.lsteamclient": "steamworks sdk licence",
			"lib/wine/x86_64-windows/d3d11.dll":                  map[Tree]string{TreePlain: "wine d3d11", TreeDXMT: "dxmt d3d11"}[tree],
		} {
			if got := read(t, filepath.Join(installed.Root(tree), path)); got != want {
				t.Errorf("%s tree %s = %q, want %q", tree, path, got, want)
			}
		}
	}
	if f.requests != 5 {
		t.Errorf("made %d downloads, want 5", f.requests)
	}
}

func TestInstallWithoutASteamBridgeDownloadsNoBridge(t *testing.T) {
	f := serve(t)
	layout := Layout{Home: t.TempDir()}

	installed, err := Install(context.Background(), layout, f.manifest, func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(installed.Root(TreePlain), steamBridgeUnixLib)); err == nil {
		t.Error("the plain tree has a Steam bridge nobody pinned")
	}
	if f.requests != 4 {
		t.Errorf("made %d downloads, want 4", f.requests)
	}
}

func TestASteamBridgeWithoutItsUnixLibraryInstallsNothing(t *testing.T) {
	f := serve(t)
	withSteamBridge(t, f, map[string]string{"lib/wine/x86_64-windows/lsteamclient.dll": "bridge 64"})
	layout := Layout{Home: t.TempDir()}

	_, err := Install(context.Background(), layout, f.manifest, func(string) {})

	if failure.CodeOf(err) != failure.CodeExtractFailed {
		t.Fatalf("got %v, want %s", err, failure.CodeExtractFailed)
	}
	if _, err := Find(layout, f.manifest); failure.CodeOf(err) != failure.CodeRuntimeMissing {
		t.Fatalf("Find after a failed install = %v", err)
	}
}

func TestPinnedRuntimesWithoutABridgeStillMatch(t *testing.T) {
	f := serve(t)
	layout := Layout{Home: t.TempDir()}
	if _, err := Install(context.Background(), layout, f.manifest, func(string) {}); err != nil {
		t.Fatal(err)
	}
	recorded := filepath.Join(layout.RuntimeDir(f.manifest.Version), installedMarker)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(read(t, recorded)), &fields); err != nil {
		t.Fatal(err)
	}
	delete(fields, "steamBridge")
	older, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recorded, older, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Find(layout, f.manifest); err != nil {
		t.Fatalf("a runtime recorded before the steamBridge field existed is no longer found: %v", err)
	}
}
