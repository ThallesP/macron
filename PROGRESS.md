# Progress

The loop's memory. Read it first, update it last. GOAL.md has the gates and the rules.

## State

2026-11-14 (loop day 1): gate 1 is green. The Direct3D 11 probe goes through `macron` (DXVK backend) on a GitHub macOS runner and matches its Windows reference (SSIM 0.996); every negative control is rejected. Three agents work on branches: `steam-hook` (gate 5), `game-survey` (candidates for gates 2 and 4), `wine-dxmt` (a Wine that lets DXMT present).

| Gate | State |
|---|---|
| 1. The loop runs | **green**. `macron-check sandbox d3d11-probe` passed from the dev machine (sandbox sbx_3qexmqkk, exit 0); Scenarios workflow green on main ([run 43559902376](https://github.com/ThallesP/macron/actions/runs/43559902376)) |
| 2. A real game at its menu | game survey running |
| 3. Through Macron, not scripts | `macron` launcher: pinned runtime, per-game prefixes, fixes database (backend, env, DLL overrides, extra installs, args, files), Steam verbs; the probe passes through it. Needs real games through it to count |
| 4. Three games | not started |
| 5. Steam's Play button | agent working on branch `steam-hook` |

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

1. Gate 2: pick the first real game from the survey, write its profile and scenario, capture its Windows reference, get it green.
2. Merge `wine-dxmt` when it presents, and offer DXMT as the backend for Direct3D 11 games.
3. Gate 4: two more games from other engines, one of them 1 GB or larger.
4. Gate 5: review and merge `steam-hook`.

## Needs Thalles

- **A licence for Macron.** Until one is picked, no code is copied from GPL projects, which matters most for the Steam hook and the Steam bridge.
- **A Steam account for testing Play.** Gate 5 can't press Play without a logged-in account that owns a Windows game. A throwaway account is safer than a personal one.
- **Fork pull requests.** The repo only asks approval for first-time contributors. "Require approval for all external contributors" in the Actions settings stops a fork from using the Mac runner slots.
- **The MacBook's screen was locked** on 2026-11-14, so no screenshots can be taken there from outside a game.
