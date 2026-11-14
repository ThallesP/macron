package main

import (
	"context"
	"fmt"
	"io"

	"github.com/ThallesP/macron/internal/wineruntime"
)

func runtimeCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "install" && args[0] != "path") {
		fmt.Fprint(stderr, "usage: macron runtime install|path\n")
		return exitUsage
	}
	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}

	if args[0] == "path" {
		installed, err := wineruntime.Find(layout, wineruntime.Pinned)
		if err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, installed.Dir)
		return 0
	}
	installed, err := wineruntime.Install(ctx, layout, wineruntime.Pinned, progressTo(stderr))
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "runtime %s is at %s\n", installed.Version, installed.Dir)
	return 0
}
