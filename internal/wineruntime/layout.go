package wineruntime

import (
	"os"
	"path/filepath"
	"runtime"
)

type Layout struct {
	Home string
}

func DefaultLayout() (Layout, error) {
	if home := os.Getenv("MACRON_HOME"); home != "" {
		return Layout{Home: home}, nil
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return Layout{}, err
	}
	if runtime.GOOS == "darwin" {
		return Layout{Home: filepath.Join(user, "Library", "Application Support", "Macron")}, nil
	}
	return Layout{Home: filepath.Join(user, ".local", "share", "macron")}, nil
}

func (l Layout) RuntimeDir(version string) string {
	return filepath.Join(l.Home, "runtimes", version)
}

func (l Layout) DownloadsDir() string {
	return filepath.Join(l.Home, "downloads")
}

func (l Layout) PrefixDir(gameID string) string {
	return filepath.Join(l.Home, "prefixes", gameID)
}

func (l Layout) LogsDir() string {
	return filepath.Join(l.Home, "logs")
}
