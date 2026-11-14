package githubactions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

var ErrNotFound = errors.New("not found")

func GH(ctx context.Context, method, path string, body any) ([]byte, error) {
	command := exec.CommandContext(ctx, "gh", "api", "--method", method, "-H", "X-GitHub-Api-Version: 2026-03-10", path)
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		command.Args = append(command.Args, "--input", "-")
		command.Stdin = bytes.NewReader(payload)
	}

	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, sandbox.Errorf(sandbox.CodeMissingTool, "the GitHub CLI (gh) is not installed; sbx uses its login")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if strings.Contains(stderr.String(), "(HTTP 404)") {
		return nil, fmt.Errorf("GitHub %s %s: %w", method, path, ErrNotFound)
	}
	if err != nil {
		return nil, sandbox.Errorf(sandbox.CodeGitHubFailed, "GitHub %s %s: %s", method, path, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}
