# Macron

Mac + Proton. Press Play on a Windows-only game in the Mac Steam client and have it run.

**Status:** early, not released. Working: the launcher (`macron`) with its pinned runtime, which is Wine 11.21 with Macron's six Wine patches, DXMT and DXVK. Also the fixes database, and the scenario checks (`macron-check`): three real games and two programs of ours pass in CI on GitHub's macOS runners, checked against the originals on Windows. That includes a frame-rate check of the 32-bit Direct3D 9 path. On Apple's OpenGL, Wine 11.21 copied the whole streaming vertex buffer twice for every draw a 32-bit Direct3D game made (Ultimate Custom Night ran at 5 fps on an M3), and Macron's patches remove that. There is also an installer (a DMG with a pkg, and `macron uninstall`). Not working yet:
- **Steam's own Install and Play buttons:** registering Macron as a compatibility tool is not enough; the Mac client still refuses Windows-only games with "Invalid platform". How the Steam Play switch goes ahead is Thalles's decision.
- **The Steam bridge:** on hold for its licence.
- **Tests with a logged-in Steam:** they need a test account.

PROGRESS.md has the details and the table below has every game tried.

## How it will work

1. **Steam hook**: switches on the Steam Play code that already ships (disabled) in Steam for Mac and registers Macron as a compatibility tool.
2. **Launcher**: the headless program Steam calls on Play. It prepares a private Windows environment for the game and starts it under Wine.
3. **Steam bridge**: a Mac port of Proton's `lsteamclient`, so games reach the native Steam client for ownership, achievements and cloud saves. On hold: `lsteamclient` is under the Steamworks SDK License, which does not let this repository publish it (see Licences).
4. **Runtime**: Wine 11.21 with Macron's patches (runtime/wine/patches), DXMT (Direct3D 10/11) and DXVK; D3DMetal (Direct3D 12) later. Rosetta 2 first, FEX later.
5. **Fixes database**: per-game settings keyed by Steam app ID.
6. **Installer**: one package that sets everything up. The only UI.

## Milestones

- [ ] Steam calls a dummy compatibility tool when Play is pressed
- [ ] One DirectX 11 game installs, launches, saves and unlocks an achievement
- [ ] Installer, fixes database and ten tested games
- [ ] DirectX 12, controllers, FEX backend

## The launcher (`macron`)

```sh
macron runtime install                       # Wine 11.21 + Macron's Wine patches + DXMT 0.80 + DXVK, pinned URLs and checksums
macron launch --game d3d11-probe -- probe.exe # start a program with its profile from fixes/games/
macron kill --game d3d11-probe               # stop everything in that game's Windows environment
macron waitforexitandrun <exe> [args]        # what Steam calls; the profile comes from SteamAppId
macron uninstall                             # undo the Steam hook, remove runtimes, prefixes and logs
```

`MACRON_BACKEND=dxmt|dxvk|wined3d` overrides the profile's graphics backend for one launch, like Proton's `PROTON_USE_WINED3D`; it is how backends are compared on a game without changing its profile. Macron starts Wine with `WINEDEBUG=-all` unless `WINEDEBUG` is already set, so `WINEDEBUG=fps` prints wined3d's frame rate into the launch log.

To see a game's frame rate, create the file `~/Library/Application Support/Macron/show-fps` (`touch` it) and start the game, from Steam or anywhere: macron then logs wined3d's frame counter (`trace:fps:... @ approx 59.9fps`, Direct3D 8 and 9 and anything on wined3d) into the launch log under `~/Library/Application Support/Macron/logs/<game>/`, and turns on DXVK's HUD and Apple's Metal HUD (DXMT, Direct3D 10 and 11) over the game. Delete the file to turn it off.

`MACRON_OPENGL=llvmpipe` draws OpenGL in software with Mesa (downloaded on first use), for Macs without a hardware OpenGL renderer such as virtual machines and GitHub's runners. Like Proton's `PROTON_USE_WINED3D`, it is a user switch, not part of any game's profile. `MACRON_OPENGL=apple-software` is the other choice there: it sets the Mac driver's `AllowSoftwareRendering`, so Wine's own OpenGL falls back to Apple's software renderer (with Macron's Wine patch 0002, DXMT tree only). Unlike llvmpipe it keeps the path a game takes on a real Mac, including Wine's 32-on-64 OpenGL, which is what the frame-rate check needs.

Everything lives in `~/Library/Application Support/Macron` (or `$MACRON_HOME`): `runtimes/<version>/` holds two Wine trees, plain and with DXMT (an APFS clone, so it costs almost no disk), `prefixes/<game>/` one private Windows environment per game (its user folders are not linked to the Mac's), `logs/<game>/` a log per launch. A profile (`fixes/games/<id>.json`) picks the graphics backend (`dxmt` for Direct3D 10/11 through Metal, `wined3d` for Wine's Direct3D-to-OpenGL), and can add environment variables, DLL overrides, registry values (applied again whenever they change), extra installs (`vcrun2022`, pinned in `fixes/installers.go`, run once per prefix), launch arguments and files to write before the first start (`{game}/…`, `{appdata}/…`, `{documents}/…`). Failures inside macron exit 125 with a stable error code; otherwise macron exits with the game's exit code.

## The installer

`ci/package/build.sh <version> <out-dir>` builds `Macron-<version>.dmg` on a Mac: `macron` as a universal binary, in a pkg that installs it and `macron-uninstall` to `/usr/local/bin`. The pkg's postinstall runs as the logged-in user: `macron runtime install`, then `macron steam enable` if Steam is installed and closed (if Steam is running it skips that and says so). It logs to `/Library/Logs/Macron-install.log`, and when a step fails it undoes what it added. `sudo macron-uninstall` runs `macron uninstall` as that user, then deletes the two programs. `macron uninstall` removes only the paths macron creates (`runtimes`, `prefixes`, `logs`, `downloads`, `show-fps`) and names anything else it finds in the folder. The `Release` workflow (manual) builds the DMG on a macOS 26 runner and uploads it as an artifact; it publishes a release only on `main` with its `publish` input set. The pkg is unsigned: docs/install.md has the Gatekeeper steps.

## Scenario checks (`macron-check`)

A scenario launches a real Windows program through `macron` on a Mac, waits for its menu, and compares a screenshot with the same program on real Windows. docs/checks.md defines the check and its negative controls.

```sh
go build -o ~/.local/bin/macron-check ./cmd/macron-check
macron-check sandbox d3d11-probe     # the whole check in a fresh sbx sandbox; evidence lands in evidence/
```

A frame-rate scenario (`frameRate` in scenario.json, `scenarios/d3d9-fps-probe`) also reads the frame rate a probe of ours prints and runs it a second time throttled, which must fail. The `Frame rate (Windows)` workflow measures the probe on a Windows runner.

CI runs every scenario marked `"ci": true` on each push to `main` (`.github/workflows/scenarios.yml`). References come from the `Reference` workflow on a Windows runner.

## Steam hook (`macron steam`)

The Mac Steam client ships Linux Steam's compatibility-tool code: it scans `compatibilitytools.d` and reads `CompatToolMapping` from `config.vdf`. The hook registers Macron there the way Steam expects.

```sh
macron steam status    # which Steam build this is, and whether Macron is registered
macron steam enable    # register Macron and make it the default tool for every app (AppID 0)
macron steam disable   # put config.vdf's old AppID 0 mapping back and remove Macron's tool folder
```

All three take `--steam-dir <dir>` (default `~/Library/Application Support/Steam`) and `--json`. Quit Steam first: it rewrites `config.vdf` when it exits, so `enable` and `disable` refuse (`STEAM_RUNNING`) while `steam_osx` runs. `steam_osx -shutdown` quits it from a script.

`enable` writes `compatibilitytools.d/macron/` (`compatibilitytool.vdf`, a version 2 `toolmanifest.vdf` whose command line is `/macron %verb%`, and a link to the `macron` binary, so Steam calls `macron waitforexitandrun <game>`), backs `config.vdf` up to `config.vdf.before-macron`, and maps AppID 0 to `macron`. It changes nothing inside Steam's app bundle.

`enable` only runs on a Steam build it knows: the SHA-256 of `steamclient.dylib` and `steam_osx` in `Steam.AppBundle/Steam/Contents/MacOS/` must match a row of `KnownBuilds` in `internal/steamhook/builds.go`. Anything else exits 125 with `UNKNOWN_STEAM_BUILD`, the observed build id and hashes, and changes nothing. `disable` works on any build, so it still undoes the hook after Steam updates itself.

Steam updates itself, so the table has to follow Valve's builds. `cmd/steam-builds` makes the rows from Valve's own update manifest, with no Mac and no account:

```sh
go run ./cmd/steam-builds           # print the KnownBuilds rows of Valve's current Mac build
go run ./cmd/steam-builds --check --write internal/steamhook/builds.go
                                    # exit 1 when a row is missing, and add the missing rows to the table
```

It reads `steam_client_osx` on Valve's update CDN (`client-update.akamai.steamstatic.com`), the manifest Mac clients update from. It downloads `bins_client_osx` and every `steam_osx` package listed there, refuses a zip whose SHA-256 differs from the manifest's `sha2`, and hashes `steamclient.dylib` and each package's `steam_osx`: one row per variant. Each row names its variant and the package file each hash came from, so anyone can download `https://client-update.akamai.steamstatic.com/<package>` and check it. Only the stable manifest is read; beta clients (`steam_client_publicbeta_osx`) are not covered.

How a new build gets supported:

1. The `Steam builds` workflow runs the tool every day, and on demand. When Valve's current build has a row missing, the run fails, pushes branch `steam-build-<id>` (`main` plus the new rows), opens or updates its pull request, and starts the `Steam hook` workflow on that branch. GitHub lets a workflow open pull requests only while Settings > Actions > General > "Allow GitHub Actions to create and approve pull requests" is on; while it is off, the run still pushes the branch and its error links to the page that opens the pull request in one click.
2. The `Steam hook` run installs Steam from Valve's DMG, applies the hook to the new build, restarts Steam and checks it reaches its login window. It also runs on any pull request that changes `builds.go`.
3. Merge the pull request when that run is green. Nothing merges by itself. Old rows stay: people who have not updated yet still run them.

A build Valve no longer lists can still be added by hand: `macron steam status` on a Mac running it prints its build id and hashes.

Valve's manifest lists three `steam_osx` packages for each build, and which one a client runs depends on how it was installed:

- `macos-signed-2` (`steam_osx_macos-signed-2.zip`): what a Steam started from the notarised `/Applications/Steam.app` runs, which is how Valve's DMG installs it. GitHub's runner and Thalles's Mac both had it. The `Steam hook` workflow checks this row on a running client.
- `plain` (`steam_osx.zip`): a different binary for the same build (`c15565c6...` at 1794536281, against `99cc0889...` for the signed one). Its row is in the table, because the evidence is the same as for the signed row short of a running client: Valve's manifest for that build id names it as the build's `steam_osx`, the zip matches the manifest's `sha2`, and it ships with the same `steamclient.dylib`, which holds the compatibility-tool code the hook registers with. What `enable` does (it writes `compatibilitytools.d/macron` and config.vdf's mapping, nothing in Steam's bundle) does not depend on which `steam_osx` runs, so refusing a client on this variant protected nothing. CI cannot check it on a client, because the runner always gets the signed variant, so this row rests on Valve's manifest alone. Which clients run it is not established. Anything Macron later does to the running `steam_osx` itself has to decide again per variant.
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
| Ultimate Custom Night (Steam 871720, 262 MB) | Clickteam Fusion 2.5 (32-bit) | Direct3D 9 | wined3d on OpenGL (default profile, DXMT tree) | title screen, but far too slow to play | MacBook | checked by hand, not a scenario; files fetched with Steam's `+download_depot` because Steam's Install button refuses Windows-only games |
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
| Steam for Mac (Valve) | proprietary | installed by the `Steam hook` workflow to test against; never redistributed, the repo keeps only file hashes |
| Proton's `lsteamclient` (ValveSoftware/Proton, tag `proton-11.0-2e`, commit `bc896217ffcd67f892f952b010f898fe7bd0f0a1`) | Steamworks SDK License (`lsteamclient/LICENSE`), not the BSD 3-Clause of Proton's top level (`LICENSE.proton`). It grants local copies of the source, only to develop software that uses Steamworks, and redistribution of the SDK's `redistributable_bin` folder only; all other rights are reserved. It covers the whole folder: Valve's Wine glue, the generated wrappers and the 91 Steamworks SDK header sets (`cxx.h` alone is Wine's, LGPL 2.1+). Proton's README: "If you distribute a built version of Proton to other users, you must adhere to the terms of these licenses" | not used. Its source, a patch made of its lines, and a build of it (a public artifact or a release) would all be redistribution, which the licence does not grant. PROGRESS.md, "Needs Thalles" |

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
