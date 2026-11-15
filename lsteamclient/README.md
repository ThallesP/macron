# The Steam bridge (`lsteamclient`)

A Windows game finds Steam through `steam_api.dll` (or `steam_api64.dll`), which loads `steamclient.dll` (or `steamclient64.dll`) from the folder named in `HKCU\Software\Valve\Steam\ActiveProcess`. Under Macron that file is a copy of Proton's `lsteamclient.dll`, whose Unix side (`lsteamclient.so`) loads the Mac Steam client's own `steamclient.dylib` and forwards every Steamworks call to it. A 32-bit game uses the i386 `lsteamclient.dll` with the same 64-bit `lsteamclient.so` (Wine's WoW64 mode).

This folder is a port of Proton's `lsteamclient` (ValveSoftware/Proton, tag `proton-11.0-2e`, commit `bc896217ffcd67f892f952b010f898fe7bd0f0a1`) to macOS.

## Licence

Proton's `lsteamclient` is under the Steamworks SDK License, which is in `LICENSE` here (Valve's file, unchanged). This folder and everything built from it stay under that licence; Macron's own licence does not apply to it. As NotProton does, this repository keeps only Macron's changes (`patches/`) and the scripts that fetch Valve's source at build time; Valve's source and the Steamworks SDK headers it carries are never committed. Valve has not confirmed that redistributing builds of it is allowed: ValveSoftware/Proton issue #7801 asks and is open.

## Files

- `release.env`: the Proton commit, a SHA-256 digest of each folder taken from it, and the release asset's name.
- `fetch.sh <work-dir>`: downloads Proton at that commit from GitHub, checks `lsteamclient/` and `steam_helper/` against the digests (every file, sorted by path), checks that `LICENSE` here and `../steam_helper/LICENSE` are Valve's, and applies `patches/` and `../steam_helper/patches/`.
- `build.sh <work-dir> <out-dir>`: on a Mac, builds the bridge in Wine 11.21's tree with the toolchain and configure flags of `runtime/wine/build-overlay.sh`, checks it, and packs `lib/wine/...` and `share/doc/macron-steam-bridge/` (licences, patches, check report) into the release asset.
- `patches/0001-lsteamclient-Load-the-Mac-Steam-client.patch`: Macron's changes to Valve's files.

## What Macron changes

Valve's `unixlib.cpp` already has a macOS path; it loads `$STEAM_COMPAT_CLIENT_INSTALL_PATH/steamclient.dylib`. The patch:

- looks for `steamclient.dylib` there, then in `Steam.AppBundle/Steam/Contents/MacOS/` under that folder, then under `~/Library/Application Support/Steam`. The Mac client's own folder holds `steamapps`, and its `steamclient.dylib` (universal, x86_64 and arm64; Wine runs under Rosetta, so the x86_64 half is loaded) sits inside its app bundle;
- treats `Steam_IsKnownInterface` and `Steam_NotifyMissingInterface` as optional. The Mac client (build 1794536281) does not export them, and Valve's loader gives up without them. Without them every interface is taken as known;
- logs `dlerror()` when `steamclient.dylib` does not load;
- does not borrow memory from `steamclient.dll`'s `.data` section for the `SteamClient` interface and its vtables when `steamclient.dll` is this module itself. Proton loads Valve's Windows `steamclient.dll` and redirects its exports to `lsteamclient.dll` with a change to Wine's loader, so a few games (Mafia II) that check that the interface points into `steamclient.dll` are satisfied. Macron has no such loader change: upstream Wine 11.21 loads a renamed copy of a builtin DLL by its export name (`find_builtin_dll` with the image's `exp_name`), so the copy named `steamclient.dll` is `lsteamclient.dll`, and writing into its `.data` would overwrite its own variables. Those interfaces come from the heap instead;
- returns no native key code for Steam Input's keyboard glyphs on macOS, where Valve's code has only an X11 table;
- defines `NOMINMAX`, because upstream Wine's `windef.h` defines `max()` and Valve's `std::max` does not compile against it.
