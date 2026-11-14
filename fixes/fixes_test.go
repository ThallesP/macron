package fixes

import (
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
		"unknown install":  `{"id": "a", "backend": "dxmt", "installs": ["vcrun1999"]}`,
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
		"files": [{"path": "{appdata}/Some Game/settings.ini", "content": "width=800"}]
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
