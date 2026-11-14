package wineruntime

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

	"github.com/ThallesP/macron/internal/failure"
)

func tarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, content := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func sum(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

type fixture struct {
	manifest Manifest
	requests int
}

func serve(t *testing.T) *fixture {
	t.Helper()
	wine := tarball(t, map[string]string{
		"Wine Devel.app/Contents/Resources/wine/bin/wine":                          "wine",
		"Wine Devel.app/Contents/Resources/wine/lib/wine/x86_64-windows/d3d11.dll": "wine d3d11",
		"Wine Devel.app/Contents/Resources/wine/lib/wine/x86_64-unix/ntdll.so":     "ntdll",
		"Wine Devel.app/Contents/Resources/wine/lib/wine/i386-windows/d3d11.dll":   "wine d3d11 32",
	})
	dxmt := tarball(t, map[string]string{
		"v0.80/x86_64-windows/d3d11.dll":     "dxmt d3d11",
		"v0.80/x86_64-windows/winemetal.dll": "winemetal",
		"v0.80/x86_64-unix/winemetal.so":     "winemetal unix",
		"v0.80/i386-windows/d3d11.dll":       "dxmt d3d11 32",
	})
	dxvk := tarball(t, map[string]string{
		"dxvk-macOS/x64/d3d11.dll":     "dxvk d3d11",
		"dxvk-macOS/x64/d3d10core.dll": "dxvk d3d10core",
		"dxvk-macOS/x32/d3d11.dll":     "dxvk d3d11 32",
	})
	f := &fixture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		bodies := map[string][]byte{"/wine.tar.gz": wine, "/dxmt.tar.gz": dxmt, "/dxvk.tar.gz": dxvk}
		body, ok := bodies[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	f.manifest = Manifest{
		Version: "test-1",
		Wine:    Archive{Name: "wine.tar.gz", URL: server.URL + "/wine.tar.gz", SHA256: sum(wine), SizeBytes: int64(len(wine))},
		DXMT:    Archive{Name: "dxmt.tar.gz", URL: server.URL + "/dxmt.tar.gz", SHA256: sum(dxmt), SizeBytes: int64(len(dxmt))},
		DXVK:    Archive{Name: "dxvk.tar.gz", URL: server.URL + "/dxvk.tar.gz", SHA256: sum(dxvk), SizeBytes: int64(len(dxvk))},
	}
	return f
}

func read(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestInstallBuildsAPlainAndADXMTTree(t *testing.T) {
	f := serve(t)
	layout := Layout{Home: t.TempDir()}

	installed, err := Install(context.Background(), layout, f.manifest, func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	if got := read(t, filepath.Join(installed.Root(TreePlain), "lib/wine/x86_64-windows/d3d11.dll")); got != "wine d3d11" {
		t.Errorf("plain tree d3d11.dll = %q", got)
	}
	if got := read(t, filepath.Join(installed.Root(TreeDXMT), "lib/wine/x86_64-windows/d3d11.dll")); got != "dxmt d3d11" {
		t.Errorf("DXMT tree d3d11.dll = %q", got)
	}
	if got := read(t, filepath.Join(installed.Root(TreeDXMT), "lib/wine/i386-windows/d3d11.dll")); got != "dxmt d3d11 32" {
		t.Errorf("DXMT tree 32-bit d3d11.dll = %q", got)
	}
	if got := read(t, filepath.Join(installed.Root(TreeDXMT), "lib/wine/x86_64-unix/winemetal.so")); got != "winemetal unix" {
		t.Errorf("DXMT tree winemetal.so = %q", got)
	}
	if got := read(t, installed.Wine(TreeDXMT)); got != "wine" {
		t.Errorf("DXMT tree bin/wine = %q", got)
	}
	if got := read(t, filepath.Join(installed.DXVK(DXVK64), "d3d11.dll")); got != "dxvk d3d11" {
		t.Errorf("DXVK x64 d3d11.dll = %q", got)
	}
	if got := read(t, filepath.Join(installed.DXVK(DXVK32), "d3d11.dll")); got != "dxvk d3d11 32" {
		t.Errorf("DXVK x32 d3d11.dll = %q", got)
	}
	if _, err := os.Stat(filepath.Join(installed.Root(TreePlain), "lib/wine/x86_64-unix/winemetal.so")); err == nil {
		t.Error("the plain tree has DXMT's files")
	}
}

func TestInstallTwiceDownloadsOnce(t *testing.T) {
	f := serve(t)
	layout := Layout{Home: t.TempDir()}

	if _, err := Install(context.Background(), layout, f.manifest, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(layout.RuntimeDir(f.manifest.Version)); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), layout, f.manifest, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(context.Background(), layout, f.manifest, func(string) {}); err != nil {
		t.Fatal(err)
	}

	if f.requests != 3 {
		t.Fatalf("made %d downloads, want 3 (cached after the first install)", f.requests)
	}
}

func TestChecksumMismatchInstallsNothing(t *testing.T) {
	f := serve(t)
	f.manifest.DXVK.SHA256 = sum([]byte("something else"))
	layout := Layout{Home: t.TempDir()}

	_, err := Install(context.Background(), layout, f.manifest, func(string) {})

	if failure.CodeOf(err) != failure.CodeChecksumMismatch {
		t.Fatalf("got %v, want %s", err, failure.CodeChecksumMismatch)
	}
	if f.requests != 3 {
		t.Errorf("retried a checksum mismatch: %d requests", f.requests)
	}
	if _, err := Find(layout, f.manifest); failure.CodeOf(err) != failure.CodeRuntimeMissing {
		t.Fatalf("Find after a failed install = %v", err)
	}
}

func TestFindRejectsADifferentRuntime(t *testing.T) {
	f := serve(t)
	layout := Layout{Home: t.TempDir()}
	if _, err := Install(context.Background(), layout, f.manifest, func(string) {}); err != nil {
		t.Fatal(err)
	}
	changed := f.manifest
	changed.DXMT.SHA256 = sum([]byte("a newer DXMT"))

	_, err := Find(layout, changed)

	if failure.CodeOf(err) != failure.CodeRuntimeMissing {
		t.Fatalf("got %v, want %s", err, failure.CodeRuntimeMissing)
	}
}
