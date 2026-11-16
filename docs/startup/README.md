# Starting a game: where the time goes, and Wine out of sight

## In two minutes

Thalles installed v0.5.0-rc3. On his first Play of Overwatch, Wine's "The Wine configuration in … is being updated, please wait…" window sat on screen for 36 seconds. That was wineboot reinstalling Wine's setup into the game's Windows environment (its prefix). Macron ran it whenever the runtime changed, even when Wine itself needed no update. Four changes, all in v0.5.0-rc4:

1. **No update when Wine needs none.** wineboot records in the prefix (`.update-timestamp`) the modification time of the `wine.inf` it installed. After a Macron update, if that record still matches the runtime's `share/wine/wine.inf` and the backend is the same, Macron skips wineboot. It only copies DXVK's DLLs again (for DXVK games) and rewrites its record. Wine loads its own modules from the runtime, not from the prefix (ntdll's loader takes the runtime's builtin DLL even when a copy is in `system32`), so the prefix needs nothing else. Every overlay from macron.5 to macron.13 keeps Gcenx's `wine.inf` and its time stamp 1793450195. An overlay that needed a real update would ship a changed `wine.inf`, and that changes the time stamp.
2. **New games start from a template.** Per runtime and Wine tree, Macron makes one prefix, once: under `<runtime>/prefix-templates/<tree>`, by `macron runtime install` or on the first game that needs it. A new game's prefix is an APFS clone of it (`cp -c`, a plain copy where cloning fails). The clone matches a freshly made prefix: the same 1,802 files at the same sizes, and the same registry, apart from three IDs that wineboot draws at random (below).
3. **The window never shows.** Wine patch 0013 stops wineboot from creating its wait window, for a new prefix, for Wine's own automatic update and for Macron's `wineboot -u`. Without the window, wineboot never becomes a Dock app.
4. **Time stamps.** Each `macron:` line of a game's log now starts with the UTC time and the seconds since macron started, and three new lines mark `prefix ready`, `Steam bridge ready` and `game process started`.

On a sandbox, measured as the time from Steam's command to the game's `main()` (sandbox seconds; their proportions carry over to a Mac, the seconds do not):

| Start | rc3 | rc4 | rc4 as a share of an ordinary start |
|---|---|---|---|
| Ordinary (prefix current) | 5.4, 5.9, 5.5 s | 5.6, 5.2, 5.4 s | 1 |
| First start after a Macron update | 41.5 s | 5.3 s | 1.0 (was 7.7) |
| A new game | 58.7 s | 6.4, 8.4 s | 1.2 to 1.5 (was 10.8) |

On Thalles's M3 the first start after an update took 36 and 38 s and a new game 13 to 14 s. With rc4 the first should cost what an ordinary start costs, and a new game an ordinary start plus a few seconds. That is a projection from the sandbox's proportions, not a measurement on the M3.

What is left of an ordinary start is Wine's: about 60% is Wine bringing up its server and system services, about 30% is the Steam bridge's `steam.exe`, about 8% is the game loading up to `main()`, and 2% is Macron. Nothing there is cheap to remove ("What is left").

Wine still shows its name while a game runs in the Dock, the menu bar and the Force Quit window, and may in macOS's permission and crash dialogs. They read the name `wine`, the name of Wine's loader program. Changing it takes either a Wine patch to ntdll or every game running from an app bundle ("Where a player sees Wine").

Evidence: `docs/proof/startup-sbx_ngcpx472/`. Scripts: `tools/startup/`.

## How it was measured

Sandbox sbx_ngcpx472 (macOS 26.7, Apple M1 virtual, 3 cores, no Steam running). `tools/startup/startup.c` is a 64-bit Windows program that writes the Unix time at the start of its `main()` into a file next to itself (its output does not reach macron's log when `steam.exe` starts it), shows a window for 2 s and exits. `tools/startup/m.sh` starts it the way Steam does: `SteamAppId=<n> STEAM_COMPAT_DATA_PATH=… macron waitforexitandrun startup.exe 2`. It prints the time to `main()` and to macron's exit. `seq.sh` ran the steps below in order, with rc3's macron (built from 8285127) and this branch's, on the same runtime (macron.11). The update was simulated by renaming the runtime in the prefix's `.macron-prefix.json`, which is what a Macron update with a new runtime looks like to both builds. `watch.sh` recorded, every 0.3 s, the window list and the apps macOS counts as Dock apps (`lsappinfo`, type Foreground).

`seq.log`:

```
rc3-new-prefix               to main()  58.67 s  to macron exit  66.13 s
rc3-ordinary-1/2/3           to main()   5.46 / 5.90 / 5.36 s
rc3-after-update             to main()  41.53 s
new-ordinary-1/2/3           to main()   5.63 / 5.21 / 5.44 s
new-after-update             to main()   5.27 s   ("needs no update for this runtime", no wineboot)
macron runtime install (both templates)  105.6 s
new-new-game-1 / -2          to main()   6.35 / 8.40 s   (clone 0.33 / 0.58 s)
new-boot-in-place            to main()  59.46 s   (a prefix folder that already had a file in it)
```

macron's own lines for a new game from the template (`new-new-game-1-macron-lines.txt`):

```
macron: 13:29:24.297Z +0.00s game steam-1002, log …
macron: 13:29:24.629Z +0.33s made the Windows environment at …/compat/1002/pfx from the runtime's template
macron: 13:29:24.629Z +0.33s prefix ready
macron: 13:29:24.674Z +0.38s writing the registry keys games read to find Steam
macron: 13:29:26.888Z +2.59s Steam bridge ready
macron: 13:29:26.889Z +2.59s game process started (Wine process 32432)
```

and for an ordinary start, where macron's part ends at 0.09 s:

```
macron: 13:26:47.058Z +0.00s game steam-1001, log …
macron: 13:26:47.059Z +0.00s prefix ready
macron: 13:26:47.148Z +0.09s Steam bridge ready
macron: 13:26:47.150Z +0.09s game process started (Wine process 25352)
```

## 1. No update when Wine needs none

`internal/launch/prefix.go`, `EnsurePrefix`. A prefix with Macron's record for the same runtime and backend is used as it is, as before. Otherwise:

- record present, same backend, `.update-timestamp` equal to the mtime of `<tree>/share/wine/wine.inf`: no wineboot (the line `the Windows environment at … needs no update for this runtime`);
- prefix folder missing or empty: a clone of the template (part 2);
- anything else (another backend, another `wine.inf`, a folder with other files in it): `wineboot -u` as before.

Why skipping is safe: when a program loads a DLL that Wine builds, ntdll finds the copy in `system32`, sees Wine's builtin mark and loads the runtime's own file from `lib/wine` (`LO_BUILTIN_NATIVE` in `dlls/ntdll/loader.c`). The prefix's copies of Wine's modules are stubs, whatever runtime made them. What `wineboot -u` would add on a new runtime is what a new `wine.inf` adds, and Wine itself uses the same test (`update_timestamp` in `programs/wineboot/wineboot.c`) to decide whether to update a prefix when it starts.

The rule holds for CI's runtimes: the overlays (`runtime/wine/build-overlay.sh`) never ship a `wine.inf`, so every runtime keeps Gcenx's file with the time stamp 1793450195 (macron.5, .6 and .11 checked in both trees on the M3 and the sandboxes; macron.13's archive has no `share/` file but its patches and licence). Patch 0013 changes `wineboot.exe`, not `wine.inf`. An overlay that needs a prefix updated has to ship a changed `wine.inf`, which carries a new time stamp. Tests: `TestANewRuntimeWithTheSameWineInfDoesNotUpdateThePrefix`, `TestANewRuntimeWithAnotherWineInfUpdatesThePrefix`.

## 2. New games from a template

`ensureTemplate` makes `<runtime>/prefix-templates/<tree>`: wineboot in a fresh folder next to it, Macron's private user folders, then a rename into place, so a game started meanwhile never clones half a template. `clonePrefix` copies it to the game's prefix with `cp -c -R`, which clones each file on APFS (the copy shares the template's blocks until either side changes). Where that fails (another volume, another file system) it falls back to a plain `cp -R`. The per-game steps follow as before: DXVK's DLLs, Macron's record, the profile's registry values and the Steam bridge's.

`macron runtime install` now makes both templates (DXMT tree and plain tree; on the sandbox 105.6 s for the two). Macron's installer does not run it, so on a Mac the first new game of a runtime makes its template (about as long as a new prefix took before) and every later one clones it. A game whose prefix already exists never needs the template. Templates live in the runtime's folder and go with it, and `sudo macron-uninstall` removes them with Macron's folder.

**Clone against fresh** (`clone-vs-fresh.txt`, from `tools/startup/cmp.sh`: game 1002 cloned, game 1004 booted in place, both after a start):

- files and links: 1,804 in the clone, 1,805 in the fresh one. The extra one is `keep.txt`, the file the test put in 1004's folder to make Macron boot it in place. The other 1,802 files have the same path, type and size.
- `user.reg`, `userdef.reg`: identical apart from time stamps.
- `system.reg`: identical apart from time stamps and three values that wineboot draws at random for each new prefix: `MachineGuid`, `MachineId` and a display adapter's `VideoID`. With clones, every game of a runtime sees the same values, which is how one Windows PC looks to its games.

Games run from it: every start in "How it was measured" after the template existed ran in a clone. In the release test, Ultimate Custom Night and the bridge's probe also ran in clones (`docs/proof/release-0.5.0-rc4/`). Tests: `TestANewGameIsClonedFromTheRuntimesTemplate`, `TestAPrefixInAFolderWithFilesIsBootedInPlace`.

## 3. The dialog never shows

Wine patch 0013 (`runtime/wine/patches/0013-wineboot-Don-t-show-the-wait-window.patch`, overlay `runtime-wine-11.21-macron.13`). `update_wineprefix` no longer creates the wait window. The install runs as before, in the same message loop. winemac makes a process a regular Dock app only when one of its windows is shown (`transformProcessToForeground` in `cocoa_app.m`), so without the window wineboot never shows up in the Dock, the menu bar or the Force Quit list. `MACRON_WINEBOOT_WAIT_WINDOW=1` brings the window back, which the check uses as its control. The overlay ships the patched `wineboot.exe` (x86_64) in both Wine trees. `check-pe-imports.sh` checks on every build that it imports exactly what Gcenx's does (134 functions).

`tools/startup/dialog.sh`, sbx_ngcpx472, the runtime's own `wineboot.exe` and then the overlay's, each for a new prefix, Wine's automatic update (`.update-timestamp` set to 0, then any program) and a forced `wineboot -u`. Screenshots every second, and the window list and Dock apps every 0.3 s (`*-watch.txt`):

| Case | Gcenx's wineboot | Patch 0013's |
|---|---|---|
| New prefix (71 s / 67 s) | window "Wine" (wine glass, "The Wine configuration in … is being updated"), Dock app `wine` | no window, no Dock app |
| Automatic update (41 s / 37 s) | the same | none |
| Forced `wineboot -u` (35 s / 32 s) | the same | none |
| Control: 0013 with `MACRON_WINEBOOT_WAIT_WINDOW=1` (38 s) | | the window and the Dock app are back |

`gcenx-forced-update-10.png` and `macron13-forced-update-10.png` are the same moment of the same update (the prefix's path is blacked out). The first has the dialog and an extra generic program icon in the Dock; the second has only the desktop.

With parts 1 and 2, wineboot also runs far less often: once per runtime and tree for the template, and on an existing prefix only when `wine.inf` changes or the backend does.

## 4. Where an ordinary start's time goes

`tools/startup/parts.sh`, the same prefix, current, three rounds (`parts.log`):

| | to `main()` |
|---|---|
| The program alone, Wine not running (cold) | 3.48, 2.72, 2.90 s |
| Through the bridge's `steam.exe`, cold (what Play does) | 4.79, 4.77, 4.54 s |
| The program alone, Wine's server and services already running (warm) | 0.43, 0.35, 0.37 s |

So of an ordinary start (about 4.7 s here, 5.4 s through macron):

- **Wine's server and system services, about 60%.** A cold start brings up `wineserver`, then `services.exe`, two `winedevice.exe`, `plugplay.exe`, `svchost.exe` and `rpcss.exe` (`process-trace.txt`). With all of them already running the program reaches `main()` in 0.4 s.
- **The bridge's `steam.exe`, about 30%** (1.3 to 2 s): it sets up the Steam client library before it starts the game. Measured with no Steam running on the sandbox; with Steam running it may be shorter or longer.
- **The program up to `main()`, about 8%.**
- **Macron, about 2%** (0.09 to 0.11 s).

Fixed because it was cheap:

- **The server's linger.** Wine's server stays 3 s after its last program exits (`master_socket_timeout` in `server/main.c`), and `wineserver -w` waits for that: 3.15 s measured, against 0.16 s for a server started with `-p0`. Macron waited for it after wineboot, after each registry import and after each extra install, so a new game paid it two or three times. Those steps now start the server with `-p0` first (`startServerWithoutLinger`). An ordinary start never waited for it.
- **winemenubuilder.** Wine starts `winemenubuilder.exe -a -r` on every cold start, to turn the shortcuts and file types that Windows programs register into Mac menu entries under Wine's name. It made none on the sandbox. Macron now disables it along with `winedbg`, `mscoree` and `mshtml`.

What is left, with what it would take:

- **Keep Wine running between games.** A warm start is 0.4 s against about 3 s. That means a server that stays up after the game quits (memory, and Steam must still see the game as stopped), or one started when Steam starts. It changes how Steam tracks a game, so it is not a cheap change.
- **Fewer services.** `winedevice.exe` carries game controllers (winebus) and `plugplay.exe` device arrival. Dropping them would break input, so no.
- **`steam.exe`.** About 30%, and needed for any game that talks to Steam. Worth timing on a Mac with Steam running before touching it.
- **After the game quits**, macron waits for Wine's server to exit (`waitforexitandrun`), 4.2 s on the sandbox, 3 s of it the linger. Steam shows the game as running for that long after its window closes. Starting the game's own server with `-p0` would cut 3 s of it. It is not in rc4 because every game's quit would go through it; it should be tried on its own.

## Where a player sees "Wine"

Playing through Steam, with rc4:

| Where | What it shows | State |
|---|---|---|
| wineboot's wait window (new prefix, Wine's automatic update, `wineboot -u`) | "Wine", the wine glass, "The Wine configuration in … is being updated", plus a Dock icon | **gone** (patch 0013), and wineboot rarely runs (parts 1 and 2) |
| The Dock, while a game runs | the game's own icon (winemac takes it from the `.exe`), named `wine` (`lsappinfo` on the sandbox: `"wine"`) | Needs Thalles |
| The menu bar, while a game is in front | the application menu reads `wine`; its items read Hide, Hide Others, Show All, Quit | Needs Thalles |
| Force Quit (⌥⌘⎋) | `wine` | Needs Thalles |
| macOS's permission prompts (microphone, camera, local network, input monitoring) and its crash report if a Wine process crashes | the program's name, `wine`, or the app macOS holds responsible for it (Steam); not seen on the sandbox, which asked for nothing | Needs Thalles |
| With the game-mode switch on | the loader runs from `Macron Game.app`, whose `CFBundleName` (the name the application menu and its Hide and Quit items use) was "Wine" and is now **"Macron"**; macOS's local-network prompt already called it "Macron Game" | fixed in rc4 (not looked at on a screen yet) |
| Activity Monitor | processes `wine`, `wineserver`, and the Windows programs by name | left as it is: a tool for looking under the hood |
| Wine's crash dialog ("Program Error") | | never shown: `winedbg.exe` is disabled |
| Wine's Mono and Gecko install prompts | "Wine Mono Installer", "Wine Gecko Installer" | never shown: `mscoree` and `mshtml` are disabled |
| Menu entries and file types made by winemenubuilder | entries under Wine's name | never made: disabled in rc4 |
| Steam's own windows | the compatibility tool is listed as "Macron"; nothing of Wine | nothing to do |
| macron's log and `macron diagnose` | Wine's output and the runtime's name | for whoever reads logs; not shown while playing |

What the four "Needs Thalles" rows would take. They share one cause: every Windows process, the game included, runs Wine's loader program `wine`, and macOS names an app after its bundle, or after its program when there is no bundle.

- **Name it "Macron" for every game:** run the loader from an app bundle named Macron at all times, the mechanism the game-mode switch already uses (`internal/wineruntime/gamemode.go`), without the games category. Small in code. But macOS keys permissions (microphone, local network) to the bundle, so each would be asked again once, and it changes how every game is started. His call.
- **Name it after the game** ("Overwatch" in the Dock): one bundle per game, named from Steam's `appmanifest_<id>.acf`. Wine starts every new process with the loader path that ntdll computes from where `ntdll.so` lives (`init_paths` in `dlls/ntdll/unix/loader.c`), not from the environment, so it also needs a Wine patch letting Macron choose that path, and `ntdll.so` in the overlay (checked as a drop-in like `opengl32.so`). About a day's work with tests on a sandbox; two games of the same runtime running at once would each keep their own name.
