# Installing Macron

Macron makes Steam's own Install and Play buttons work for Windows-only games in Steam for Mac, on Apple silicon. It comes as `Macron-<version>.dmg`, which holds `Macron.pkg`.

## After installing, in short

1. Open Steam. A Windows-only game now has a working Install button: install one and press Play. The installer also prepares the Windows environment new games start from (about two more minutes on the test machines); a new game then takes a few seconds more than usual on its first start, and games you already played start as usual even after a Macron update.
2. To log its frame rate, turn the frame counter on before pressing Play, and off again afterwards:
   ```sh
   touch ~/Library/Application\ Support/Macron/show-fps   # on
   rm ~/Library/Application\ Support/Macron/show-fps      # off
   ```
3. Then, with the game closed, make the report and paste it back (into a chat or an issue):
   ```sh
   macron diagnose
   ```
   It takes a few minutes, prints the report and keeps it in `~/Library/Application Support/Macron/logs/diagnose-<time>.txt`. It sends nothing anywhere, and games run exactly the same whether you ever run it or not. What it holds: the macOS version and chip, whether SIP is on, the Steam build and whether Macron knows it, whether Steam Play came on (the end of `steamplay.log` and of the install log), the runtime and Steam bridge versions, what one call from a game into Wine and macOS costs on this Mac (it runs small timing programs that come with Macron), and, for each game started with the frame counter on, its backend, its frame rates and, for Direct3D 10 and 11 games, how many frames took longer than 50 and 100 ms. It leaves out your Steam account (no account name, Steam ID, library or Steam settings) and writes `~` and `<user>` instead of your home folder and user name.

## What changed for speed, and what is not known yet

Ultimate Custom Night ran at 5 frames a second in its menu on Thalles's M3 with 0.4.0. Two causes, both measured there:

- Apple's OpenGL on Apple silicon is built on Metal, and every time a game's vertices were uploaded into a buffer the GPU was still drawing from, it sent its work to the GPU and waited, about 2,000 to 4,000 times a frame. This build writes those uploads without waiting (Wine patch 0009): the menu went from 5 to 31.
- Then the game's own thread did all of Wine's Direct3D work too, and three times per draw it waited for itself. This build maps the game's system memory buffers without that wait (0010) and runs Wine's Direct3D work on a second thread (0011): in a test build on the M3 the menu ran at 60, the game's frame rate, using nearly two cores.

A night (about 1.7 times the menu's work) has not been measured on a real Mac yet. Turn the frame counter on (below) and `macron diagnose` reports it.

If a game draws garbage, flickers or stutters with this build, these go in its Steam launch options, one at a time, to find which change it is: `WINE_D3D_CONFIG=csmt=0 %command%` (Direct3D work back on the game's thread), `WINE_OPENGL_UNSYNCHRONIZED=0 %command%` (the old uploads), `WINE_OPENGL_BATCH=0 %command%` (no batching of OpenGL calls). Direct3D 10 and 11 games go through DXMT and are not affected by any of them.

## Starting games

A game's first Play after a Macron update no longer waits for a Windows setup step: it starts as quickly as any other start (on the test machines the first start after an update took 7.7 times an ordinary start before this build, and the same as an ordinary start now). A new game starts from a prepared copy instead of building its own Windows environment: about 1.2 to 1.5 times an ordinary start, where it took about 11 times before. The "Wine configuration is being updated" window no longer appears. While a game runs, the Dock, the menu bar and Force Quit still call it `wine`. `docs/startup/README.md` in the source has the measurements.

## Before you start

Install Steam from <https://store.steampowered.com/about/> into `/Applications`, start it once so it finishes updating itself, and sign in. Steam may stay open: the installer closes it and opens it again.

## The package is not notarised

Macron has no Apple Developer ID yet, so the package is unsigned and cannot be notarised. macOS 26 therefore refuses to open it with a double-click: "“Macron.pkg” Not Opened. Apple could not verify “Macron.pkg” is free of malware that may harm your Mac or compromise your privacy." Pick one of these ways past that:

- **Terminal, installing directly (the shortest).** Open the DMG and run `sudo installer -pkg "/Volumes/Macron <version>/Macron.pkg" -target /`. The `installer` command does not ask Gatekeeper.
- **System Settings.** Double-click `Macron.pkg` and press Done on the warning. Open System Settings > Privacy & Security, scroll to Security, where it says "Macron.pkg" was blocked, and press Open Anyway, then confirm with your password or Touch ID. The installer opens.
- **Terminal, removing the quarantine.** Before opening the DMG, run `xattr -dr com.apple.quarantine ~/Downloads/Macron-<version>.dmg`, then open the DMG and double-click `Macron.pkg`.

Control-click > Open, which used to skip the check, no longer does since macOS 15: it shows the same warning.

## What the install does

1. Copies `macron` and `macron-uninstall` to `/usr/local/bin` and the Steam Play switch library to `/usr/local/lib/macron`.
2. On an Apple silicon Mac that has no Rosetta 2 yet (a new Mac has none), installs it from Apple with `softwareupdate --install-rosetta --agree-to-license`, which accepts Apple's Rosetta licence for you. Wine is an Intel program and cannot start without it.
3. As you, not as root, runs `macron runtime install`: downloads Wine, DXMT, DXVK and the Steam bridge at pinned versions (about 265 MB) and unpacks them into `~/Library/Application Support/Macron` (about 2 GB as Finder counts it, less on disk because the second Wine tree is an APFS clone).
4. Closes Steam if it is open, then runs `macron steam enable`:
   - adds Macron to `~/Library/Application Support/Steam/compatibilitytools.d` and makes it the compatibility tool for every app in Steam's `config.vdf`, after backing that file up;
   - switches on the Steam Play code that ships, switched off, in Steam for Mac. For that it puts its library into `/Applications/Steam.app`, names it in the app's `Info.plist`, and signs Steam.app again ad hoc, because macOS only loads a library into an app whose signature allows it. Valve's three files that this changes (`Info.plist`, `steam_osx`, `CodeResources`) are kept in `~/Library/Application Support/Macron/steam-app-backup` and go back byte for byte on uninstall. `steamplay/README.md` in the repository has the details.
5. Opens Steam again if it was open.

Because Steam.app's signature changes, macOS forgets Steam's Accessibility and input-monitoring permissions and asks again the next time Steam wants them (they are for controllers through Steam Input). If macOS asks whether the installer or Terminal may modify apps (App Management), allow it; if it blocked the change, the log says so and `macron steam enable` in Terminal tries again.

If Rosetta or the runtime does not install (no internet, a full disk), the install undoes itself and Installer says it failed. If only the Steam step does not finish (macOS blocked the change to Steam.app under App Management, Steam is a build Macron does not know yet, or Steam would not quit), Macron stays installed and Steam Play is simply not on yet. Either way the installer opens `/Users/Shared/Macron install log.txt` in TextEdit: what went wrong, what to do next, and the log of that run, ready to send to whoever helps you. To finish the Steam step, quit Steam and run `macron steam enable` in Terminal; if that says "Operation not permitted", turn on Terminal under System Settings > Privacy & Security > App Management and run it again. Every install run is also appended to `/Library/Logs/Macron-install.log`.

## Using it

Open Steam. A Windows-only game in your library now has a working Install button; Steam downloads it into its normal library, and Play starts it through Macron. A new game's first start takes a few seconds more than later ones: it gets a copy of the Windows environment the installer prepared, in under a second, and Macron writes the registry keys the Steam bridge needs. Steam's Stop button stops it. Games that have a Mac version install and start exactly as before.

- **A log of every start** is in `~/Library/Application Support/Macron/logs/steam-<app id>/`.
- **Frame rate:** create the file `~/Library/Application Support/Macron/show-fps` (`touch` it) and start the game. Direct3D 8 and 9 games then log their frame rate into that log (`trace:fps:… @ approx 59.9fps`), which `macron diagnose` sums up; Direct3D 10 and 11 games show Apple's Metal HUD over the game and write every frame's interval into that log, which `macron diagnose` turns into the average and the hitches. Delete the file to turn it off.
- **When Steam updates itself** to a build this Macron does not know, Steam Play goes off (Windows-only games show a greyed Install again) and nothing else changes; `~/Library/Application Support/Macron/logs/steamplay.log` says `UNKNOWN_STEAM_BUILD`. Install the newest Macron. If an update replaces Steam.app itself, run `macron steam enable` once with Steam closed.
- **Games that talk to Steam** (ownership, achievements, Steam Cloud, the Steam profile, through `SteamAPI_Init`) reach your running Mac Steam through the Steam bridge, a Mac port of Proton's `lsteamclient`: macron starts them through a small `steam.exe` in their Windows environment, and the launch log says `starting it through steam.exe`. Steam must be running and signed in, as for any game. The bridge stays under Valve's licences (Steamworks SDK License, BSD); Valve has not confirmed that redistributing it is allowed (ValveSoftware/Proton issue #7801).

## Uninstalling

Run in Terminal:

```sh
sudo macron-uninstall
```

It closes Steam, then runs `macron uninstall` as you: Valve's three files in Steam.app go back byte for byte (Steam.app is notarised again, `codesign --verify --strict /Applications/Steam.app` passes), Macron's library leaves Steam.app, Steam's `config.vdf` mapping goes back, Macron's folder leaves Steam's `compatibilitytools.d`, and Macron's runtimes, Windows environments and logs are deleted from `~/Library/Application Support/Macron`, each path listed first. It never deletes anything Macron did not create. Then it deletes `/usr/local/bin/macron`, `/usr/local/bin/macron-uninstall`, `/usr/local/lib/macron` and the install log. macOS will again ask for Steam's Accessibility permission when Steam next wants it.

Games you installed stay in Steam's library, with their Windows environments next to their saves (`steamapps/compatdata/<app id>/pfx`); uninstall them from Steam like any other game.

## What was checked, and what was not

Checked on GitHub's Mac machines (macOS 26, Apple silicon, a virtual GPU) with a Steam test account, through Steam's real window (`ci/steam-play/gate3.sh`), and, for the Steam bridge, with Valve's own Steamworks example app (`SteamAPI_Init`, an achievement stored and read back, a Steam Cloud file written and read back) and Overwatch, which reaches its Battle.net sign-in screen only with the bridge: Steam from Valve's DMG, signed in; this installer; Ultimate Custom Night (Windows-only, free) installed with Steam's Install button and started with Play; Stop and quitting return Steam to Play; a Mac-native game runs as before; `sudo macron-uninstall` leaves Steam.app byte for byte as Valve shipped it.

Not checked, because those machines cannot show it:

- **A Mac with System Integrity Protection on**, which is every normal Mac. The test machines have it off, where macOS is more lenient about loading a library into an app. Macron does what Apple's rules ask for on a normal Mac (the ad hoc signature), but the first install on one is yours. If Windows-only games still show a greyed Install afterwards, `~/Library/Application Support/Macron/logs/steamplay.log` and `/Library/Logs/Macron-install.log` say why.
- **Speed on a real Apple GPU.** The test machines have no hardware OpenGL, which is what Ultimate Custom Night's Direct3D 9 goes through, so its frame rate there says little about your Mac. The frame counter above shows it.
