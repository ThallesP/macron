# Macron

Mac + Proton. Press Play on a Windows-only game in the Mac Steam client and have it run.

**Status:** early. The launcher (`macron`), its pinned runtime, the fixes database and the scenario checks (`macron-check`) exist, and three real games reach their menus through it on GitHub's macOS runners, checked against the originals on Windows. Steam integration is not done. PROGRESS.md has the details, the table below has every game tried.

## How it will work

1. **Steam hook**: switches on the Steam Play code that already ships (disabled) in Steam for Mac and registers Macron as a compatibility tool.
2. **Launcher**: the headless program Steam calls on Play. It prepares a private Windows environment for the game and starts it under Wine.
3. **Steam bridge**: a Mac port of Proton's `lsteamclient`, so games reach the native Steam client for ownership, achievements and cloud saves.
4. **Runtime**: Wine with DXMT (DirectX 11) and D3DMetal (DirectX 12). Rosetta 2 first, FEX later.
5. **Fixes database**: per-game settings keyed by Steam app ID.
6. **Installer**: one package that sets everything up. The only UI.

## Milestones

- [ ] Steam calls a dummy compatibility tool when Play is pressed
- [ ] One DirectX 11 game installs, launches, saves and unlocks an achievement
- [ ] Installer, fixes database and ten tested games
- [ ] DirectX 12, controllers, FEX backend

## The launcher (`macron`)

```sh
macron runtime install                       # Wine 11.21 + DXMT 0.80, pinned URLs and checksums
macron launch --game d3d11-probe -- probe.exe # start a program with its profile from fixes/games/
macron kill --game d3d11-probe               # stop everything in that game's Windows environment
macron waitforexitandrun <exe> [args]        # what Steam calls; the profile comes from SteamAppId
```

`MACRON_OPENGL=llvmpipe` draws OpenGL in software with Mesa (downloaded on first use), for Macs without a hardware OpenGL renderer such as virtual machines and GitHub's runners. Like Proton's `PROTON_USE_WINED3D`, it is a user switch, not part of any game's profile.

Everything lives in `~/Library/Application Support/Macron` (or `$MACRON_HOME`): `runtimes/<version>/` holds two Wine trees, plain and with DXMT (an APFS clone, so it costs almost no disk), `prefixes/<game>/` one private Windows environment per game (its user folders are not linked to the Mac's), `logs/<game>/` a log per launch. A profile (`fixes/games/<id>.json`) picks the graphics backend (`dxmt` for Direct3D 10/11 through Metal, `wined3d` for Wine's Direct3D-to-OpenGL), and can add environment variables, DLL overrides, registry values (applied again whenever they change), extra installs (`vcrun2022`, pinned in `fixes/installers.go`, run once per prefix), launch arguments and files to write before the first start (`{game}/…`, `{appdata}/…`, `{documents}/…`). Failures inside macron exit 125 with a stable error code; otherwise macron exits with the game's exit code.

## Scenario checks (`macron-check`)

A scenario launches a real Windows program through `macron` on a Mac, waits for its menu, and compares a screenshot with the same program on real Windows. docs/checks.md defines the check and its negative controls.

```sh
go build -o ~/.local/bin/macron-check ./cmd/macron-check
macron-check sandbox d3d11-probe     # the whole check in a fresh sbx sandbox; evidence lands in evidence/
```

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

Steam updates itself, so the known build goes stale. When the `Steam hook` workflow fails with "Steam build not in the table", add the new build:

1. Take the build id and the two hashes from the failed run's log (or run `macron steam status` on a Mac with the new Steam).
2. Add a row to `KnownBuilds` with that id and those hashes. Keep the old rows: people who have not updated yet still run them.
3. Push; the workflow applies the hook to the new build, restarts Steam and checks it reaches its login window again.

Known builds: `1794536281` (stable, 2026-11-13).

## Games

Every game tried. "Where" is where it was checked: the sandbox/CI runner (virtual M2 Pro, no Metal 3) or Thalles's MacBook (M3).

| Game | Engine | Graphics API | Backend | How far | Where | Notes |
|---|---|---|---|---|---|---|
| OpenTTD 15.3 (11 MB + OpenGFX/OpenSFX) | OpenTTD (own) | GDI (2D) | wined3d (plain Wine) | menu, matches Windows (menu panel SSIM 0.942, menu text found) | runner, CI | needs FontSubstitutes in its profile: Wine stubs `GetFontResourceInfoW`, so its own font was replaced by Wine's default |
| Xonotic 0.8.6 (1.24 GB) | DarkPlaces | OpenGL | wined3d + `MACRON_OPENGL=llvmpipe` (software GL) | menu, matches Windows (SSIM 0.953, menu text found) | runner, CI | the runner has no hardware OpenGL: without the switch, `No matching GL pixel format available`; Mesa Zink needs robustness2 nullDescriptor, which MoltenVK lacks. Hardware GL on a real Mac not checked yet |
| SuperTuxKart 1.5 (922 MB) | Antarctica (Irrlicht fork) | Direct3D 9 (`--render-driver=directx9`) | wined3d, Vulkan renderer (`WINE_D3D_CONFIG=renderer=vulkan`) | menu, matches Windows (SSIM 0.971, menu text found) | runner, CI | DXVK d3d9 can't make a device (MoltenVK lacks geometryShader, shaderCullDistance); no d3dx9_43, so fixed pipeline on both sides |
| Direct3D 11 probe (ours, not a game) | none | Direct3D 11 | DXVK | draws its picture, matches Windows (SSIM 0.996) | runner | DXMT: device works, presenting needs a Wine that exports winemac functions |

## Licences

| Dependency | Licence | Used as |
|---|---|---|
| Wine (Gcenx/macOS_Wine_builds 11.21) | LGPL 2.1+ | downloaded at install time, unmodified |
| DXMT 0.80 (3Shain/dxmt) | LGPL 2.1+ | downloaded at install time, unmodified |
| DXVK-macOS 1.10.3-20230507 (Gcenx/DXVK-macOS) | zlib | downloaded at install time, unmodified (the `dxvk` backend) |
| Mesa 26.1.8 for Windows (pal1000/mesa-dist-win) | MIT | downloaded on Windows runners for OpenGL references, and by macron when `MACRON_OPENGL=llvmpipe` |
| Go standard library | BSD 3-Clause | built into macron and macron-check |
| Steam for Mac (Valve) | proprietary | installed by the `Steam hook` workflow to test against; never redistributed, the repo keeps only file hashes |

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
