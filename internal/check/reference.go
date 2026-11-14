package check

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/imagecheck"
	"github.com/ThallesP/macron/internal/launch"
	"github.com/ThallesP/macron/internal/scenario"
)

type ReferenceInput struct {
	Scenario scenario.Scenario
	Profile  fixes.Profile
	Out      string
	CacheDir string
	Progress func(string)
}

type ReferenceReport struct {
	Scenario      string           `json:"scenario"`
	Captured      bool             `json:"captured"`
	Error         string           `json:"error,omitempty"`
	Window        string           `json:"window,omitempty"`
	Size          string           `json:"size,omitempty"`
	Stability     imagecheck.Score `json:"stabilityBetweenCaptures"`
	SecondsToMenu float64          `json:"secondsToMenu"`
	Extras        []string         `json:"extras"`
}

type windowsWindow struct {
	Handle uintptr
	Title  string
}

func CaptureReference(ctx context.Context, in ReferenceInput) ReferenceReport {
	report := ReferenceReport{Scenario: in.Scenario.Name}
	if err := captureReference(ctx, in, &report); err != nil {
		report.Error = err.Error()
	}
	return report
}

func captureReference(ctx context.Context, in ReferenceInput, report *ReferenceReport) error {
	if err := os.MkdirAll(in.Out, 0o755); err != nil {
		return err
	}
	in.Progress("preparing the game files")
	exe, err := PrepareGame(ctx, in.Scenario, in.CacheDir, WorkDir(in.Scenario))
	if err != nil {
		return err
	}
	if err := launch.WriteFiles(in.Profile.Files, windowsPlaces(filepath.Dir(exe))); err != nil {
		return err
	}
	env := os.Environ()
	if slices.Contains(in.Scenario.WindowsExtra, "mesa-llvmpipe") {
		in.Progress("adding Mesa's software OpenGL next to the game")
		copied, err := addMesaLLVMPipe(ctx, in.CacheDir, WorkDir(in.Scenario), exe)
		if err != nil {
			return err
		}
		report.Extras = append(report.Extras, fmt.Sprintf("Mesa llvmpipe (%s): %v", mesaLLVMPipe.Name, copied))
		env = append(env, "GALLIUM_DRIVER=llvmpipe")
	}

	in.Progress("starting the game on Windows")
	log, err := os.Create(filepath.Join(in.Out, "game.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	output := &lockedBuffer{}
	game := exec.Command(exe, in.Profile.Args...)
	game.Dir = filepath.Dir(exe)
	game.Env = env
	game.Stdout = io.MultiWriter(log, output)
	game.Stderr = game.Stdout
	started := time.Now()
	if err := game.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", exe, err)
	}
	defer exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(game.Process.Pid)).Run()

	window, err := waitForWindowsWindow(ctx, in.Scenario, output)
	if err != nil {
		return err
	}
	report.SecondsToMenu = time.Since(started).Seconds()
	report.Window = window.Title
	time.Sleep(time.Duration(in.Scenario.Reached.SettleSeconds) * time.Second)

	in.Progress("capturing the window")
	first, err := captureClientArea(window.Handle)
	if err != nil {
		return err
	}
	time.Sleep(5 * time.Second)
	second, err := captureClientArea(window.Handle)
	if err != nil {
		return err
	}
	report.Size = fmt.Sprintf("%dx%d", first.Bounds().Dx(), first.Bounds().Dy())
	report.Stability = imagecheck.Compare(first, second, in.Scenario.Compare.Ignore)
	if err := WritePNG(filepath.Join(in.Out, "reference.png"), first); err != nil {
		return err
	}
	if err := WritePNG(filepath.Join(in.Out, "reference-5s-later.png"), second); err != nil {
		return err
	}
	if first.Bounds().Dx() != in.Scenario.Window.Width || first.Bounds().Dy() != in.Scenario.Window.Height {
		return fmt.Errorf("the window's picture is %s, the scenario expects %dx%d", report.Size, in.Scenario.Window.Width, in.Scenario.Window.Height)
	}
	report.Captured = true
	return nil
}

func waitForWindowsWindow(ctx context.Context, s scenario.Scenario, output *lockedBuffer) (windowsWindow, error) {
	deadline := time.Now().Add(time.Duration(s.Reached.TimeoutSeconds) * time.Second)
	pattern := s.TitlePattern()
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return windowsWindow{}, err
		}
		time.Sleep(2 * time.Second)
		windows, err := listWindowsWindows()
		if err != nil {
			return windowsWindow{}, err
		}
		index := slices.IndexFunc(windows, func(w windowsWindow) bool { return pattern.MatchString(w.Title) })
		if index < 0 || (s.Reached.Output != "" && !output.Contains(s.Reached.Output)) {
			continue
		}
		return windows[index], nil
	}
	windows, _ := listWindowsWindows()
	var titles bytes.Buffer
	for _, w := range windows {
		fmt.Fprintf(&titles, "%q ", w.Title)
	}
	return windowsWindow{}, fmt.Errorf("no window matching %q (and output %q) within %ds; windows: %s", s.Window.Title, s.Reached.Output, s.Reached.TimeoutSeconds, titles.String())
}

func windowsPlaces(gameDir string) map[string]string {
	profile := os.Getenv("USERPROFILE")
	return map[string]string{
		"{game}":         gameDir,
		"{userprofile}":  profile,
		"{appdata}":      os.Getenv("APPDATA"),
		"{localappdata}": os.Getenv("LOCALAPPDATA"),
		"{documents}":    filepath.Join(profile, "Documents"),
	}
}
