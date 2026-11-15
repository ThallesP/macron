package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/wineruntime"
)

var version = "dev"

const diagnoseUsage = "usage: macron diagnose [--tools <dir>]\n"

const installLog = "/Library/Logs/Macron-install.log"

var (
	steamAccountID = regexp.MustCompile(`\[U:1:\d+\]`)
	steamID64      = regexp.MustCompile(`7656119\d{10}`)
	frameRateLine  = regexp.MustCompile(`@ approx ([0-9.]+)fps`)
	startLine      = regexp.MustCompile(`starting (.+) with the (\w+) backend`)
)

func diagnoseCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	tools := flags.String("tools", defaultDiagnoseTools(), "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprint(stderr, diagnoseUsage)
		return exitUsage
	}
	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fail(stderr, err)
	}

	var report strings.Builder
	fmt.Fprintf(&report, "Macron diagnose report, %s\nmacron %s\n", time.Now().UTC().Format(time.RFC3339), version)
	progress := progressTo(stderr)
	progress("looking at this Mac, Steam and the runtime")
	writeSystem(&report)
	writeSteam(&report, layout)
	writeRuntime(&report, layout)
	progress("timing what a game pays to call into Wine and macOS (about two minutes)")
	writeTransitions(ctx, &report, *tools)
	writeGames(&report, layout)

	text := scrubber(home)(report.String())
	path := filepath.Join(layout.LogsDir(), "diagnose-"+time.Now().UTC().Format("20060102T150405Z")+".txt")
	if err := os.MkdirAll(layout.LogsDir(), 0o755); err != nil {
		return fail(stderr, err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprint(stdout, text)
	progress(fmt.Sprintf("the report is in %s; nothing was sent anywhere", scrubber(home)(path)))
	return 0
}

func defaultDiagnoseTools() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "..", "lib", "macron", "diagnose")
}

func scrubber(home string) func(string) string {
	names := []string{home}
	if current, err := user.Current(); err == nil {
		names = append(names, current.Username)
	}
	names = append(names, filepath.Base(home))
	return func(text string) string {
		text = strings.ReplaceAll(text, home, "~")
		for _, name := range names[1:] {
			if len(name) >= 3 {
				text = strings.ReplaceAll(text, name, "<user>")
			}
		}
		text = steamAccountID.ReplaceAllString(text, "[U:1:<id>]")
		return steamID64.ReplaceAllString(text, "<steam id>")
	}
}

func writeSystem(report io.Writer) {
	fmt.Fprintln(report, "\n== This Mac")
	fmt.Fprintf(report, "macOS:            %s (%s)\n", commandOutput("sw_vers", "-productVersion"), commandOutput("sw_vers", "-buildVersion"))
	fmt.Fprintf(report, "chip:             %s, model %s, %s cores, %s bytes of memory\n", commandOutput("sysctl", "-n", "machdep.cpu.brand_string"), commandOutput("sysctl", "-n", "hw.model"), commandOutput("sysctl", "-n", "hw.ncpu"), commandOutput("sysctl", "-n", "hw.memsize"))
	fmt.Fprintf(report, "virtual machine:  %s\n", map[string]string{"1": "yes", "0": "no"}[commandOutput("sysctl", "-n", "kern.hv_vmm_present")])
	fmt.Fprintf(report, "SIP:              %s\n", commandOutput("csrutil", "status"))
	fmt.Fprintf(report, "Rosetta:          %s\n", map[bool]string{true: "installed", false: "not installed"}[exec.Command("arch", "-x86_64", "/usr/bin/true").Run() == nil])
}

func writeSteam(report io.Writer, layout wineruntime.Layout) {
	fmt.Fprintln(report, "\n== Steam")
	hook, err := newHook(defaultSteamDir(), defaultSteamApp)
	if err != nil {
		fmt.Fprintf(report, "status: %s\n", err)
	} else if status, err := hook.Status(); err != nil {
		fmt.Fprintf(report, "status: %s\n", err)
	} else {
		fmt.Fprintf(report, "build:            %s (known to this macron: %t)\n", status.Build.ID, status.Build.IsKnown)
		fmt.Fprintf(report, "steamclient:      sha256 %s\n", status.Build.SHA256["steamclient.dylib"])
		fmt.Fprintf(report, "Steam.app:        %s\n", status.SteamApp)
		fmt.Fprintf(report, "tool registered:  %t, default tool: %t\n", status.IsToolRegistered, status.IsDefaultTool)
		fmt.Fprintf(report, "steam play:       %s\n", map[bool]string{true: "on", false: "off"}[status.IsSwitchOn])
	}
	writeTail(report, "steamplay.log, last lines", filepath.Join(layout.LogsDir(), "steamplay.log"), 15)
	writeTail(report, installLog+", last lines", installLog, 25)
}

func writeTail(report io.Writer, title string, path string, lines int) {
	fmt.Fprintf(report, "-- %s\n", title)
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(report, "(not there: %s)\n", err)
		return
	}
	all := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	fmt.Fprintln(report, strings.Join(all[max(0, len(all)-lines):], "\n"))
}

func writeRuntime(report io.Writer, layout wineruntime.Layout) {
	fmt.Fprintln(report, "\n== Runtime")
	fmt.Fprintf(report, "pinned:           %s\n", wineruntime.Pinned.Version)
	fmt.Fprintf(report, "steam bridge:     %s\n", wineruntime.Pinned.SteamBridge.Name)
	dir := layout.RuntimeDir(wineruntime.Pinned.Version)
	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintln(report, "installed:        no (macron runtime install)")
		return
	}
	_, bridgeErr := os.Stat(filepath.Join(dir, "wine-dxmt", "lib", "wine", "x86_64-unix", "lsteamclient.so"))
	fmt.Fprintf(report, "installed:        yes, bridge in it: %t\n", bridgeErr == nil)
	_, showFPSErr := os.Stat(layout.ShowFPSFile())
	fmt.Fprintf(report, "frame counter:    %s\n", map[bool]string{true: "on (show-fps exists)", false: "off"}[showFPSErr == nil])
}

func writeTransitions(ctx context.Context, report io.Writer, tools string) {
	fmt.Fprintln(report, "\n== What a call costs (ns each, best of three)")
	native := filepath.Join(tools, "transition-cost")
	if _, err := os.Stat(native); err != nil {
		fmt.Fprintf(report, "(no measuring programs at %s)\n", tools)
		return
	}
	for _, arch := range []string{"arm64", "x86_64"} {
		fmt.Fprintf(report, "-- macOS, %s\n", arch)
		fmt.Fprintln(report, resultLines(commandOutput("arch", "-"+arch, native)))
	}
	for _, exe := range []string{"cost64.exe", "cost32.exe"} {
		fmt.Fprintf(report, "-- Windows program under Wine, %s\n", exe)
		var output bytes.Buffer
		exitCode := startGame(ctx, gameStart{Profile: fixes.Default("diagnose"), Exe: filepath.Join(tools, exe), Cwd: tools, WaitForWine: true}, &output)
		fmt.Fprintln(report, resultLines(output.String()))
		if exitCode != 0 {
			fmt.Fprintf(report, "(exit %d)\n", exitCode)
		}
	}
}

func resultLines(output string) string {
	var results []string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "RESULT ") {
			results = append(results, strings.TrimPrefix(line, "RESULT "))
		}
	}
	if len(results) == 0 {
		return "(no result) " + strings.TrimSpace(output[max(0, len(output)-300):])
	}
	return strings.Join(results, "\n")
}

type frameRateLog struct {
	Name      string
	Exe       string
	Backend   string
	HasBridge bool
	Rates     []float64
}

func readFrameRateLog(name string, text string) (frameRateLog, bool) {
	if !strings.Contains(text, "logging the frame rate") {
		return frameRateLog{}, false
	}
	log := frameRateLog{Name: name, HasBridge: strings.Contains(text, "starting it through steam.exe")}
	if match := startLine.FindStringSubmatch(text); match != nil {
		log.Exe, log.Backend = match[1], match[2]
	}
	for _, match := range frameRateLine.FindAllStringSubmatch(text, -1) {
		rate, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			log.Rates = append(log.Rates, rate)
		}
	}
	return log, true
}

func (l frameRateLog) summary() string {
	head := fmt.Sprintf("%s: %s, %s backend, Steam bridge %t", l.Name, l.Exe, l.Backend, l.HasBridge)
	if len(l.Rates) == 0 {
		return head + "\n   no frame counter lines (Direct3D 10 and 11 games show Apple's Metal HUD instead of logging)"
	}
	sorted := slices.Clone(l.Rates)
	sort.Float64s(sorted)
	last := l.Rates[max(0, len(l.Rates)-10):]
	return fmt.Sprintf("%s\n   %d samples, median %.1f fps, lowest %.1f, highest %.1f; last ones: %s", head, len(sorted), sorted[len(sorted)/2], sorted[0], sorted[len(sorted)-1], strings.Trim(fmt.Sprint(last), "[]"))
}

func writeGames(report io.Writer, layout wineruntime.Layout) {
	fmt.Fprintln(report, "\n== Games started with the frame counter on (three latest starts each)")
	games, err := os.ReadDir(layout.LogsDir())
	if err != nil {
		fmt.Fprintf(report, "(no logs: %s)\n", err)
		return
	}
	found := 0
	for _, game := range games {
		if !game.IsDir() || game.Name() == "diagnose" {
			continue
		}
		logs, _ := filepath.Glob(filepath.Join(layout.LogsDir(), game.Name(), "*.log"))
		sort.Strings(logs)
		var summaries []string
		for _, path := range logs {
			text, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if log, isFrameRateLog := readFrameRateLog(game.Name()+"/"+filepath.Base(path), string(text)); isFrameRateLog {
				summaries = append(summaries, log.summary())
			}
		}
		for _, summary := range summaries[max(0, len(summaries)-3):] {
			fmt.Fprintln(report, summary)
			found++
		}
	}
	if found == 0 {
		fmt.Fprintln(report, "(none yet: touch ~/Library/Application\\ Support/Macron/show-fps, then start a game from Steam)")
	}
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		return fmt.Sprintf("%s (%s)", text, err)
	}
	return text
}
