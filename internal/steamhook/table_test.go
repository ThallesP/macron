package steamhook

import (
	"os"
	"strings"
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

func TestRenderHeaderReproducesTheSwitchLibrarysTable(t *testing.T) {
	committed, err := os.ReadFile("../../steamplay/builds.h")
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := RenderHeader(KnownBuilds)
	if err != nil {
		t.Fatal(err)
	}

	if rendered != string(committed) {
		t.Errorf("steamplay/builds.h is not what KnownBuilds renders to; run: go run ./cmd/steam-builds --header-only steamplay/builds.h\n%s", rendered)
	}
}

func TestRenderHeaderListsASteamclientOnce(t *testing.T) {
	client := ClientCode{UUID: "aa", Enabled: Instruction{Address: 0x10, Original: 1, Patched: 2}, Stop: Instruction{Address: 0x20, Original: 3, Patched: 4}}
	header, err := RenderHeader([]KnownBuild{{ID: "1", Variant: "macos-signed-2", Client: client}, {ID: "1", Variant: "plain", Client: client}})
	if err != nil {
		t.Fatal(err)
	}

	row := `    {"1", "aa", { 0x10, 0x00000001, 0x00000002 }, { 0x20, 0x00000003, 0x00000004 }},`
	if strings.Count(header, row) != 1 {
		t.Errorf("want this row once:\n%s\ngot:\n%s", row, header)
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
