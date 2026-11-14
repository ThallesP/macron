package check

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ThallesP/macron/internal/imagecheck"
	"github.com/ThallesP/macron/internal/scenario"
)

type MacInput struct {
	Scenario  scenario.Scenario
	Macron    string
	Out       string
	CacheDir  string
	OtherGame string
	Progress  func(string)
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) Contains(text string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Contains(b.buffer.Bytes(), []byte(text))
}

type macRun struct {
	MacInput
	report  Report
	started time.Time
	helper  macwin
}

func RunOnMac(ctx context.Context, in MacInput) Report {
	run := &macRun{
		MacInput: in,
		started:  time.Now(),
		report:   Report{Scenario: in.Scenario.Name, Where: describeMac(ctx), TimingsSecs: map[string]float64{}, Notes: []string{}},
	}
	if err := run.do(ctx); err != nil {
		run.report.Error = err.Error()
		run.report.Passed = false
	}
	run.report.TimingsSecs["total"] = time.Since(run.started).Seconds()
	return run.report
}

func (r *macRun) stage(name string) {
	r.report.Stage = name
	r.Progress(name)
}

func (r *macRun) mark(name string, since time.Time) {
	r.report.TimingsSecs[name] = time.Since(since).Seconds()
}

func (r *macRun) do(ctx context.Context) error {
	if err := os.MkdirAll(r.Out, 0o755); err != nil {
		return err
	}
	r.stage("building the window helper")
	var err error
	r.helper, err = buildMacwin(ctx, filepath.Join(r.Out, "bin"))
	if err != nil {
		return err
	}

	r.stage("preparing the game files")
	since := time.Now()
	game, err := PrepareGame(ctx, r.Scenario, r.CacheDir, WorkDir(r.Scenario))
	if err != nil {
		return err
	}
	r.mark("prepareGame", since)

	if err := r.helper.ParkCursor(ctx); err != nil {
		return err
	}
	r.stage("capturing the desktop before the game starts")
	desktop := filepath.Join(r.Out, "desktop-before.png")
	if err := screencapture(ctx, desktop); err != nil {
		return err
	}

	r.stage("installing the runtime")
	since = time.Now()
	if err := r.macron(ctx, "runtime-install.log", "runtime", "install"); err != nil {
		return err
	}
	r.mark("runtimeInstall", since)

	r.stage("starting the game through macron")
	window, err := r.launchAndWait(ctx, game)
	defer r.stopGame()
	if err != nil {
		return err
	}
	r.report.Window = &window

	r.stage("capturing the game window")
	frame, err := r.captureWindow(ctx, window)
	if err != nil {
		return err
	}

	r.stage("comparing with the reference")
	reference, err := ReadPNG(r.Scenario.ReferencePath())
	if err != nil {
		return fmt.Errorf("reading the reference: %w", err)
	}
	return r.judge(ctx, reference, frame, desktop)
}

func (r *macRun) macron(ctx context.Context, logName string, args ...string) error {
	log, err := os.Create(filepath.Join(r.Out, logName))
	if err != nil {
		return err
	}
	defer log.Close()
	command := exec.CommandContext(ctx, r.Macron, args...)
	command.Stdout = log
	command.Stderr = log
	if err := command.Run(); err != nil {
		return fmt.Errorf("macron %v failed (%s); see %s", args, err, logName)
	}
	return nil
}

func (r *macRun) launchAndWait(ctx context.Context, g Game) (MacWindow, error) {
	log, err := os.Create(filepath.Join(r.Out, "game.log"))
	if err != nil {
		return MacWindow{}, err
	}
	output := &lockedBuffer{}
	game := exec.Command(r.Macron, "launch", "--game", r.Scenario.Game, "--cwd", g.Cwd, "--", g.Exe)
	game.Stdout = io.MultiWriter(log, output)
	game.Stderr = game.Stdout
	launched := time.Now()
	if err := game.Start(); err != nil {
		log.Close()
		return MacWindow{}, fmt.Errorf("starting macron: %w", err)
	}
	exited := make(chan error, 1)
	go func() {
		exited <- game.Wait()
		log.Close()
	}()

	timeout := time.After(time.Duration(r.Scenario.Reached.TimeoutSeconds) * time.Second)
	pattern := r.Scenario.TitlePattern()
	var lastWindows []MacWindow
	sawWindow := false
	for {
		select {
		case err := <-exited:
			r.saveWindows(lastWindows)
			return MacWindow{}, fmt.Errorf("the game ended before reaching its menu (%v); see game.log", err)
		case <-timeout:
			r.saveWindows(lastWindows)
			game.Process.Kill()
			return MacWindow{}, fmt.Errorf("the game did not reach its menu within %ds (window seen: %t, output %q seen: %t)",
				r.Scenario.Reached.TimeoutSeconds, sawWindow, r.Scenario.Reached.Output, output.Contains(r.Scenario.Reached.Output))
		case <-ctx.Done():
			game.Process.Kill()
			return MacWindow{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		windows, err := r.helper.Windows(ctx)
		if err != nil {
			return MacWindow{}, err
		}
		lastWindows = windows
		window, found := findWindow(windows, pattern.MatchString)
		if !found {
			continue
		}
		if !sawWindow {
			sawWindow = true
			r.mark("firstWindow", launched)
		}
		if r.Scenario.Reached.Output != "" && !output.Contains(r.Scenario.Reached.Output) {
			continue
		}

		r.mark("reached", launched)
		time.Sleep(time.Duration(r.Scenario.Reached.SettleSeconds) * time.Second)
		r.saveWindows(windows)
		windows, err = r.helper.Windows(ctx)
		if err != nil {
			return MacWindow{}, err
		}
		if settled, found := findWindow(windows, pattern.MatchString); found {
			window = settled
		}
		return window, nil
	}
}

func findWindow(windows []MacWindow, matches func(string) bool) (MacWindow, bool) {
	for _, window := range windows {
		if window.Layer == 0 && matches(window.Title) {
			return window, true
		}
	}
	return MacWindow{}, false
}

func (r *macRun) saveWindows(windows []MacWindow) {
	content, err := json.MarshalIndent(windows, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(r.Out, "windows.json"), content, 0o644)
}

func (r *macRun) stopGame() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	exec.CommandContext(ctx, r.Macron, "kill", "--game", r.Scenario.Game).Run()
}

func (r *macRun) captureWindow(ctx context.Context, window MacWindow) (image.Image, error) {
	path := filepath.Join(r.Out, "window.png")
	output, err := exec.CommandContext(ctx, "screencapture", "-x", "-o", "-l", strconv.Itoa(window.ID), path).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("screencapture of window %d: %s: %s", window.ID, err, output)
	}
	if err := screencapture(ctx, filepath.Join(r.Out, "screen.png")); err != nil {
		return nil, err
	}
	captured, err := ReadPNG(path)
	if err != nil {
		return nil, err
	}

	area := ContentArea(captured.Bounds(), window.Width, r.Scenario.Window.Width, r.Scenario.Window.Height)
	scale := float64(captured.Bounds().Dx()) / window.Width
	expectedWidth := float64(r.Scenario.Window.Width) * scale
	if diff := float64(captured.Bounds().Dx()) - expectedWidth; diff > expectedWidth*0.02 || diff < -expectedWidth*0.02 {
		return nil, fmt.Errorf("the game window is %.0fx%.0f points, the scenario expects a %dx%d picture", window.Width, window.Height, r.Scenario.Window.Width, r.Scenario.Window.Height)
	}
	frame := Crop(captured, area)
	if err := WritePNG(filepath.Join(r.Out, "frame.png"), frame); err != nil {
		return nil, err
	}
	return frame, nil
}

func (r *macRun) judge(ctx context.Context, reference, frame image.Image, desktopPath string) error {
	controls := imagecheck.BrokenRenders(reference)
	desktop, err := ReadPNG(desktopPath)
	if err != nil {
		return err
	}
	controls = append(controls, imagecheck.Control{Name: "the desktop without the game", Image: desktop})
	if r.OtherGame != "" {
		other, err := ReadPNG(r.OtherGame)
		if err != nil {
			return fmt.Errorf("reading the other game's reference: %w", err)
		}
		controls = append(controls, imagecheck.Control{Name: "another game's menu", Image: other})
	} else {
		r.report.Notes = append(r.report.Notes, "no other game's menu to use as a control yet")
	}

	text := ""
	if len(r.Scenario.Compare.Text) > 0 {
		text, err = r.helper.Text(ctx, filepath.Join(r.Out, "frame.png"))
		if err != nil {
			return err
		}
		os.WriteFile(filepath.Join(r.Out, "frame-text.txt"), []byte(text), 0o644)
	}

	verdict := Evaluate(EvaluateInput{
		Reference:      reference,
		Frame:          frame,
		Ignore:         r.Scenario.Compare.Ignore,
		Controls:       controls,
		TextRequired:   r.Scenario.Compare.Text,
		RecognisedText: text,
	})
	r.report.Verdict = &verdict
	r.report.Passed = verdict.Passed
	r.report.Stage = "done"
	return nil
}

func screencapture(ctx context.Context, path string) error {
	output, err := exec.CommandContext(ctx, "screencapture", "-x", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("screencapture: %s: %s", err, output)
	}
	return nil
}

func describeMac(ctx context.Context) string {
	version, _ := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
	model, _ := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string").Output()
	return fmt.Sprintf("macOS %s on %s", bytes.TrimSpace(version), bytes.TrimSpace(model))
}
