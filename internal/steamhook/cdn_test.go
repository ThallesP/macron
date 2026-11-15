package steamhook

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

const fakeCDNManifest = `"osx"
{
	"version"		"1700000002"
	"ostype"		"macos1015"
	"steam_osx"
	{
		"steamchina"
		{
			"file"		"steam_osx_steamchina.zip.4"
			"sha2"		"%s"
			"IsBootstrapperPackage"		"1"
		}
		"macos-signed-2"
		{
			"file"		"steam_osx_macos-signed-2.zip.3"
			"sha2"		"%s"
		}
		"file"		"steam_osx.zip.2"
		"sha2"		"%s"
		"IsBootstrapperPackage"		"1"
	}
	"bins_client_osx"
	{
		"file"		"bins_client_osx.zip.1"
		"sha2"		"%s"
	}
}
"kvsignatures"
{
	"osx"		"00"
}
`

type fakeCDN struct {
	packages map[string][]byte
	manifest string
}

func fakePackages(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"bins_client_osx.zip.1":          zipped(t, "steamclient.dylib", string(wholeFakeClient.build())),
		"steam_osx.zip.2":                zipped(t, "steam_osx", "plain steam_osx"),
		"steam_osx_macos-signed-2.zip.3": zipped(t, "steam_osx", "signed steam_osx"),
		"steam_osx_steamchina.zip.4":     zipped(t, "steamchina_osx", "china steam"),
	}
}

func newFakeCDN(packages map[string][]byte) *fakeCDN {
	manifest := fmt.Sprintf(fakeCDNManifest,
		sha256Text(string(packages["steam_osx_steamchina.zip.4"])),
		sha256Text(string(packages["steam_osx_macos-signed-2.zip.3"])),
		sha256Text(string(packages["steam_osx.zip.2"])),
		sha256Text(string(packages["bins_client_osx.zip.1"])),
	)
	return &fakeCDN{packages: packages, manifest: manifest}
}

func (f *fakeCDN) serve(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/steam_client_osx" {
			w.Write([]byte(f.manifest))
			return
		}
		body, ok := f.packages[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func zipped(t *testing.T, name string, content string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

func TestFetchReleaseGivesARowPerVariantWithASteamOSX(t *testing.T) {
	cdn := newFakeCDN(fakePackages(t)).serve(t)

	release, err := FetchRelease(context.Background(), cdn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	client := KnownFile{Path: "steamclient.dylib", SHA256: sha256Text(string(wholeFakeClient.build())), Package: "bins_client_osx.zip.1"}
	code, err := FindClientCode(wholeFakeClient.build())
	if err != nil {
		t.Fatal(err)
	}
	want := Release{
		ID: "1700000002",
		Builds: []KnownBuild{
			{ID: "1700000002", Variant: "macos-signed-2", Client: code, Files: []KnownFile{
				client,
				{Path: "steam_osx", SHA256: sha256Text("signed steam_osx"), Package: "steam_osx_macos-signed-2.zip.3"},
			}},
			{ID: "1700000002", Variant: "plain", Client: code, Files: []KnownFile{
				client,
				{Path: "steam_osx", SHA256: sha256Text("plain steam_osx"), Package: "steam_osx.zip.2"},
			}},
		},
		SkippedVariants: []string{"steamchina"},
	}
	if !reflect.DeepEqual(release, want) {
		t.Errorf("release = %+v\nwant      %+v", release, want)
	}
}

func TestFetchReleaseRefusesAPackageThatDoesNotMatchItsSHA2(t *testing.T) {
	f := newFakeCDN(fakePackages(t))
	f.packages["steam_osx_macos-signed-2.zip.3"] = zipped(t, "steam_osx", "a different steam_osx")
	cdn := f.serve(t)

	_, err := FetchRelease(context.Background(), cdn, t.TempDir())

	assertCode(t, err, failure.CodeChecksumMismatch)
}

func TestFetchReleaseRefusesAFileNameWithAPath(t *testing.T) {
	f := newFakeCDN(fakePackages(t))
	f.manifest = strings.Replace(f.manifest, `"steam_osx.zip.2"`, `"../steam_osx.zip.2"`, 1)
	cdn := f.serve(t)

	_, err := FetchRelease(context.Background(), cdn, t.TempDir())

	assertCode(t, err, failure.CodeBadSteamManifest)
}

func TestFetchReleaseRefusesAManifestWithoutTheClientPackage(t *testing.T) {
	f := newFakeCDN(fakePackages(t))
	f.manifest = strings.Replace(f.manifest, `"bins_client_osx"`, `"bins_other_osx"`, 1)
	cdn := f.serve(t)

	_, err := FetchRelease(context.Background(), cdn, t.TempDir())

	assertCode(t, err, failure.CodeBadSteamManifest)
}

func TestFetchReleaseRefusesABuildWithoutAnySteamOSX(t *testing.T) {
	packages := fakePackages(t)
	packages["steam_osx.zip.2"] = zipped(t, "steam", "renamed plain")
	packages["steam_osx_macos-signed-2.zip.3"] = zipped(t, "steam", "renamed signed")
	cdn := newFakeCDN(packages).serve(t)

	_, err := FetchRelease(context.Background(), cdn, t.TempDir())

	assertCode(t, err, failure.CodeBadSteamManifest)
}
