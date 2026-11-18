package wineruntime

import (
	"context"
	"runtime"

	"github.com/ThallesP/macron/internal/failure"
)

// CheckRosetta fails on an Apple silicon Mac without Rosetta 2. The Wine
// builds are x86_64 only, and macOS does not install Rosetta by itself: a new
// Mac has none until some other Intel program asked for it.
func CheckRosetta(ctx context.Context) error {
	if runtime.GOARCH != "arm64" || HostCommand(ctx, "arch", "-x86_64", "/usr/bin/true").Run() == nil {
		return nil
	}
	return failure.Errorf(failure.CodeRosettaMissing, "Rosetta 2 is not installed, and Wine needs it on an Apple silicon Mac. Install it with: softwareupdate --install-rosetta --agree-to-license")
}
