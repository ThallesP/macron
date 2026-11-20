package fixes

import (
	"slices"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

func TestEveryCommittedProfileLoads(t *testing.T) {
	db, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Get("d3d11-probe"); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsBadProfiles(t *testing.T) {
	cases := map[string]string{
		"unknown backend":  `{"id": "a", "backend": "vulkan"}`,
		"unknown field":    `{"id": "a", "backend": "dxmt", "backnd": "dxmt"}`,
		"bad id":           `{"id": "My Game", "backend": "dxmt"}`,
		"file outside":     `{"id": "a", "backend": "dxmt", "files": [{"path": "/etc/hosts", "content": ""}]}`,
		"file escapes":     `{"id": "a", "backend": "dxmt", "files": [{"path": "{game}/../../x", "content": ""}]}`,
		"bad DLL override": `{"id": "a", "backend": "dxmt", "dllOverrides": {"d3d9": "sometimes"}}`,
		"registry outside": `{"id": "a", "backend": "dxmt", "registry": [{"key": "HKEY_USERS\\x", "name": "a", "type": "sz", "value": "b"}]}`,
		"bad dword":        `{"id": "a", "backend": "dxmt", "registry": [{"key": "HKEY_CURRENT_USER\\x", "name": "a", "type": "dword", "value": "zz"}]}`,
		"unknown install":  `{"id": "a", "backend": "dxmt", "installs": ["vcrun1999"]}`,
		"chips, no size":   `{"id": "a", "backend": "dxmt", "renderPixelsChips": ["Apple M3"]}`,
		"negative size":    `{"id": "a", "backend": "dxmt", "renderPixels": -1}`,
		"size, not dxmt":   `{"id": "a", "backend": "wined3d", "renderPixels": 1000000}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(content))

			if failure.CodeOf(err) != failure.CodeBadProfile {
				t.Fatalf("got %v, want %s", err, failure.CodeBadProfile)
			}
		})
	}
}

func TestParseAcceptsAFullProfile(t *testing.T) {
	profile, err := Parse([]byte(`{
		"id": "some-game",
		"name": "Some Game",
		"steamAppIds": [10],
		"backend": "wined3d",
		"installs": ["vcrun2022"],
		"args": ["-windowed"],
		"env": {"DXMT_LOG_LEVEL": "info"},
		"dllOverrides": {"d3d9": "n,b"},
		"files": [{"path": "{appdata}/Some Game/settings.ini", "content": "width=800"}],
		"registry": [{"key": "HKEY_CURRENT_USER\\Software\\Wine\\Mac Driver", "name": "AllowSoftwareRendering", "type": "sz", "value": "y"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if profile.Backend != BackendWineD3D || profile.Files[0].Content != "width=800" {
		t.Fatalf("parsed %+v", profile)
	}
}

func TestUnknownSteamAppGetsTheDefaultProfile(t *testing.T) {
	db := Database{profiles: []Profile{{ID: "known", Backend: BackendWineD3D, SteamAppIDs: []int{70}}}}

	if got := db.ForSteamApp(70); got.ID != "known" {
		t.Errorf("app 70 got %q", got.ID)
	}
	if got := db.ForSteamApp(71); got.ID != "steam-71" || got.Backend != BackendDXMT {
		t.Errorf("app 71 got %+v", got)
	}
}

func TestGetUnknownGame(t *testing.T) {
	_, err := Database{}.Get("nope")

	if failure.CodeOf(err) != failure.CodeUnknownGame {
		t.Fatalf("got %v", err)
	}
}

func TestDeadlockIsShownASmallerScreenOnTheBaseChips(t *testing.T) {
	db, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	deadlock := db.ForSteamApp(1422450)

	if deadlock.ID != "deadlock" || deadlock.RenderPixels != 1440*932 {
		t.Fatalf("app 1422450 got profile %q with renderPixels %d, want deadlock with 1440x932", deadlock.ID, deadlock.RenderPixels)
	}
	if !slices.Contains(deadlock.RenderPixelsChips, "Apple M3") || slices.Contains(deadlock.RenderPixelsChips, "Apple M3 Max") {
		t.Errorf("chips %q: want the M3 and not the M3 Max", deadlock.RenderPixelsChips)
	}
}
