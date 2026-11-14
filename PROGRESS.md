# Progress

The loop's memory. Read it first, update it last. GOAL.md has the gates and the rules.

## State

2026-11-14 (loop day 1): gates 1 to 4 are green in CI on main ([Scenarios run 43569307839](https://github.com/ThallesP/macron/actions/runs/43569307839)): the Direct3D 11 probe and three games from three engines (OpenTTD, Xonotic at 1.24 GB, SuperTuxKart) reach their main menus through `macron` on GitHub's macOS runner and match the original games captured on Windows runners. Gate 5 is green as far as it can go without a Steam account, and parked under "Needs Thalles" for pressing Play.

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

2. Pin the published winemac.so overlay (release `runtime-wine-11.21-macron.1`) in the runtime and run the probe through DXMT.
3. Fold the game survey's full results into the README table.
4. Try the hardware OpenGL path (Xonotic without `MACRON_OPENGL`) on the MacBook, with the game taking its own screenshot since the screen is locked.


## Needs Thalles

- **Pressing Play in Steam (gate 5, parked).** Built and checked without an account, on the GitHub macOS runner with the current Mac Steam (build 1794536281), in the Steam hook workflow on main ([run 43570046626](https://github.com/ThallesP/macron/actions/runs/43570046626), artifact `steam-hook-evidence`):
  - `macron steam enable` recognises the build by the SHA-256 of `steamclient.dylib` and `steam_osx` (table in `internal/steamhook/builds.go`), backs up `config/config.vdf`, writes `compatibilitytools.d/macron/` (whose command line calls `macron waitforexitandrun`) and sets `CompatToolMapping` app 0 to `macron`.
  - After a restart Steam logs `Mapping AppID 0 to tool "macron" with priority 75` in `logs/compat_log.txt`, reaches its login window (`SetLoginState: WaitingForCredentials`, a "Sign in to Steam" window, screenshot `2-after-hook.png`), and keeps the mapping when it quits. `macron steam disable` restores the backup.
  - On a copy of the client with one byte added to `steamclient.dylib`, `enable` exits 125 with `UNKNOWN_STEAM_BUILD` and changes no file.
  - Not checked: whether the Mac client, once logged in, offers Play on a Windows-only game and actually calls `macron waitforexitandrun`. NotProton's notes say Mac Steam's compatibility manager keeps an "enabled" flag off while the client runs, and NotProton turns it on by injecting a dylib into Steam and setting the flag in memory. Macron's hook does only the registration Steam reads from its own config files and changes nothing inside Valve's signed client.
  - **Decision for Thalles:** if a logged-in test shows Steam Play stays off, the only way known so far is to inject into or patch Valve's client, as NotProton does. That needs (a) a licence for Macron, because the known approach is GPL, and (b) your call on whether Macron should modify the Steam client at all. `macron`'s Steam verbs take the profile from `SteamAppId` and the prefix from `STEAM_COMPAT_DATA_PATH`, but have never been called by Steam.
  - What is needed: a Steam account that owns a free Windows-only game (a throwaway account is safer than a personal one), logged in on the MacBook or provided as CI secrets for a workflow that never runs on forks. Then: `macron steam enable`, press Play, and check `logs/compat_log.txt` and macron's log in `~/Library/Application Support/Macron/logs/`.
  - Steam updates itself; when the build changes, the workflow fails with the new build id and hashes, and README.md "Steam hook" says how to add it.

- **A licence for Macron.** Until one is picked, no code is copied from GPL projects, which matters most for the Steam hook and the Steam bridge.
- **A Steam account for testing Play.** Gate 5 can't press Play without a logged-in account that owns a Windows game. A throwaway account is safer than a personal one.
- **Fork pull requests.** The repo only asks approval for first-time contributors. "Require approval for all external contributors" in the Actions settings stops a fork from using the Mac runner slots.
- **The MacBook's screen was locked** on 2026-11-14, so no screenshots can be taken there from outside a game.
