package wineruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestEnsureMesaUnpacksTheOpenGLDLLsOnce(t *testing.T) {
	body := tarball(t, map[string]string{
		"x64/opengl32.dll":       "gl 64",
		"x64/libgallium_wgl.dll": "gallium 64",
		"x86/opengl32.dll":       "gl 32",
		"x86/libgallium_wgl.dll": "gallium 32",
		"x64/vulkan_lvp.dll":     "not wanted",
	})
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	archive := Archive{Name: "mesa.tar.gz", URL: server.URL + "/mesa.tar.gz", SHA256: sum(body)}
	layout := Layout{Home: t.TempDir()}
	installed := Installed{Version: "test", Dir: t.TempDir()}

	for range 2 {
		if err := EnsureMesa(context.Background(), layout.DownloadsDir(), installed, archive, func(string) {}); err != nil {
			t.Fatal(err)
		}
	}

	if got := read(t, filepath.Join(installed.Mesa(Mesa64), "opengl32.dll")); got != "gl 64" {
		t.Errorf("x64 opengl32.dll = %q", got)
	}
	if got := read(t, filepath.Join(installed.Mesa(Mesa32), "libgallium_wgl.dll")); got != "gallium 32" {
		t.Errorf("x86 libgallium_wgl.dll = %q", got)
	}
	if requests != 1 {
		t.Errorf("downloaded %d times", requests)
	}
}
