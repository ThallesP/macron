package wineruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ThallesP/macron/internal/failure"
)

const steamBridgeUnixLib = "lib/wine/x86_64-unix/lsteamclient.so"

func addSteamBridge(ctx context.Context, layout Layout, bridge Archive, staging string, progress func(string)) error {
	if bridge.URL == "" {
		return nil
	}
	progress(fmt.Sprintf("downloading %s", bridge.Name))
	archive, err := Download(ctx, layout.DownloadsDir(), bridge)
	if err != nil {
		return err
	}
	progress("adding the Steam bridge")
	unpacked := filepath.Join(staging, "unpacked-steam-bridge")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(unpacked, steamBridgeUnixLib)); err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "%s has no %s", bridge.Name, steamBridgeUnixLib)
	}
	for _, tree := range []Tree{TreePlain, TreeDXMT} {
		if err := copyTree(ctx, unpacked, filepath.Join(staging, string(tree))); err != nil {
			return err
		}
	}
	return os.RemoveAll(unpacked)
}
