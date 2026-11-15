# The Steam bridge (gate 4), build side

Branch `steam-bridge`, 2026-11-15, Opus 5.5. This is the section for PROGRESS.md; it is kept here so that two threads do not edit PROGRESS.md at once.

## What was built

A Windows game reaches the Mac Steam client this way, as on Linux with Proton:

1. Steam runs `macron waitforexitandrun <game.exe>`. macron sees that Steam started it and, if the runtime has the bridge, puts it into the game's prefix (`internal/launch/steambridge.go`):
   - `lib/wine/i386-windows/lsteamclient.dll` → `C:\Program Files (x86)\Steam\steamclient.dll`
   - `lib/wine/x86_64-windows/lsteamclient.dll` → `C:\Program Files (x86)\Steam\steamclient64.dll`
   - `lib/wine/x86_64-windows/steam.exe` → `C:\Program Files (x86)\Steam\steam.exe` and `C:\windows\system32\steam.exe`
   - the registry values Proton's prefix has (from Proton's `wine.inf`): `HKCU\Software\Valve\Steam\ActiveProcess` `SteamClientDll`, `SteamClientDll64`, `SteamPath`, `Universe`; `HKCU\Software\Valve\Steam` `SteamPath`, `SteamExe`; `InstallPath` under `HKLM\Software\Valve\Steam` and its `Wow6432Node` twin. Imported once per prefix with regedit (marker `.macron-steam-bridge.json`).
   - `SteamGameId` and `SteamAppId` (from whichever of `SteamGameId`, `SteamAppId`, `STEAM_COMPAT_APP_ID` is set) and `STEAM_COMPAT_CLIENT_INSTALL_PATH` (default `~/Library/Application Support/Steam`) when Steam did not set them.
2. The game starts as `wine C:\windows\system32\steam.exe <game.exe> <args>`. `steam.exe` is Proton 11's `steam_helper` (`steam_helper/`): it writes `ActiveProcess\pid` (its own pid, so `SteamAPI_IsSteamRunning()` is true while the game runs), calls the bridge's `steamclient_init_registry` (language and app keys, when Steam answers), writes `steamapps\libraryfolders.vdf`, starts the game and returns its exit code.
3. The game's `steam_api(64).dll` loads `steamclient(64).dll` from the path in the registry. That file is a copy of a Wine builtin, so upstream Wine 11.21 loads the builtin it was built as, by its export name (`find_builtin_dll` with the image's `exp_name`): `lsteamclient.dll` from the runtime, and its Unix side `lsteamclient.so`. A 32-bit game gets the i386 DLL with the same 64-bit `.so`, through Wine's WoW64 thunks.
4. `lsteamclient.so` loads `steamclient.dylib`: `$STEAM_COMPAT_CLIENT_INSTALL_PATH/steamclient.dylib`, else `$STEAM_COMPAT_CLIENT_INSTALL_PATH/Steam.AppBundle/Steam/Contents/MacOS/steamclient.dylib`, else the same under `~/Library/Application Support/Steam`. The x86_64 half of the universal dylib is loaded (Wine runs under Rosetta).

The pieces:

- `lsteamclient/` (Steamworks SDK License, Valve's `LICENSE`): `release.env` (Proton tag `proton-11.0-2e`, commit `bc896217ffcd67f892f952b010f898fe7bd0f0a1`, a SHA-256 digest of each folder taken from it), `fetch.sh` (downloads and checks Valve's `lsteamclient/` and `steam_helper/`, compares the committed licences with Valve's, applies Macron's patches), `build.sh` (builds on a Mac inside Wine 11.21's tree with the overlay's llvm-mingw and configure flags, checks the result, packs the release asset), `patches/0001-lsteamclient-Load-the-Mac-Steam-client.patch` (what it changes and why: `lsteamclient/README.md`). No file of Valve's is committed.
- `steam_helper/` (BSD 3-Clause, Proton's `LICENSE.proton`): `patches/0001-steam_helper-Forward-to-the-Mac-Steam-client.patch` (`steam.exe steam://...` goes to `/usr/bin/open`; no SteamVR setup).
- `.github/workflows/steam-bridge.yml` (`Steam bridge build`): manual; builds on macos-26, uploads the artifact, and only on `main` publishes the release `steam-bridge-proton-11.0-2e-macron.1` with `macron-steam-bridge-proton-11.0-2e-macron.1-osx64.tar.xz`. Not run: a new workflow can only be dispatched once it is on the default branch.
- `internal/wineruntime`: `Manifest.SteamBridge`. When it names an archive, `Install` downloads it (pinned SHA-256), checks that it has `lib/wine/x86_64-unix/lsteamclient.so`, and copies it into both Wine trees (`wine` and `wine-dxmt`). `Pinned.SteamBridge` is empty because no release exists yet; with it empty nothing is downloaded and installed runtimes still match.
- `internal/launch/steambridge.go`, `Input.SteamBridge`, set by `macron waitforexitandrun` and `run` only (not `macron launch`, so the scenarios are unchanged). A runtime without the bridge logs `this runtime has no Steam bridge, so the game cannot reach Steam` and starts the game directly, as before.
- `tools/steam-api-probe/`: `probe.c`, built 32-bit and 64-bit with the overlay's llvm-mingw by `build.sh`, which also fetches Valve's redistributable `steam_api.dll` and `steam_api64.dll` (Steamworks SDK 1.65, from Steamworks.NET at commit `ea51b029`, SHA-256 checked, Valve-signed). It calls `SteamAPI_InitFlat`, then `SteamUtils`'s app id, `SteamAPI_SteamUserStats_v013` and `SteamAPI_SteamRemoteStorage_v016`, and writes `RESULT` lines to stdout and to `steam-api-probe-<bits>.txt` beside itself (`steam.exe` starts console programs in a new console, so their stdout does not reach macron's log).

### One archive or two

The bridge is its own release asset, not part of the Wine overlay. The overlay archive carries `share/doc/macron-wine/COPYING.LIB` and its release notes say the whole asset is LGPL 2.1+; `lsteamclient` is under the Steamworks SDK License, so putting it there would make that statement false, and README.md's licence table is meant to say which folder stays under Valve's licence. It also lets the bridge follow Valve's Proton releases without rebuilding Wine. Both are built from the same pinned Wine source and llvm-mingw (`runtime/wine/release.env`), and the runtime puts both into both trees.

## What was proven, where

Two sandboxes, macOS 26.7.2, runtime `wine-11.21-macron.5` (installed by `macron runtime install`), the bridge unpacked over both Wine trees by hand (`tar -xJf ... -C <tree>`), probes run as Steam runs games: `SteamAppId=480 WINEDEBUG=+steamclient,+steam,+loaddll macron waitforexitandrun probeNN.exe`.

- **Build** (sbx_m7fkmdqs): `lsteamclient/fetch.sh` matched both digests (`lsteamclient`: 3133 files, `0f0974a9...`; `steam_helper`: 2 files, `333b6c08...`). `build.sh` built and checked:
  - `i386-windows/lsteamclient.dll`: coff-i386, 27,774,976 bytes, exports `CreateInterface`, `Steam_BGetCallback`, `steamclient_init_registry`
  - `x86_64-windows/lsteamclient.dll`: coff-x86-64, 29,536,256 bytes, same exports
  - `x86_64-windows/steam.exe`: coff-x86-64, 180,224 bytes
  - `x86_64-unix/lsteamclient.so`: x86_64, 2,388,512 bytes; its 11 imports from `ntdll.so` are all exported by Gcenx's `ntdll.so`; links `@rpath/ntdll.so`, `libc++.1.dylib`, `libSystem.B.dylib`
  - **The clean rebuild.** The fixes below were found during a first run on the same sandbox. Then the committed scripts (commit `4e66041`; the rebase onto main after it changed only README.md) ran from an empty work folder: `lsteamclient/build.sh /tmp/bw /tmp/bout` exited 0 with the same checks (the `.so` is 2,388,752 bytes this time), archive SHA-256 `fefa490810c424ef0011ec49cd9d99568bb503d6b79276497f6244a793fd39c2`, holding the four binaries under `lib/wine/` and `LICENSE.lsteamclient`, `LICENSE.steam_helper`, `COPYING.LIB.wine`, both patches, `README.md`, `release.env` and `bridge-check.txt` under `share/doc/macron-steam-bridge/`. The no-Steam probe runs below were repeated with that archive and gave the same lines.
- **No Steam installed** (sbx_m7fkmdqs), 64-bit and 32-bit alike, exit 1, no crash, no dialog:
  ```
  RESULT registry SteamClientDll64: C:\Program Files (x86)\Steam\steamclient64.dll
  RESULT load steam_api64.dll: OK
  RESULT SteamAPI_InitFlat: 1 (FailedGeneric), message "No SteamClient023"
  RESULT module steamclient64.dll: loaded from C:\Program Files (x86)\Steam\steamclient64.dll
  trace:loaddll:build_module Loaded L"C:\\Program Files (x86)\\Steam\\steamclient64.dll" at 00006FFFFC830000: builtin
  err:steamclient:steamclient_init unable to load native steamclient library "steamclient.dylib": dlopen(steamclient.dylib, 0x0002): tried: ... (no such file)
  ```
  The 32-bit run: `Loaded L"C:\\Program Files (x86)\\Steam\\steamclient.dll" at 794E0000: builtin`, same result.
- **Steam installed and running at its login window, nobody signed in** (sbx_4uhkif7q, Steam from Valve's DMG in /Applications, `steamui_login.txt`: `SetLoginState: WaitingForCredentials`), 64-bit and 32-bit:
  ```
  trace:steamclient:steamclient_init Loaded host steamclient from "/Users/runner/Library/Application Support/Steam/Steam.AppBundle/Steam/Contents/MacOS/steamclient.dylib"
  trace:steamclient:winISteamClient_SteamClient023_CreateSteamPipe
  trace:steamclient:winISteamClient_SteamClient023_ConnectToGlobalUser
  RESULT SteamAPI_InitFlat: 1 (FailedGeneric), message "ConnectToGlobalUser failed."
  ```
  So the bridge finds and opens the Mac client's `steamclient.dylib`, gets `SteamClient017` and `SteamClient023` from it, and gets as far as the user. The 32-bit run also shows Macron's change in use: `get_mem_from_steamclient_dll steamclient.dll is lsteamclient, not allocating from it.`
- **Steam installed but not running** (sbx_4uhkif7q, Steam quit; only Valve's `ipcserver` helper left), 64-bit and 32-bit:
  ```
  trace:steamclient:steamclient_init Loaded host steamclient from ".../Steam.AppBundle/Steam/Contents/MacOS/steamclient.dylib"
  RESULT SteamAPI_InitFlat: 2 (NoSteamClient), message "Cannot create IPC pipe to Steam client process.  Steam is probably not running."
  ```
- In every run `steam.exe` loaded `lsteamclient.dll` for `steamclient_init_registry`, which logged `Failed to connect to Steam` (or the missing dylib) and returned; the game then started and its exit code came back through macron.
- Go: `go test ./...`, `go vet ./...`, `gofmt -l .` (empty) and staticcheck pass, with new tests for the bridge in the prefix, the registry import, the environment, a runtime without the bridge, games not started by Steam, and `Install` with and without a pinned bridge.

## Found on the sandbox and fixed

- **Upstream Wine links Unix libraries with the C driver**, so `lsteamclient.so` did not link (`operator new`, `___gxx_personality_v0`); the patch adds `UNIX_LIBS = -lc++`.
- **Valve's `STEAMCLIENT_CALL` asserts on any failure.** With no Steam installed, `steam.exe` stopped on `_wassert (L"!status", L"../src/dlls/lsteamclient/steamclient_main.c", 384)`, a message box in a program without a console, and the game never started. `load_steamclient()` now returns failure instead, and `steamclient_init_registry` returns early.
- **A missing `system32` exe is only treated as a builtin while Wine bootstraps a prefix** (`get_pe_file_info`, `is_prefix_bootstrap`), so an existing prefix needs a real `C:\windows\system32\steam.exe`; macron copies it.
- **`build.sh`'s import check first matched nothing** (`nm` names the library `@rpath/ntdll.so`, not `ntdll`) and passed without checking; it now fails when the list is empty.

## Tried and dropped, or not needed

- **NotProton's loader change.** NotProton (on CrossOver) stages Valve's real Windows `steamclient.dll`/`steamclient64.dll` (from the Linux client's `legacycompat`, plus `tier0_s64.dll`, `vstdlib_s64.dll`) and patches ntdll so their exports jump to `lsteamclient.dll`, as Proton's Wine does. Upstream Wine 11.21 needs no patch for the simpler arrangement Macron uses (the copy is the builtin), and no Valve Windows binary has to be downloaded. The cost: games that check that the `SteamClient` interface points into Valve's `steamclient.dll` (Proton's comment names Mafia II) get heap memory instead. If one of them matters, the next step is NotProton's arrangement as a Wine patch in `runtime/wine/patches/`.
- **NotProton's `serialized005.patch`** (a `SteamNetworkingSocketsSerialized005` interface for Steam's own networking DLL) and its overlay hooks: not taken; no game path that `steam_api` uses asked for them on the sandbox.
- **NotProton's `alloc_vtable` change for 64-bit**: only needed when Valve's `steamclient64.dll` is the module; Valve's code looks for `steamclient.dll` only.
- **Proton 9's C++ `steam.cpp`** (what NotProton ported; it needs jsoncpp, OpenVR and the Steamworks headers): Proton 11's `steam.c` is plain C with no Steamworks header, so `steam_helper/` stays BSD-only.
- **Steam Input's native key codes**: Valve has only an X11 table; macOS gets 0 (no keyboard glyph), as in NotProton.

## What the signed-in check still has to show

Not done here (no signed-in Steam on this branch's sandboxes, by the brief): `SteamAPI_Init` returning OK inside a game started by Steam's Play button, an achievement unlocking, a Steam Cloud save going up and coming back, and nothing breaking on Steam's overlay. Also unknown: what the Mac client puts in `STEAM_COMPAT_CLIENT_INSTALL_PATH` when it starts a compatibility tool (the bridge accepts the dylib's own folder or Steam's folder), and whether `steam.exe steam://...` reaches the Mac client through `open`.

Commands, on a sandbox with the Steam Play switch and Steam signed in (gate 2's setup):

```sh
# 1. Build the bridge (about 25 minutes on a sandbox), or download the Steam bridge build workflow's artifact once this branch is on main.
lsteamclient/build.sh /tmp/bw /tmp/bout
# 2. Put it into both Wine trees of the installed runtime (until Manifest.SteamBridge pins a release).
R=$(macron runtime path); for t in wine wine-dxmt; do tar -xJf /tmp/bout/macron-steam-bridge-*.tar.xz -C "$R/$t"; done
# 3. The probe with the signed-in client (Spacewar, app 480, works for any account): expect "SteamAPI_InitFlat: 0 (OK)", "app id: 480",
#    and both interfaces "returned an interface", in 64-bit and 32-bit.
tools/steam-api-probe/build.sh /tmp/bw/llvm-mingw/bin /tmp/probe
cd /tmp/probe && for b in 64 32; do SteamAppId=480 WINEDEBUG=+steamclient macron waitforexitandrun probe$b.exe; cat steam-api-probe-$b.txt; done
# 4. Then Play on a free game of the account that uses achievements and Steam Cloud, from Steam's own button, and read
#    ~/Library/Application Support/Macron/logs/<game>/*.log for "starting it through steam.exe" and Steam's compat_log.txt.
```

To ship it: merge, run `Steam bridge build` on `main` (publishes `steam-bridge-proton-11.0-2e-macron.1`), then set `Pinned.SteamBridge` in `internal/wineruntime/manifest.go` to the asset's URL, SHA-256 and size, and change `Pinned.Version` so installed runtimes are rebuilt with it.
