package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/imagecheck"
	"github.com/ThallesP/macron/internal/wineruntime"
)

type Window struct {
	Title  string `json:"title"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Reached struct {
	Output         string `json:"output"`
	SettleSeconds  int    `json:"settleSeconds"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type Compare struct {
	Text   []string          `json:"text"`
	Ignore []imagecheck.Rect `json:"ignore"`
}

type Reference struct {
	Image  string `json:"image"`
	Source string `json:"source"`
}

type Download struct {
	wineruntime.Archive
	Unpack string `json:"unpack"`
}

type Scenario struct {
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Game         string    `json:"game"`
	Download     *Download `json:"download"`
	Build        string    `json:"build"`
	Exe          string    `json:"exe"`
	Window       Window    `json:"window"`
	Reached      Reached   `json:"reached"`
	Compare      Compare   `json:"compare"`
	Reference    Reference `json:"reference"`
	OtherGame    string    `json:"otherGame"`
	WindowsExtra []string  `json:"windowsExtra"`
	CI           bool      `json:"ci"`

	Dir string `json:"-"`
}

var (
	validName     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	unpackFormats = []string{"zip", "tar", "7z"}
	windowsExtras = []string{}
)

func Load(dir string) (Scenario, error) {
	content, err := os.ReadFile(filepath.Join(dir, "scenario.json"))
	if err != nil {
		return Scenario{}, failure.Errorf(failure.CodeInvalidInput, "no scenario at %s: %s", dir, err)
	}
	var s Scenario
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&s); err != nil {
		return Scenario{}, failure.Errorf(failure.CodeInvalidInput, "%s/scenario.json: %s", dir, err)
	}
	s.Dir = dir
	if err := s.validate(); err != nil {
		return Scenario{}, err
	}
	return s, nil
}

func (s Scenario) validate() error {
	invalid := func(format string, args ...any) error {
		return failure.Errorf(failure.CodeInvalidInput, "scenario %s: "+format, append([]any{s.Name}, args...)...)
	}
	if !validName.MatchString(s.Name) || filepath.Base(s.Dir) != s.Name {
		return invalid("the name must match its folder and be lowercase letters, digits and dashes")
	}
	if s.Game == "" || s.Exe == "" {
		return invalid("game and exe are required")
	}
	if (s.Download == nil) == (s.Build == "") {
		return invalid("exactly one of download and build is required")
	}
	if s.Download != nil && (s.Download.URL == "" || len(s.Download.SHA256) != 64 || s.Download.Name == "" || !slices.Contains(unpackFormats, s.Download.Unpack)) {
		return invalid("download needs a name, url, 64-character sha256 and an unpack format (%s)", strings.Join(unpackFormats, ", "))
	}
	if s.Window.Title == "" || s.Window.Width <= 0 || s.Window.Height <= 0 {
		return invalid("window needs a title pattern, a width and a height")
	}
	if _, err := regexp.Compile(s.Window.Title); err != nil {
		return invalid("window title is not a pattern: %s", err)
	}
	if s.Reached.TimeoutSeconds <= 0 {
		return invalid("reached.timeoutSeconds is required")
	}
	if s.Reference.Image == "" || s.Reference.Source == "" {
		return invalid("reference needs an image and where it came from")
	}
	for _, extra := range s.WindowsExtra {
		if !slices.Contains(windowsExtras, extra) {
			return invalid("unknown Windows extra %q", extra)
		}
	}
	return nil
}

func (s Scenario) ReferencePath() string {
	return filepath.Join(s.Dir, s.Reference.Image)
}

func (s Scenario) TitlePattern() *regexp.Regexp {
	return regexp.MustCompile(s.Window.Title)
}
