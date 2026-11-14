package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ThallesP/macron/fixes"
)

func steamVerb(waitForWine bool) command {
	return func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
		if len(args) == 0 {
			fmt.Fprint(stderr, "usage: macron waitforexitandrun|run <exe> [args]; Steam calls this\n")
			return exitUsage
		}
		appID, err := strconv.Atoi(firstEnv("SteamAppId", "STEAM_COMPAT_APP_ID"))
		if err != nil {
			fmt.Fprint(stderr, "macron: Steam did not say which app this is (SteamAppId is not set)\n")
			return exitUsage
		}

		db, err := fixes.Load()
		if err != nil {
			return fail(stderr, err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fail(stderr, err)
		}
		in := gameStart{Profile: db.ForSteamApp(appID), Exe: args[0], Cwd: cwd, Args: args[1:], WaitForWine: waitForWine}
		if compatData := os.Getenv("STEAM_COMPAT_DATA_PATH"); compatData != "" {
			in.Prefix = filepath.Join(compatData, "pfx")
		}
		return startGame(ctx, in, stderr)
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
