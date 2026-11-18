package wineruntime

import (
	"context"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// HostCommand is a command for one of macOS's own tools (cp, tar, codesign).
// Steam starts macron with DYLD_INSERT_LIBRARIES naming its overlay loader,
// which is meant for the game. On a Mac with SIP off, dyld loads it into
// Apple's tools too, and they abort: the loader has no arm64e slice. So the
// tools run without any of dyld's variables.
func HostCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		return strings.HasPrefix(variable, "DYLD_")
	})
	return command
}
