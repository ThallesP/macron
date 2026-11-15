# Macron

Mac + Proton. Press Play on a Windows-only game in the Mac Steam client and have it run.

**Status:** early; [release 0.2.0](https://github.com/ThallesP/macron/releases/tag/v0.2.0) is the first to try. On a Mac sandbox, with Steam signed in to a test account: a Windows-only game (Ultimate Custom Night) gets a working Install button, Steam downloads it into its normal library, Play starts it through `macron`, Steam shows it running, and quitting it or pressing Stop returns Steam to Play; a Mac-native game still installs and runs as before; `macron uninstall` puts Steam back byte for byte. docs/install.md says how to install it; PROGRESS.md has the evidence and the table below has every game tried.

What is in it: the launcher (`macron`) with its pinned runtime (Wine 11.21 with Macron's six Wine patches, DXMT and DXVK), the Steam Play switch (`steamplay/`), the fixes database, an installer (a DMG with a pkg, and `macron uninstall`), and the scenario checks (`macron-check`): three real games and three programs of ours pass in CI on GitHub's macOS runners, checked against the originals on Windows, including a frame-rate check of the 32-bit Direct3D 9 path.

Not working yet:
- **The Steam bridge:** built (`lsteamclient/`, `steam_helper/`) and checked on sandboxes without a signed-in Steam (`SteamAPI_Init` reaches the Mac client and fails cleanly), but not released yet, so games that talk to Steam (`SteamAPI_Init`, achievements, cloud saves) do not find it yet (docs/steam-bridge.md).
- **Speed on a real Mac is unmeasured since the fixes.** The sandboxes have a virtual GPU. On an M3, before Macron's Wine patches, Ultimate Custom Night ran at 5 fps.

## How it works

1. **Steam hook**: switches on the Steam Play code that already ships (disabled) in Steam for Mac (`steamplay/README.md`) and registers Macron as a compatibility tool.
2. **Launcher**: the headless program Steam calls on Play. It prepares a private Windows environment for the game and starts it under Wine.
3. **Steam bridge**: a Mac port of Proton's `lsteamclient` and its `steam.exe` helper, so games reach the native Steam client for ownership, achievements and cloud saves. It stays under Valve's licences in its own folders (see Licences).
4. **Runtime**: Wine 11.21 with Macron's patches (runtime/wine/patches), DXMT (Direct3D 10/11) and DXVK; D3DMetal (Direct3D 12) later. Rosetta 2 first, FEX later.
5. **Fixes database**: per-game settings keyed by Steam app ID.
6. **Installer**: one package that sets everything up. The only UI.

## Milestones

- [x] Steam calls a compatibility tool when Play is pressed
- [ ] One DirectX 11 game installs, launches, saves and unlocks an achievement
- [ ] Installer, fixes database and ten tested games
- [ ] DirectX 12, controllers, FEX backend

## The launcher (`macron`)

```sh
macron runtime install                       # Wine 11.21 + Macron's Wine patches + DXMT 0.80 + DXVK, pinned URLs and checksums
macron launch --game d3d11-probe -- probe.exe # start a program with its profile from fixes/games/
macron kill --game d3d11-probe               # stop everything in that game's Windows environment
macron waitforexitandrun <exe> [args]        # what Steam calls; the profile comes from SteamAppId
macron uninstall                             # put Steam back as it was, remove runtimes, prefixes and logs
```

`MACRON_BACKEND=dxmt|dxvk|wined3d` overrides the profile's graphics backend for one launch, like Proton's `PROTON_USE_WINED3D`; it is how backends are compared on a game without changing its profile. Macron starts Wine with `WINEDEBUG=-all` unless `WINEDEBUG` is already set, so `WINEDEBUG=fps` prints wined3d's frame rate into the launch log.

To see a game's frame rate, create the file `~/Library/Application Support/Macron/show-fps` (`touch` it) and start the game, from Steam or anywhere: macron then logs wined3d's frame counter (`trace:fps:... @ approx 59.9fps`, Direct3D 8 and 9 and anything on wined3d) into the launch log under `~/Library/Application Support/Macron/logs/<game>/`, and turns on DXVK's HUD and Apple's Metal HUD (DXMT, Direct3D 10 and 11) over the game. Delete the file to turn it off.

`MACRON_OPENGL=llvmpipe` draws OpenGL in software with Mesa (downloaded on first use), for Macs without a hardware OpenGL renderer such as virtual machines and GitHub's runners. Like Proton's `PROTON_USE_WINED3D`, it is a user switch, not part of any game's profile. `MACRON_OPENGL=apple-software` is the other choice there: it sets the Mac driver's `AllowSoftwareRendering`, so Wine's own OpenGL falls back to Apple's software renderer (with Macron's Wine patch 0002, DXMT tree only). Unlike llvmpipe it keeps the path a game takes on a real Mac, including Wine's 32-on-64 OpenGL, which is what the frame-rate check needs.

Everything lives in `~/Library/Application Support/Macron` (or `$MACRON_HOME`): `runtimes/<version>/` holds two Wine trees, plain and with DXMT (an APFS clone, so it costs almost no disk), `prefixes/<game>/` one private Windows environment per game (its user folders are not linked to the Mac's), `logs/<game>/` a log per launch. A profile (`fixes/games/<id>.json`) picks the graphics backend (`dxmt` for Direct3D 10/11 through Metal, `wined3d` for Wine's Direct3D-to-OpenGL), and can add environment variables, DLL overrides, registry values (applied again whenever they change), extra installs (`vcrun2022`, pinned in `fixes/installers.go`, run once per prefix), launch arguments and files to write before the first start (`{game}/…`, `{appdata}/…`, `{documents}/…`). Failures inside macron exit 125 with a stable error code; otherwise macron exits with the game's exit code.

## The installer

`ci/package/build.sh <version> <out-dir>` builds `Macron-<version>.dmg` on a Mac: `macron` as a universal binary and the Steam Play switch library, in a pkg that installs `macron` and `macron-uninstall` to `/usr/local/bin` and the library to `/usr/local/lib/macron`. The pkg's postinstall runs as the logged-in user: `macron runtime install`, then, when Steam is installed, it closes Steam, runs `macron steam enable` and opens Steam again if it was open. It logs to `/Library/Logs/Macron-install.log`, and when a step fails it undoes what it added. `sudo macron-uninstall` closes Steam, runs `macron uninstall` as that user, then deletes the programs and the library. `macron uninstall` removes only the paths macron creates (`runtimes`, `prefixes`, `logs`, `downloads`, `show-fps`, `steam-app-backup`) and names anything else it finds in the folder. The `Release` workflow (manual) builds the DMG on a macOS 26 runner and uploads it as an artifact; it publishes a release only on `main` with its `publish` input set. The pkg is unsigned: docs/install.md has the Gatekeeper steps, and is the text of the release notes.

## The Steam bridge

When Steam starts a game (`macron waitforexitandrun` or `run`), macron starts it through `C:\windows\system32\steam.exe`, Proton's `steam_helper` (`steam_helper/`), after putting the bridge into the prefix: `lsteamclient.dll` from the runtime's Wine tree is copied to `C:\Program Files (x86)\Steam\steamclient.dll` (i386) and `steamclient64.dll` (x86_64), and `HKCU\Software\Valve\Steam\ActiveProcess` names them, the way Proton's prefix does. A game's `steam_api(64).dll` then loads that copy, Wine loads it as its builtin `lsteamclient.dll`, and its Unix side loads the Mac client's `steamclient.dylib` (from `STEAM_COMPAT_CLIENT_INSTALL_PATH`, else `~/Library/Application Support/Steam`, in `Steam.AppBundle/Steam/Contents/MacOS/`). `lsteamclient/build.sh` builds it on a Mac from Valve's source, which `lsteamclient/fetch.sh` downloads and checks; the `Steam bridge build` workflow publishes it, and `internal/wineruntime` installs it into both Wine trees once `Manifest.SteamBridge` pins a release. A runtime without it starts the game directly, as before. docs/steam-bridge.md has what was built and checked.

## Scenario checks (`macron-check`)

A scenario launches a real Windows program through `macron` on a Mac, waits for its menu, and compares a screenshot with the same program on real Windows. docs/checks.md defines the check and its negative controls.

```sh
go build -o ~/.local/bin/macron-check ./cmd/macron-check
macron-check sandbox d3d11-probe     # the whole check in a fresh sbx sandbox; evidence lands in evidence/
```

A frame-rate scenario (`frameRate` in scenario.json, `scenarios/d3d9-fps-probe`) also reads the frame rate a probe of ours prints and runs it a second time throttled, which must fail. The `Frame rate (Windows)` workflow measures the probe on a Windows runner.

CI runs every scenario marked `"ci": true` on each push to `main` (`.github/workflows/scenarios.yml`). References come from the `Reference` workflow on a Windows runner.

## Steam hook (`macron steam`)

The Mac Steam client ships Linux Steam's compatibility-tool code (Steam Play), switched off. The hook switches it on and registers Macron the way Steam expects.

```sh
macron steam status    # which Steam build this is, whether Macron is registered and Steam Play is on
macron steam enable    # switch Steam Play on and make Macron the default tool for every app (AppID 0)
macron steam disable   # put Steam.app and config.vdf back and remove Macron's tool folder
```

All three take `--steam-dir <dir>` (default `~/Library/Application Support/Steam`), `--steam-app <Steam.app>` (default `/Applications/Steam.app`) and `--json`. Quit Steam first: it rewrites `config.vdf` when it exits, so `enable` and `disable` refuse (`STEAM_RUNNING`) while `steam_osx` runs. `steam_osx -shutdown` quits it from a script.

`enable` does two things:

- **Registers the tool.** It writes `compatibilitytools.d/macron/` (`compatibilitytool.vdf`, a version 2 `toolmanifest.vdf` whose command line is `/macron %verb%`, and a link to the `macron` binary, so Steam calls `macron waitforexitandrun <game>`), backs `config.vdf` up to `config.vdf.before-macron`, and maps AppID 0 to `macron`.
- **Switches Steam Play on** (`steamplay/README.md` has the whole story). It copies the switch library to `Steam.app/Contents/MacOS/macron-steamplay.dylib`, sets `DYLD_INSERT_LIBRARIES` and `STEAM_EXTRA_COMPAT_TOOLS_PATHS` in `LSEnvironment` of Steam.app's `Info.plist`, and signs Steam.app ad hoc, because macOS ignores that variable under Valve's hardened-runtime signature. Valve's `Info.plist`, `steam_osx` and `CodeResources` are kept in `~/Library/Application Support/Macron/steam-app-backup`. In the running client the library changes two instructions of `steamclient.dylib` in memory: the one that decides Steam Play is off on a Mac, and the one that makes Steam's Stop button give up on a game that is not a Mac application. It refuses (`STEAM_APP_CHANGED`) a Steam.app into which another tool already loads a library.

`disable` puts Valve's three files back byte for byte when Steam.app is as `enable` left it (Valve's notarised signature then verifies again). If something else changed Steam.app since, it takes only its own settings and library out and signs ad hoc again, and says that Valve's signature needs a reinstall of Steam; if Steam replaced the bundle itself, there is nothing to undo. Because the signature changes, both reset Steam's Accessibility and input-monitoring permissions, so macOS asks again instead of silently refusing.

`enable` only runs on a Steam build it knows: the SHA-256 of `steamclient.dylib` and `steam_osx` in `Steam.AppBundle/Steam/Contents/MacOS/` must match a row of `KnownBuilds` in `internal/steamhook/builds.go`. Anything else exits 125 with `UNKNOWN_STEAM_BUILD`, the observed build id and hashes, and changes nothing. `disable` works on any build, so it still undoes the hook after Steam updates itself. The switch library checks again every time Steam starts, by the build id inside the loaded `steamclient.dylib` (its `LC_UUID`): when Steam has updated itself to a build without a row, it logs `UNKNOWN_STEAM_BUILD` to `~/Library/Application Support/Macron/logs/steamplay.log`, changes nothing, and Steam runs as stock Steam until a Macron that knows the build is installed.

Steam updates itself, so the table has to follow Valve's builds. `cmd/steam-builds` makes the rows from Valve's own update manifest, with no Mac and no account:

```sh
go run ./cmd/steam-builds           # print the KnownBuilds rows of Valve's current Mac build
go run ./cmd/steam-builds --check --write internal/steamhook/builds.go
                                    # exit 1 when a row is missing, and add the missing rows to the table
```

It reads `steam_client_osx` on Valve's update CDN (`client-update.akamai.steamstatic.com`), the manifest Mac clients update from. It downloads `bins_client_osx` and every `steam_osx` package listed there, refuses a zip whose SHA-256 differs from the manifest's `sha2`, and hashes `steamclient.dylib` and each package's `steam_osx`: one row per variant. In the arm64 half of `steamclient.dylib` it also finds the two instructions the switch changes, by what surrounds them (the `"linux"` string and the enabled flag's offset for the first, the `terminate` selector's one caller for the second), and refuses a binary where either is missing or ambiguous (`UNKNOWN_STEAM_CLIENT_CODE`): then a person has to look at what Valve changed. `--header steamplay/builds.h` regenerates the library's table from the rows. Each row names its variant and the package file each hash came from, so anyone can download `https://client-update.akamai.steamstatic.com/<package>` and check it. Only the stable manifest is read; beta clients (`steam_client_publicbeta_osx`) are not covered.

How a new build gets supported:

1. The `Steam builds` workflow runs the tool every day, and on demand. When Valve's current build has a row missing, the run fails, pushes branch `steam-build-<id>` (`main` plus the new rows and the regenerated `steamplay/builds.h`), opens or updates its pull request, and starts the `Steam hook` workflow on that branch. GitHub lets a workflow open pull requests only while Settings > Actions > General > "Allow GitHub Actions to create and approve pull requests" is on; while it is off, the run still pushes the branch and its error links to the page that opens the pull request in one click.
2. The `Steam hook` run installs Steam from Valve's DMG, applies the hook to the new build, restarts Steam and checks that the library switched Steam Play on in the running client, that Steam registered the tool and reached its login window, that a library built without the build's row refuses it, and that `disable` leaves Steam.app byte for byte as Valve shipped it. It also runs on any pull request that changes `builds.go`. It needs no account.
3. Merge the pull request when that run is green. Nothing merges by itself. Old rows stay: people who have not updated yet still run them. The `Steam Play` workflow presses Steam's real Install and Play buttons with the test account (`ci/steam-play/gate2.sh`) every morning on `main` and on demand; run it before cutting the release that carries the new row to people's Macs.

A build Valve no longer lists can still be added by hand: `macron steam status` on a Mac running it prints its build id and hashes.

Valve's manifest lists three `steam_osx` packages for each build, and which one a client runs depends on how it was installed:

- `macos-signed-2` (`steam_osx_macos-signed-2.zip`): what a Steam started from the notarised `/Applications/Steam.app` runs, which is how Valve's DMG installs it. GitHub's runner and Thalles's Mac both had it. The `Steam hook` workflow checks this row on a running client.
- `plain` (`steam_osx.zip`): a different binary for the same build (`c15565c6...` at 1794536281, against `99cc0889...` for the signed one). Its row is in the table, because the evidence is the same as for the signed row short of a running client: Valve's manifest for that build id names it as the build's `steam_osx`, the zip matches the manifest's `sha2`, and it ships with the same `steamclient.dylib`, which holds the compatibility-tool code the hook registers with. What `enable` does does not depend on which `steam_osx` runs: the registration is files outside Steam's bundle, and both instructions the switch changes are in `steamclient.dylib`, which is the same file for both variants. CI cannot check it on a client, because the runner always gets the signed variant (also after Steam.app is signed ad hoc), so this row rests on Valve's manifest alone. Which clients run it is not established.
- `steamchina` (`steam_osx_steamchina.zip`): holds `steamchina_osx` and no `steam_osx`, so it gets no row. A client on it would have no `steam_osx` for the hook to hash, so `enable` stops at `STEAM_NOT_FOUND` and changes nothing (inferred from the package, not seen on a client).

Known builds: `1794536281` (stable, 2026-11-13), variants `macos-signed-2` and `plain`.

## Games

Every game tried. The first three are scenarios that pass in CI; the next five come from the game survey (branch `game-survey`, `docs/games.md`), checked by screenshot in a sandbox, not by a scenario. "Where" is where it was checked: the sandbox/CI runner (virtual M2 Pro, no Metal 3) or Thalles's MacBook (M3).

| Game | Engine | Graphics API | Backend | How far | Where | Notes |
|---|---|---|---|---|---|---|
| OpenTTD 15.3 (11 MB + OpenGFX/OpenSFX) | OpenTTD (own) | GDI (2D) | wined3d (plain Wine) | menu, matches Windows (menu panel SSIM 0.942, menu text found) | runner, CI | needs FontSubstitutes in its profile: Wine stubs `GetFontResourceInfoW`, so its own font was replaced by Wine's default |
| Xonotic 0.8.6 (1.24 GB) | DarkPlaces | OpenGL | wined3d + `MACRON_OPENGL=llvmpipe` (software GL) | menu, matches Windows (SSIM 0.953, menu text found) | runner, CI | the runner has no hardware OpenGL: without the switch, `No matching GL pixel format available`; Mesa Zink needs robustness2 nullDescriptor, which MoltenVK lacks. Hardware GL on a real Mac not checked yet |
| SuperTuxKart 1.5 (922 MB) | Antarctica (Irrlicht fork) | Direct3D 9 (`--render-driver=directx9`) | wined3d, Vulkan renderer (`WINE_D3D_CONFIG=renderer=vulkan`, `MVK_CONFIG_USE_MTLHEAP=0`) | menu, matches Windows (SSIM 0.971, menu text found) | runner, CI | DXVK d3d9 can't make a device (MoltenVK lacks geometryShader, shaderCullDistance); no d3dx9_43, so fixed pipeline on both sides; with MoltenVK's Metal heaps about one second in ten of its menu is drawn broken (PROGRESS.md) |
| Endless Sky 0.11.4 | its own | OpenGL | wined3d + Mesa llvmpipe | menu (survey, not a scenario) | runner | plain winemac GL: `No matching GL pixel format available` |
| Unreal Tournament GOTY + OldUnreal 469f | Unreal Engine 1 | OpenGL / D3D9 / D3D11 renderers | OpenGLDrv + Mesa llvmpipe | title screen ("Press ESC to begin."), one key press short of the menu | runner | D3D11Drv under DXVK: black window, endless MoltenVK `VkDescriptorPool exhausted`; D3D9Drv under wined3d-Vulkan: black window |
| Rigs of Rods 2026.01 | OGRE | Direct3D 9 | wined3d, Vulkan renderer | background only, menu never appears | runner | log stops at `Texture 'accept.png': Loading`; DXVK d3d9: `Cannot create device!` |
| 0 A.D. 0.28.0 | Pyrogenesis | OpenGL | wined3d + Mesa llvmpipe | crash before the menu | runner | `unhandled exception (Access violation reading 0x00000080)` right after GL comes up |
| osu!(lazer) 2026.1005.0 | osu-framework (.NET) | Direct3D 11 | DXVK | crash before the menu | runner | its auth module crashes: `Unhandled exception code c0000005`, exit 5 |
| Ultimate Custom Night (Steam 871720, 262 MB) | Clickteam Fusion 2.5 (32-bit) | Direct3D 9 | wined3d on OpenGL (default profile, DXMT tree) | installed with Steam's Install button and started with Play: intro, title screen and character-select menu draw; Steam shows it running; Stop and quitting return to Play. Frame rate not measured yet | runner (sandbox, test account); MacBook before the Wine patches: title screen at 5 fps | `ci/steam-play/gate2.sh`, screenshots in `docs/proof/gate2-sbx_qsjpicyv/`. On the MacBook the files had to be fetched with Steam's `+download_depot`, because Steam's Install button refused Windows-only games then |
| Overwatch (Steam 2357570, 82 GB) | Blizzard's own | Direct3D 11 (its Direct3D 12 probe fails first) | DXMT (default) with the Steam bridge | its Battle.net sign-in screen, the farthest it goes without a Blizzard account | runner (sandbox, test account), installed with Steam's Install button, started with Play | without the bridge it stops at "unable to connect to a running instance of Steam"; with it, it shows the Steam profile name and the sign-in screen on the Paravirtual device. Main menu and frame rate need a Blizzard account and a real GPU (Thalles). `docs/proof/gate5/` |
| Endless Sky (Steam 404410, free, Mac-native) | its own | OpenGL | none: Steam starts its own `.app`, not macron | installs and starts from Steam before and after `macron steam enable`, the same way (`Endless Sky.app/Contents/MacOS/Endless Sky`, no macron process); Stop works | runner (sandbox, test account) | the regression check that Mac-native games are untouched. Its window stays black on the runner both before and after: the virtual GPU has no hardware OpenGL |
| TOMAZE (free on itch.io, open source, 12 MB) | Clickteam Fusion 2.5 (32-bit) | Direct3D 9 (`mmf2d3d9.dll`) | wined3d on OpenGL (default profile `tomaze`, DXMT tree) + `MACRON_OPENGL=apple-software` | animated title screen, drawn whole by eye (no Windows reference); 24 fps on the runner's software renderer | runner | with macron.4 only 8 of 70 starts lasted through a measurement: the others crashed or exited at start the way the 32/64-bit mode switch under Rosetta fails, which Wine patch 0006 fixes (6 of 6 starts with it). Its frames make 5 draws and no buffer maps, so no wow64 copies even with stock wined3d. Not a scenario: itch.io has no stable download URL. PROGRESS.md, "A real Clickteam Fusion game" |
| Alternative Facts 0.35f (Clickteam's own, free on itch.io, 4.2 MB) | Clickteam Fusion 2.5 (32-bit) | Direct3D 9 (`mmf2d3d9.dll`) | same as TOMAZE | static title screen, drawn whole by eye; 27 fps with macron.4's wined3d, 21.5 with stock | runner | measured in 9 of 26 starts with macron.4, 7 of 7 with Wine patch 0006; Macron's wined3d patches use one core where stock wined3d uses 1.8 |
| Direct3D 9 frame-rate probe (ours, not a game) | none: 400 alpha-blended sprites, one draw each, like a Clickteam Fusion scene | Direct3D 9, 32-bit and 64-bit | wined3d with Mesa llvmpipe (`MACRON_OPENGL=llvmpipe`); wined3d's Vulkan renderer for 64-bit only | draws its picture, matches Windows (SSIM 1.000); frame rate measured, threshold not set yet (`ci: false`) | runner | 32-bit through wined3d's Vulkan renderer exits 3 after MoltenVK's `vkCreateBuffer(): Only external memory handle type VK_EXTERNAL_MEMORY_HANDLE_TYPE_MTLBUFFER_BIT_EXT and ..._MTLHEAP_BIT_EXT are supported`; DXVK cannot make a device (no geometryShader, shaderCullDistance); wined3d's OpenGL renderer gets no pixel format on the runner. docs/checks.md, "How a frame rate is checked" |
| Direct3D 11 probe (ours, not a game) | none | Direct3D 11 | DXVK and DXMT (Macron's default) in CI; DXMT on the MacBook | draws its picture, matches Windows (SSIM 0.996 in CI; by eye on the MacBook, 60 frames in 529 ms) | runner, MacBook | DXMT presents with Macron's patched `winemac.so`, which the runtime now puts in its DXMT tree |

## Licences

| Dependency | Licence | Used as |
|---|---|---|
| Wine (Gcenx/macOS_Wine_builds 11.21) | LGPL 2.1+ | downloaded at install time, unmodified |
| Wine 11.21 `winemac.so`, `wined3d.dll` and `wow64cpu.dll` with Macron's patches (this repository's `runtime-wine-11.21-macron.5` release) | LGPL 2.1+ | downloaded at install time: `winemac.so` into the DXMT tree, `wined3d.dll` and `wow64cpu.dll` into both trees; source and patches are published with the release |
| DXMT 0.80 (3Shain/dxmt) | LGPL 2.1+ | downloaded at install time, unmodified |
| DXVK-macOS 1.10.3-20230507 (Gcenx/DXVK-macOS) | zlib | downloaded at install time, unmodified (the `dxvk` backend) |
| Mesa 26.1.8 for Windows (pal1000/mesa-dist-win) | MIT | downloaded on Windows runners for OpenGL references, and by macron when `MACRON_OPENGL=llvmpipe` |
| Go standard library | BSD 3-Clause | built into macron and macron-check |
| Steam for Mac (Valve) | proprietary | installed by the `Steam hook` and `Steam Play` workflows to test against; never redistributed, the repo keeps only file hashes, build ids and the addresses of two instructions |
| NotProton (NotProtonNot/NotProton) | GPL-3.0 | read as the reference for how Steam Play is switched on in Steam for Mac; none of its source is in this repository (`steamplay/` is Macron's own code) |
| Proton's `lsteamclient` (ValveSoftware/Proton, tag `proton-11.0-2e`, commit `bc896217ffcd67f892f952b010f898fe7bd0f0a1`), ported in `lsteamclient/` | Steamworks SDK License (`lsteamclient/LICENSE`, Valve's file), not the BSD 3-Clause of Proton's top level. The `lsteamclient/` folder and everything built from it stay under Valve's licence, not Macron's. As NotProton does, the repository keeps only Macron's patches and the scripts that fetch Valve's source and the Steamworks SDK headers it carries at build time, checked by digest; Valve's source is never committed. Valve has not confirmed that redistribution is allowed: ValveSoftware/Proton issue #7801 asks and is open | built from source at build time; the built DLLs and Unix library are to ship as their own release asset (`steam-bridge-*`), not in the LGPL Wine overlay |
| Proton's `steam_helper` (same commit), ported in `steam_helper/` | BSD 3-Clause (`steam_helper/LICENSE`, Proton's `LICENSE.proton`) | fetched and patched at build time like `lsteamclient`; `steam.exe` ships in the same asset |
| Steamworks SDK 1.65 `steam_api.dll` and `steam_api64.dll` (Valve's `redistributable_bin`, taken from Steamworks.NET at commit `ea51b029`) | Steamworks SDK License, which allows redistributing `redistributable_bin` | downloaded by `tools/steam-api-probe/build.sh` to test the bridge, checked by SHA-256; never committed |

Macron itself has no licence yet (see PROGRESS.md, "Needs Thalles").

## Mac sandboxes (`sbx`)

Macron is developed on Linux, so `sbx` gives you a Mac to run commands on. Each sandbox is a GitHub Actions Mac runner (macOS 26, Apple silicon, 5 cores, 14 GB RAM, a logged-in desktop) that you reach over SSH.

Build it once. It needs Go 1.24+, OpenSSH, and a `gh` login that can start workflows on this repository:

```sh
cd tools/sandbox && go build -o ~/.local/bin/sbx ./cmd/sbx
```

```sh
id=$(sbx new --json | jq -r 'select(.event == "ready") | .id')   # ready in about 20 seconds
sbx exec "$id" -- sw_vers              # exits with the command's exit code
sbx exec "$id" 'cd /tmp && ls | wc -l' # one argument is a shell line
sbx scp ./build.tgz "$id":/tmp/build.tgz
sbx scp -r "$id":/tmp/out ./out
sbx ssh "$id"                          # interactive shell
sbx list                               # running sandboxes; --all adds stopped ones
sbx info "$id"
sbx extend "$id" --ttl 2h              # stop 2 hours from now instead
sbx stop "$id"
```

Things to know:

- A sandbox stops itself 30 minutes after `new` unless you pass `--ttl` or call `extend`. The timer does not look at activity. No sandbox runs longer than 5 hours 30 minutes.
- Stopping erases the machine. There are no snapshots yet.
- At most 5 sandboxes run at once (GitHub's limit for Mac jobs).
- Only the machine that ran `sbx new` holds the sandbox's SSH key, in `~/.local/state/sbx/`. Other machines can list and stop it, not log in.
- Every command accepts `--json`. `sbx`'s own failures exit 125 with an error code on stderr; wrong usage exits 2.
- The tunnel sometimes stalls. Before login `sbx` retries by itself. After login it cannot know whether your command ran, so `exec` gives up after about a minute and exits 255: check the state on the sandbox before running the command again.
- The repository is public, so the workflow's inputs, logs and artifacts are too. `sbx` puts only public keys there, and the tunnel address encrypted. Never print a secret from a workflow step.

## Not planned

Games that need kernel anti-cheat (EasyAntiCheat, BattlEye, Vanguard).
