# Progress

The loop's memory. Read it first, update it last. GOAL.md has the gates and the rules.

## State

2026-11-14 (loop day 1): the launcher, runtime, fixes database and scenario checker exist and have unit tests; the first scenario (the Direct3D 11 probe) is being brought up on the runner. Two agents work in parallel on branches: `steam-hook` (gate 5) and `game-survey` (candidates for gates 2 and 4).

| Gate | State |
|---|---|
| 1. The loop runs | built (`macron-check sandbox <scenario>`, `.github/workflows/scenarios.yml`); waiting for the probe's Windows reference and a first green run |
| 2. A real game at its menu | game survey running |
| 3. Through Macron, not scripts | `macron` launcher: pinned runtime, per-game prefixes, fixes database, Steam verbs; extra installs not yet |
| 4. Three games | not started |
| 5. Steam's Play button | agent working on branch `steam-hook` |

## Layout

- `cmd/macron`, `internal/{wineruntime,launch,failure}`, `fixes/`: the launcher. README.md documents it.
- `cmd/macron-check`, `internal/{check,imagecheck,scenario}`, `scenarios/`, `ci/build-bundle.sh`: the scenario checks. docs/checks.md defines the measure; it was committed before any Macron output was compared.
- `.github/workflows/`: `test.yml` (Go tests, Linux), `scenarios.yml` (scenarios on macos-26), `reference.yml` (references on windows-latest), `sandbox.yml` (sbx).

## What is known

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

1. Gate 1: the verify command and its CI workflow, with the Direct3D 11 probe as the first scenario.
2. In parallel, survey candidate games for gate 2 and find out which ones the runner's virtual GPU can run.

## Needs Thalles

- **A licence for Macron.** Until one is picked, no code is copied from GPL projects, which matters most for the Steam hook and the Steam bridge.
- **A Steam account for testing Play.** Gate 5 can't press Play without a logged-in account that owns a Windows game. A throwaway account is safer than a personal one.
- **Fork pull requests.** The repo only asks approval for first-time contributors. "Require approval for all external contributors" in the Actions settings stops a fork from using the Mac runner slots.
- **The MacBook's screen was locked** on 2026-11-14, so no screenshots can be taken there from outside a game.
