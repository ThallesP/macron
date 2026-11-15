package steamhook

import (
	"os"
	"testing"
)

func TestRenderTableReproducesBuildsGo(t *testing.T) {
	committed, err := os.ReadFile("builds.go")
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := RenderTable(KnownBuilds)
	if err != nil {
		t.Fatal(err)
	}

	if string(rendered) != string(committed) {
		t.Errorf("RenderTable(KnownBuilds) differs from builds.go, so steam-builds --write would reformat it:\n%s", rendered)
	}
}

func TestIsInMatchesEveryFileHash(t *testing.T) {
	table := []KnownBuild{{ID: "1", Variant: "macos-signed-2", Files: []KnownFile{
		{Path: "steamclient.dylib", SHA256: sha256Text("client")},
		{Path: "steam_osx", SHA256: sha256Text("signed")},
	}}}
	sameFiles := KnownBuild{ID: "1", Variant: "renamed", Files: []KnownFile{
		{Path: "steamclient.dylib", SHA256: sha256Text("client"), Package: "bins_client_osx.zip.1"},
		{Path: "steam_osx", SHA256: sha256Text("signed"), Package: "steam_osx_macos-signed-2.zip.3"},
	}}
	otherSteamOSX := KnownBuild{ID: "1", Variant: "plain", Files: []KnownFile{
		{Path: "steamclient.dylib", SHA256: sha256Text("client")},
		{Path: "steam_osx", SHA256: sha256Text("plain")},
	}}

	if !sameFiles.IsIn(table) {
		t.Error("a build with the same hashes is not in the table")
	}
	if otherSteamOSX.IsIn(table) {
		t.Error("a build with another steam_osx is in the table")
	}
}
