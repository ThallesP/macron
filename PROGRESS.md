# Progress

The loop's memory. Read it first, update it last. GOAL.md has the gates and the rules.

## State

2026-11-14, phase 2 starts. GOAL.md was rewritten: Thalles tried phase 1 on his MacBook, a game he owns ran only through a workaround and ran slowly, and Steam's own Install and Play buttons did not work. Phase 2's gates:

| Gate | State |
|---|---|
| 1. Fast (Ultimate Custom Night at its own frame rate, cause measured, a frame-rate check in CI) | not started; suspects listed under "From the MacBook" |
| 2. Steam's own Install and Play buttons on the MacBook | not started; registration alone is proven not enough |
| 3. A DMG release Thalles can install | not started |
| 4. The Steam bridge (`SteamAPI_Init`, achievements, cloud) | not started |

Phase 1 (loop day 1) for reference: gates 1 to 4 are green in CI on main ([Scenarios run 43569307839](https://github.com/ThallesP/macron/actions/runs/43569307839)): the Direct3D 11 probe and three games from three engines (OpenTTD, Xonotic at 1.24 GB, SuperTuxKart) reach their main menus through `macron` on GitHub's macOS runner and match the original games captured on Windows runners. Its gate 5 (the Steam registration) is green in CI; what it could not check is now answered below.

| Gate | State |
|---|---|
| 1. The loop runs | **green**. `macron-check sandbox d3d11-probe` passed from the dev machine (sandbox sbx_3qexmqkk, exit 0); Scenarios workflow green on main ([run 43559902376](https://github.com/ThallesP/macron/actions/runs/43559902376)) |
| 2. A real game at its menu | **green**. OpenTTD 15.3 (GDI): `macron-check sandbox openttd` passed (sbx_tb2l7cc7) and the Scenarios workflow is green on main with it ([run 43563385587](https://github.com/ThallesP/macron/actions/runs/43563385587)) |
| 3. Through Macron, not scripts | **green**. Every scenario launches through `macron launch` with its profile from `fixes/games/`: pinned runtime, per-game prefix, backend (dxmt, dxvk, wined3d), env, DLL overrides, registry, extra installs, args, files; Steam's verbs exist. Unit tests on Linux, scenarios as integration tests |
| 4. Three games | **green**. OpenTTD 15.3 (OpenTTD engine, GDI), Xonotic 0.8.6 (DarkPlaces, OpenGL, 1.24 GB), SuperTuxKart 1.5 (Antarctica, Direct3D 9): each passed `macron-check sandbox` (sbx_tb2l7cc7, sbx_7yz2k6go, sbx_uetmo4d4) and passes in CI |
| 5. Steam's Play button | **green without an account, parked for Play.** `macron steam enable` identifies the current Mac Steam build (1794536281) by hashes, registers Macron in `compatibilitytools.d` and makes it the default compatibility tool; Steam reads it (`Mapping AppID 0 to tool "macron"`), still reaches its login window, and an altered client is refused with `UNKNOWN_STEAM_BUILD` and nothing changed. Steam hook workflow green on main ([run 43570046626](https://github.com/ThallesP/macron/actions/runs/43570046626)). Pressing Play: see "Needs Thalles" |

## Layout

- `cmd/macron`, `internal/{wineruntime,launch,failure}`, `fixes/`: the launcher. README.md documents it.
- `cmd/macron-check`, `internal/{check,imagecheck,scenario}`, `scenarios/`, `ci/build-bundle.sh`: the scenario checks. docs/checks.md defines the measure; it was committed before any Macron output was compared.
- `.github/workflows/`: `test.yml` (Go tests, Linux), `scenarios.yml` (scenarios on macos-26), `reference.yml` (references on windows-latest), `sandbox.yml` (sbx).

## From the MacBook, 2026-11-14

Checked by hand on Thalles's MacBook Air (M3, macOS 26.5, Steam build 1794536281, logged in). Everything was removed afterwards; the MacBook is clean.

- **Registration does not switch on Steam Play.** After `macron steam enable` and a restart, `compat_log.txt` had `Client version: 1794536281` and `Mapping AppID 0 to tool "macron" with priority 75`, yet the library page of Ultimate Custom Night (871720, Windows only) showed a greyed-out Install button, and `open steam://install/871720` gave a dialog: `An error occurred while installing Ultimate Custom Night: "Invalid platform"`. So the Mac client reads the mapping and still refuses Windows apps. NotProton's README says it works by "forcibly enabling the Steam Play functionality in macOS Steam (which is present and inert)", ships per-build signatures, and pins Steam builds (stable 1794424036 when read); with it, Steam's Compatibility settings and per-game "Force the use of a specific Steam Play compatibility tool" appear as on Linux.
- **A logged-in Steam downloads Windows depots on request.** `steam_osx +download_depot 871720 871721`, run while Steam is running, is forwarded to it (`console_log.txt`: `ExecCommandLine`), and the files land in `Steam.AppBundle/Steam/Contents/MacOS/steamapps/content/app_871720/depot_871721/`. Useful for getting a test game onto the MacBook before gate 2 works; it is the workaround Thalles does not want as the product.
- **Depot and launch data are local.** `appcache/appinfo.vdf` (binary VDF, magic `0x07564429`: keys are indexes into a string table whose offset follows the header) has every library app's depots, sizes, `oslist` and launch entries. Ultimate Custom Night: depot 871721, 274,753,071 bytes, launch `Ultimate Custom Night.exe`.
- **Ultimate Custom Night runs, slowly.** `SteamAppId=871720 macron waitforexitandrun "Ultimate Custom Night.exe"` (default profile, `dxmt` tree) reached the title screen ("WARNING! ... PRESS ENTER") in a full-screen window of 1920x1243 points; Thalles called it "super slow". It did not need Steam to start. `WINEDEBUG=+loaddll` shows the path: the exe is PE32 (32-bit), unpacks Clickteam Fusion's runtime into `%TEMP%\mrt*.tmp` and loads `mmf2d3d9.dll`, then builtin `d3d9.dll`, `wined3d.dll`, `opengl32.dll`, `ddraw.dll`, `dsound.dll`, `winmm.dll`, `mmdevapi.dll`. So: Direct3D 9 on wined3d's OpenGL renderer, in Wine's 32-on-64 mode, under Rosetta. The log also repeats mpg123's `Illegal Audio-MPEG-Header ... Trying to resync` and creates several MoltenVK instances. `wineserver` in Gcenx's wine-devel 11.21 has no msync or esync strings. No frame times were measured: these are suspects.
- **DXMT presents on a real Apple GPU** with the patched `winemac.so` (now pinned in the runtime, DXMT tree only): the Direct3D 11 probe through `SteamAppId=999999 macron waitforexitandrun probe.exe` printed `adapter: Apple M3`, `RESULT present: OK (60 frames in 529 ms)` and its window showed the reference picture. MoltenVK there reports Metal 3, GPU family Apple 9, MSL 3.1.
- **The hook's build check and Steam's variants.** A Steam started from the notarised `/Applications/Steam.app` selects the package variant `macos-signed` and names its manifest `steam_client_signed_osx.manifest` (also `steam_client_signed-2_osx.manifest`), not `steam_client_osx.manifest`; the hook now globs for it. Its `steamclient.dylib` and `steam_osx` hashes at build 1794536281 are the same as the runner's. A stale client (his was from January) first installs an already-downloaded package, restarts, and only then fetches the current build, so the build can change between two starts.
- **Tools on a Mac without mingw:** `zig cc -target x86_64-windows-gnu` builds the probe. `go test ./internal/launch` fails on macOS in `TestRunPassesTheGamesExitCodeAndProfile` because the temp folder is a symlink (`/var` and `/private/var`); it passes on Linux.
- **Steam's logs folder is recreated at login**, so a watcher that tails `logs/bootstrap_log.txt` across a restart loses the file.

## What is known

From building gate 1, 2026-11-14:

- **DXMT 0.80 cannot present on upstream Wine.** It creates a device, but its swap chain looks up `macdrv_functions` (CrossOver's) or `get_win_data`/`macdrv_view_create_metal_view`/... with `dlsym`, and upstream winemac.so keeps them hidden: `err: Failed to create metal view, it seems like your Wine has no exported symbols needed by DXMT.` An agent is building Wine 11.21 with a small export patch (branch `wine-dxmt`).
- **DXVK-macOS works on stock Wine 11.21** over the bundled MoltenVK 1.4.0, on the runner's paravirtual GPU (MoltenVK reports GPU family Apple 5 / Mac 2, MSL 3.1). It needs the build that ships its own `dxgi.dll` (`v1.10.3-20230507`); the `-repack` drops it for CrossOver's patched Wine dxgi, and upstream Wine's dxgi gives `D3D11CreateDevice: Failed to create a DXGI factory`.
- **Timings on the runner:** runtime install 23-27 s from cache-less download, first `wineboot` plus start to the first window about 80 s, probe at 60 frames in 1.3 s.
- **Windows runner references work:** `windows-latest` renders Direct3D 11 with the Microsoft Basic Render Driver; `BitBlt` from the screen captures the window, two captures 5 s apart were identical.
- **sbx sandboxes can screenshot and list window titles over SSH** (`screencapture -l`, `CGWindowListCopyWindowInfo` titles are readable).
- Wine's windows belong to a process named `wine`; the macOS title bar is 32 points on macOS 26.
- **OpenTTD's title screen moves by itself.** Its camera walks through map places on a timer, so the background depends on uptime; on Windows alone the original scores SSIM 0.209 against itself 30 s later. The scenario compares only the menu panel (region from the reference layout, evidence in reference run 43562346925; see the commit "openttd: compare the menu panel").
- **Wine 11.21 stubs `GetFontResourceInfoW`.** Games that load a private font file and ask GDI for its face name (OpenTTD) get Wine's default sans-serif instead. OpenTTD's profile maps the file names with `FontSubstitutes`; the real fix is implementing the function in Wine (a candidate for a published Wine patch next to the DXMT one).
- **OpenTTD draws its own mouse pointer**, so both sides park the pointer in the top-left corner before starting a game.
- **The runner's GPU has no hardware OpenGL.** Wine's OpenGL gets no pixel format there (Xonotic: `Failed to set video mode to 800x600: No matching GL pixel format available`, also with winemac's `AllowSoftwareRendering`), and Mesa's Zink (GL on Vulkan) cannot run on MoltenVK (`Zink requires the nullDescriptor feature of KHR/EXT robustness2`). That is the runner's virtual GPU, not Macron: the Xonotic scenario sets `MACRON_OPENGL=llvmpipe` (Mesa software OpenGL inside Wine, the same renderer the Windows reference uses). The hardware OpenGL path is untested until a Mac with a GPU runs it (the MacBook).
- **Direct3D 9 works through wined3d's Vulkan renderer** (`WINE_D3D_CONFIG=renderer=vulkan`, MoltenVK); DXVK-macOS's d3d9 cannot make a device on the runner (no geometryShader/shaderCullDistance in MoltenVK). From the game survey.
- **Vision OCR misreads stylised fonts now and then** (Xonotic "Credits" as "Eredits" once); the check reads each frame at 1x and 2x.
- **Test setup belongs to the scenario** (`args`, `files`, `macronEnv`): window size, first-start answers. Profiles hold only what any player of the game needs.
- **DXMT can present with a patched winemac.so** (branch `wine-dxmt`, `runtime/wine/` on main): it adds CrossOver's `macdrv_functions` table with a client-area view per window. The probe drew 60 frames through DXMT in a sandbox. Its release is being published by the Wine build workflow.


From the probe on a GitHub `macos-26` runner (`.github/workflows/probe.yml` and `ci/probe/` on `ci-probe`), 2026-11-13:

- **Machine:** virtual Apple M2 Pro, 5 cores, 14 GB RAM, about 97 GB free disk, macOS 26.7.2. Rosetta is preinstalled.
- **Desktop:** a logged-in session at 1024x768; `screencapture -x` works.
- **GPU:** "Apple Paravirtual device". Metal works for a basic draw and readback, but it reports no Metal 3 and no Apple GPU family, only `mac2`.
- **Wine:** `wine-devel-11.21-osx64.tar.xz` from `Gcenx/macOS_Wine_builds` runs. The first `wineboot` took 65 s.
- **DXMT v0.80** (`dxmt-v0.80-builtin.tar.gz` from `3Shain/dxmt`): copy its `x86_64-windows`, `x86_64-unix` and `i386-windows` files into Wine's `lib/wine/` folders, then run `wineboot -u`, or `winemetal.dll` is not found. A Direct3D 11 test program then created a device, compiled shaders, drew a triangle and read back the right pixels. DXMT logs "Experimental non-Apple GPU support" on this GPU.
- **Wine's own Direct3D 11** fails to create a device there (`0x887a0004`).
- **Steam:** the Mac client installs from `https://cdn.cloudflare.steamstatic.com/client/installer/steam.dmg`, downloads about 1.2 GB on first start, and reaches its login window. macOS shows an "Allow Steam to find devices on local networks?" dialog over it.
- **Workflow trap:** the `runner` context is not available in job-level `env`; GitHub rejects the whole file.

Not tested: a real game, DirectX 9 or 12, Apple's D3DMetal, DXVK over MoltenVK, sound, input.

GitHub's limits: 5 Mac jobs at once, 6 hours per job, 10 GB of Actions cache per repository. Mac runners are free while the repository is public.

On the dev machine: `sbx` is at `~/.local/bin/sbx`, Go is at `~/.local/go/bin` (not on the default PATH).

Existing projects worth reading (see the licence rule in GOAL.md): NotProton (`NotProtonNot/NotProton`) switches on Steam Play in Mac Steam and ports Proton's Steam bridge; Proton-ish (`MagnusPladsen/Proton-ish`) is a fork that adds a free Wine engine and a per-game database.

## Next

1. Gate 1: reproduce the Direct3D 9 slowness without the MacBook (a free 32-bit Clickteam Fusion or other Direct3D 9 game, or a 32-bit Direct3D 9 program of ours, on a sandbox), measure which layer costs the time, then confirm on the MacBook with Ultimate Custom Night. First things to compare: wined3d on OpenGL against wined3d's Vulkan renderer and DXVK's d3d9 on a real GPU, 32-bit against 64-bit, with and without msync, windowed against full screen, Retina mode on and off.
2. Gate 2: find out how the Mac client decides "Invalid platform" and what switches Steam Play on.
3. Gate 3 as soon as gate 2 works once.
4. Left from phase 1: the scenario for the probe still uses DXVK; with the patched `winemac.so` pinned it can run through DXMT. The hardware OpenGL path (Xonotic without `MACRON_OPENGL`) is still unchecked on a real Mac.

## Needs Thalles

- **A licence for Macron.** Until one is picked, no code is copied from GPL projects. Phase 2 writes its own Steam Play switch and ports the Steam bridge from Valve's Proton source.
- **An Apple Developer ID** (99 USD a year) if the installer should open without a Gatekeeper warning. Without it the DMG is unsigned and the release notes say how to open it.
- **Fork pull requests.** The repo only asks approval for first-time contributors. "Require approval for all external contributors" in the Actions settings stops a fork from using the Mac runner slots.
