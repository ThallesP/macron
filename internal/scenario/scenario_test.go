package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/failure"
)

const valid = `{
  "name": "some-game",
  "title": "Some Game",
  "game": "some-game",
  "downloads": [
    {"name": "game.zip", "url": "https://example.org/game.zip", "sha256": "%SHA%", "sizeBytes": 1, "unpack": "zip", "into": "."},
    {"name": "data.zip", "url": "https://example.org/data.zip", "sha256": "%SHA%", "sizeBytes": 1, "unpack": "zip", "into": "Game/data"}
  ],
  "exe": "Game/game.exe",
  "cwd": "Game",
  "window": {"title": "^Some Game$", "width": 800, "height": 600},
  "reached": {"output": "", "settleSeconds": 5, "timeoutSeconds": 600},
  "compare": {"text": ["New Game"], "ignore": []},
  "reference": {"image": "reference.png", "source": "a Windows run"},
  "otherGame": "",
  "windowsExtra": ["mesa-llvmpipe"],
  "ci": true
}`

func write(t *testing.T, name, content string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content = strings.ReplaceAll(content, "%SHA%", strings.Repeat("a", 64))
	if err := os.WriteFile(filepath.Join(dir, "scenario.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadAValidScenario(t *testing.T) {
	s, err := Load(write(t, "some-game", valid))
	if err != nil {
		t.Fatal(err)
	}

	if len(s.Downloads) != 2 || s.Downloads[1].Into != "Game/data" || s.Cwd != "Game" || !s.CI {
		t.Fatalf("loaded %+v", s)
	}
}

func TestLoadRejectsBadScenarios(t *testing.T) {
	cases := map[string][2]string{
		"name differs from folder": {`"name": "some-game"`, `"name": "other-game"`},
		"exe leaves the folder":    {`"exe": "Game/game.exe"`, `"exe": "../game.exe"`},
		"download leaves":          {`"into": "Game/data"`, `"into": "../../etc"`},
		"short checksum":           {`"sha256": "%SHA%", "sizeBytes": 1, "unpack": "zip", "into": "."`, `"sha256": "abc", "sizeBytes": 1, "unpack": "zip", "into": "."`},
		"unknown unpack":           {`"unpack": "zip", "into": "."`, `"unpack": "rar", "into": "."`},
		"no timeout":               {`"timeoutSeconds": 600`, `"timeoutSeconds": 0`},
		"no reference source":      {`"source": "a Windows run"`, `"source": ""`},
		"bad title pattern":        {`"title": "^Some Game$"`, `"title": "(("`},
		"unknown extra":            {`["mesa-llvmpipe"]`, `["dxvk-on-windows"]`},
		"unknown field":            {`"ci": true`, `"ci": true, "threshold": 0.5`},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			content := strings.Replace(valid, change[0], change[1], 1)
			if content == valid {
				t.Fatalf("the change %q did not apply", change[0])
			}

			_, err := Load(write(t, "some-game", content))

			if failure.CodeOf(err) != failure.CodeInvalidInput {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestEveryCommittedScenarioLoads(t *testing.T) {
	dirs, err := filepath.Glob("../../scenarios/*/scenario.json")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no scenarios found: %v", err)
	}
	for _, path := range dirs {
		s, err := Load(filepath.Dir(path))
		if err != nil {
			t.Error(err)
			continue
		}
		if _, err := os.Stat(s.ReferencePath()); s.CI && err != nil {
			t.Errorf("%s runs in CI but has no reference: %s", s.Name, err)
		}
	}
}
