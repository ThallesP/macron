package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ThallesP/macron/internal/scenario"
)

const remoteRoot = "macron-check"

type sandboxRun struct {
	scenario scenario.Scenario
	repo     string
	id       string
	stderr   io.Writer
	progress func(string)
}

func sandboxCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var keep bool
	flags, ok := parse("sandbox", args, stderr, func(f *flag.FlagSet) {
		f.BoolVar(&keep, "keep", false, "leave the sandbox running afterwards")
	})
	if !ok || flags.NArg() != 1 {
		fmt.Fprint(stderr, "usage: macron-check sandbox <scenario> [--keep]\n")
		return exitUsage
	}
	repo, err := repoRoot()
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: run this inside the macron repository: %s\n", err)
		return exitBroken
	}
	s, err := scenario.Load(filepath.Join(repo, "scenarios", flags.Arg(0)))
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}

	run := &sandboxRun{scenario: s, repo: repo, stderr: stderr, progress: progressTo(stderr)}
	exitCode, err := run.do(ctx, keep)
	if err != nil {
		fmt.Fprintf(stderr, "macron-check: %s\n", err)
		return exitBroken
	}
	return exitCode
}

func (r *sandboxRun) do(ctx context.Context, keep bool) (int, error) {
	work, err := os.MkdirTemp("", "macron-check-sandbox-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(work)

	r.progress("building the bundle")
	bundle := filepath.Join(work, "bundle")
	if output, err := exec.CommandContext(ctx, filepath.Join(r.repo, "ci", "build-bundle.sh"), bundle, "darwin").CombinedOutput(); err != nil {
		return 0, fmt.Errorf("building the bundle: %s: %s", err, output)
	}
	archive := filepath.Join(work, "bundle.tgz")
	if output, err := exec.CommandContext(ctx, "tar", "-czf", archive, "-C", bundle, ".").CombinedOutput(); err != nil {
		return 0, fmt.Errorf("packing the bundle: %s: %s", err, output)
	}

	r.progress("starting a sandbox")
	ttl := time.Duration(r.scenario.Reached.TimeoutSeconds)*time.Second + 90*time.Minute
	r.id, err = startSandbox(ctx, ttl)
	if err != nil {
		return 0, err
	}
	r.progress(fmt.Sprintf("sandbox %s is ready", r.id))
	if !keep {
		defer func() {
			r.progress(fmt.Sprintf("stopping sandbox %s", r.id))
			exec.Command("sbx", "stop", r.id).Run()
		}()
	}

	if err := r.sbx(ctx, "scp", archive, r.id+":/tmp/bundle.tgz"); err != nil {
		return 0, err
	}
	start := fmt.Sprintf(`rm -rf ~/%[1]s && mkdir -p ~/%[1]s/bundle && tar -xzf /tmp/bundle.tgz -C ~/%[1]s/bundle && cd ~/%[1]s && `+
		`(nohup sh -c './bundle/bin/macron-check run bundle/scenarios/%[2]s --out evidence > check.log 2>&1; echo $? > check.exit' > /dev/null 2>&1 &)`,
		remoteRoot, r.scenario.Name)
	if err := r.sbx(ctx, "exec", r.id, start); err != nil {
		return 0, err
	}

	exitCode, err := r.waitForCheck(ctx)
	if err != nil {
		return 0, err
	}
	local := filepath.Join(r.repo, "evidence", fmt.Sprintf("%s-%s", r.scenario.Name, time.Now().UTC().Format("20060102T150405Z")))
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return 0, err
	}
	r.sbx(ctx, "scp", "-r", r.id+":"+remoteRoot+"/evidence", local)
	r.sbx(ctx, "scp", r.id+":"+remoteRoot+"/check.log", filepath.Join(local, "check.log"))
	r.progress(fmt.Sprintf("evidence is in %s", local))
	if summary, err := os.ReadFile(filepath.Join(local, "summary.md")); err == nil {
		fmt.Fprint(r.stderr, string(summary))
	}
	return exitCode, nil
}

func (r *sandboxRun) waitForCheck(ctx context.Context) (int, error) {
	deadline := time.Now().Add(time.Duration(r.scenario.Reached.TimeoutSeconds)*time.Second + 60*time.Minute)
	lastLine := ""
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(20 * time.Second):
		}
		output, err := exec.CommandContext(ctx, "sbx", "exec", r.id,
			fmt.Sprintf("cat ~/%[1]s/check.exit 2>/dev/null || echo running; tail -1 ~/%[1]s/check.log 2>/dev/null", remoteRoot)).Output()
		if err != nil {
			continue
		}
		lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)
		if len(lines) == 2 && lines[1] != lastLine {
			lastLine = lines[1]
			fmt.Fprintf(r.stderr, "  %s\n", lastLine)
		}
		if lines[0] == "running" {
			continue
		}
		exitCode, err := strconv.Atoi(lines[0])
		if err != nil {
			return 0, fmt.Errorf("the check wrote a strange exit code %q", lines[0])
		}
		return exitCode, nil
	}
	return 0, fmt.Errorf("the check did not finish in time")
}

func (r *sandboxRun) sbx(ctx context.Context, args ...string) error {
	var output bytes.Buffer
	command := exec.CommandContext(ctx, "sbx", args...)
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return fmt.Errorf("sbx %s: %s: %s", args[0], err, output.String())
	}
	return nil
}

func startSandbox(ctx context.Context, ttl time.Duration) (string, error) {
	command := exec.CommandContext(ctx, "sbx", "new", "--json", "--ttl", ttl.Round(time.Minute).String())
	var errors bytes.Buffer
	command.Stderr = &errors
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("sbx new: %s: %s", err, errors.String())
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var event struct {
			Event string `json:"event"`
			ID    string `json:"id"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Event == "ready" {
			return event.ID, nil
		}
	}
	return "", fmt.Errorf("sbx new never said the sandbox was ready: %s", output)
}

func repoRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
