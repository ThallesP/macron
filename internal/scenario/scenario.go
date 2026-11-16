package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ThallesP/macron/fixes"
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

type FrameRate struct {
	MinFPS             float64 `json:"minFps"`
	MinFPSSource       string  `json:"minFpsSource"`
	BaselineExe        string  `json:"baselineExe,omitempty"`
	MinShareOfBaseline float64 `json:"minShareOfBaseline,omitempty"`
}

func (f FrameRate) HasThreshold() bool {
	return f.MinFPS > 0 || f.MinShareOfBaseline > 0
}

// Uploads starts each exe with an OpenGL trace and requires that its buffer
// uploads while it measures make the host OpenGL wait for nothing (no
// glBufferSubData, no synchronized map), and that a control run with the
// unsynchronized writes switched off shows the waiting uploads.
type Uploads struct {
	Exes   []string `json:"exes"`
	Args   []string `json:"args"`
	Source string   `json:"source"`
}

type Download struct {
	wineruntime.Archive
	Unpack string `json:"unpack"`
	Into   string `json:"into"`
}

type Scenario struct {
	Name          string            `json:"name"`
	Title         string            `json:"title"`
	Game          string            `json:"game"`
	Downloads     []Download        `json:"downloads"`
	Build         string            `json:"build"`
	Exe           string            `json:"exe"`
	Cwd           string            `json:"cwd"`
	Args          []string          `json:"args"`
	MacronEnv     map[string]string `json:"macronEnv"`
	Files         []fixes.File      `json:"files"`
	Window        Window            `json:"window"`
	Reached       Reached           `json:"reached"`
	Compare       Compare           `json:"compare"`
	Reference     Reference         `json:"reference"`
	OtherGame     string            `json:"otherGame"`
	WindowsExtra  []string          `json:"windowsExtra"`
	FrameRate     *FrameRate        `json:"frameRate,omitempty"`
	Uploads       *Uploads          `json:"uploads,omitempty"`
	StartAttempts int               `json:"startAttempts,omitempty"`
	CI            bool              `json:"ci"`

	Dir string `json:"-"`
}

const FrameRateLine = "RESULT fps"

func uploadExes(uploads *Uploads) []string {
	if uploads == nil {
		return nil
	}
	return uploads.Exes
}

var (
	validName     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	unpackFormats = []string{"zip", "tar", "7z"}
	windowsExtras = []string{"mesa-llvmpipe"}
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
	if (len(s.Downloads) == 0) == (s.Build == "") {
		return invalid("exactly one of downloads and build is required")
	}
	for _, d := range s.Downloads {
		if d.URL == "" || len(d.SHA256) != 64 || d.Name == "" || !slices.Contains(unpackFormats, d.Unpack) {
			return invalid("each download needs a name, url, 64-character sha256 and an unpack format (%s)", strings.Join(unpackFormats, ", "))
		}
		if !isInside(d.Into) {
			return invalid("download %s unpacks outside the game folder (%q)", d.Name, d.Into)
		}
	}
	if !isInside(s.Exe) || !isInside(s.Cwd) {
		return invalid("exe and cwd must be paths inside the game folder")
	}
	if s.Window.Title == "" || s.Window.Width <= 0 || s.Window.Height <= 0 {
		return invalid("window needs a title pattern, a width and a height")
	}
	if _, err := regexp.Compile(s.Window.Title); err != nil {
		return invalid("window title is not a pattern: %s", err)
	}
	if err := fixes.ValidateFiles("files", s.Files); err != nil {
		return invalid("%s", err)
	}
	for name := range s.MacronEnv {
		if !strings.HasPrefix(name, "MACRON_") {
			return invalid("macronEnv may only set MACRON_ variables, not %s", name)
		}
	}
	if s.Reached.TimeoutSeconds <= 0 {
		return invalid("reached.timeoutSeconds is required")
	}
	if s.Reference.Image == "" || s.Reference.Source == "" {
		return invalid("reference needs an image and where it came from")
	}
	if s.FrameRate != nil && (s.FrameRate.MinFPS < 0 || s.FrameRate.MinFPSSource == "") {
		return invalid("frameRate needs a minFps of 0 or more and a minFpsSource saying where it came from, or that it is not set yet")
	}
	if s.FrameRate != nil && (s.FrameRate.MinShareOfBaseline < 0 || s.FrameRate.MinShareOfBaseline > 1) {
		return invalid("frameRate.minShareOfBaseline is a share of the baseline's frame rate, from 0 to 1")
	}
	if s.FrameRate != nil && (s.FrameRate.BaselineExe == "") != (s.FrameRate.MinShareOfBaseline == 0) {
		return invalid("frameRate.baselineExe and frameRate.minShareOfBaseline go together")
	}
	if s.FrameRate != nil && s.FrameRate.BaselineExe != "" && (strings.ContainsAny(s.FrameRate.BaselineExe, `/\`) || !strings.HasSuffix(s.FrameRate.BaselineExe, ".exe")) {
		return invalid("frameRate.baselineExe is an .exe next to the scenario's exe, not a path")
	}
	if s.FrameRate != nil && s.CI && !s.FrameRate.HasThreshold() {
		return invalid("a frame-rate scenario runs in CI only with a threshold (frameRate.minFps or minShareOfBaseline above 0)")
	}
	if s.StartAttempts < 0 || s.StartAttempts > 3 {
		return invalid("startAttempts is how many times a program that exits before its condition is started, from 1 to 3")
	}
	if s.FrameRate != nil && s.Reached.Output != FrameRateLine {
		return invalid("a frame-rate scenario is reached when the probe prints %q", FrameRateLine)
	}
	if s.Uploads != nil && (len(s.Uploads.Exes) == 0 || len(s.Uploads.Args) == 0 || s.Uploads.Source == "") {
		return invalid("uploads needs exes, args and a source saying why the rule holds")
	}
	if s.Uploads != nil && s.Reached.Output != FrameRateLine {
		return invalid("an uploads scenario is reached when the probe prints %q", FrameRateLine)
	}
	for _, exe := range uploadExes(s.Uploads) {
		if strings.ContainsAny(exe, `/\`) || !strings.HasSuffix(exe, ".exe") {
			return invalid("uploads.exes are .exe files next to the scenario's exe, not paths")
		}
	}
	for _, extra := range s.WindowsExtra {
		if !slices.Contains(windowsExtras, extra) {
			return invalid("unknown Windows extra %q", extra)
		}
	}
	return nil
}

func isInside(path string) bool {
	return !filepath.IsAbs(path) && !slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..")
}

func (s Scenario) ReferencePath() string {
	return filepath.Join(s.Dir, s.Reference.Image)
}

func (s Scenario) TitlePattern() *regexp.Regexp {
	return regexp.MustCompile(s.Window.Title)
}
