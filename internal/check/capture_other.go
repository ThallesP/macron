//go:build !windows

package check

import (
	"errors"
	"image"
)

var errWindowsOnly = errors.New("references are captured on Windows")

func listWindowsWindows() ([]windowsWindow, error) {
	return nil, errWindowsOnly
}

func captureClientArea(uintptr) (image.Image, error) {
	return nil, errWindowsOnly
}
