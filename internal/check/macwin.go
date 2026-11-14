package check

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed macwin.swift
var macwinSource []byte

type MacWindow struct {
	ID     int     `json:"id"`
	PID    int     `json:"pid"`
	Owner  string  `json:"owner"`
	Title  string  `json:"title"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Layer  int     `json:"layer"`
}

type textLine struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

type macwin struct {
	path string
}

func buildMacwin(ctx context.Context, dir string) (macwin, error) {
	source := filepath.Join(dir, "macwin.swift")
	binary := filepath.Join(dir, "macwin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return macwin{}, err
	}
	if err := os.WriteFile(source, macwinSource, 0o644); err != nil {
		return macwin{}, err
	}
	output, err := exec.CommandContext(ctx, "xcrun", "swiftc", "-O", "-o", binary, source).CombinedOutput()
	if err != nil {
		return macwin{}, fmt.Errorf("compiling the window helper: %s: %s", err, output)
	}
	return macwin{path: binary}, nil
}

func (m macwin) Windows(ctx context.Context) ([]MacWindow, error) {
	output, err := exec.CommandContext(ctx, m.path, "windows").Output()
	if err != nil {
		return nil, fmt.Errorf("listing windows: %w", err)
	}
	var windows []MacWindow
	if err := json.Unmarshal(output, &windows); err != nil {
		return nil, fmt.Errorf("listing windows: %w", err)
	}
	return windows, nil
}

func (m macwin) Text(ctx context.Context, image string) (string, error) {
	output, err := exec.CommandContext(ctx, m.path, "ocr", image).Output()
	if err != nil {
		return "", fmt.Errorf("reading text: %w", err)
	}
	var lines []textLine
	if err := json.Unmarshal(output, &lines); err != nil {
		return "", fmt.Errorf("reading text: %w", err)
	}
	texts := make([]string, len(lines))
	for i, line := range lines {
		texts[i] = line.Text
	}
	return strings.Join(texts, "\n"), nil
}
