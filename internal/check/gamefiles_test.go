package check

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ThallesP/macron/internal/scenario"
	"github.com/ThallesP/macron/internal/wineruntime"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, content := range files {
		archive.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))})
		archive.Write([]byte(content))
	}
	archive.Close()
	zipped.Close()
	return buffer.Bytes()
}

func TestPrepareGameUnpacksEveryDownloadIntoPlace(t *testing.T) {
	bodies := map[string][]byte{
		"/game.tar.gz": tarGz(t, map[string]string{"Game/game.exe": "MZ", "Game/readme.txt": "hi"}),
		"/data.tar.gz": tarGz(t, map[string]string{"graphics.dat": "pixels"}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(bodies[r.URL.Path]) }))
	t.Cleanup(server.Close)
	download := func(name, into string) scenario.Download {
		hash := sha256.Sum256(bodies["/"+name])
		return scenario.Download{Archive: wineruntime.Archive{Name: name, URL: server.URL + "/" + name, SHA256: hex.EncodeToString(hash[:])}, Unpack: "tar", Into: into}
	}
	s := scenario.Scenario{
		Name:      "some-game",
		Dir:       t.TempDir(),
		Downloads: []scenario.Download{download("game.tar.gz", "."), download("data.tar.gz", "Game/baseset")},
		Exe:       "Game/game.exe",
	}
	work := t.TempDir()

	game, err := PrepareGame(context.Background(), s, filepath.Join(work, "cache"), work)
	if err != nil {
		t.Fatal(err)
	}

	if game.Exe != filepath.Join(work, "game", "Game", "game.exe") || game.Cwd != filepath.Dir(game.Exe) {
		t.Errorf("game %+v", game)
	}
	if content, err := os.ReadFile(filepath.Join(work, "game", "Game", "baseset", "graphics.dat")); err != nil || string(content) != "pixels" {
		t.Errorf("graphics.dat: %q, %v", content, err)
	}
}

func TestPrepareGameNeedsTheBuiltProgram(t *testing.T) {
	s := scenario.Scenario{Name: "probe", Dir: t.TempDir(), Build: "build.sh", Exe: "probe.exe"}

	if _, err := PrepareGame(context.Background(), s, t.TempDir(), t.TempDir()); err == nil {
		t.Fatal("a scenario whose program was never built prepared fine")
	}
}
