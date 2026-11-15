# Installing Macron

Macron makes Steam's own Install and Play buttons work for Windows-only games in Steam for Mac, on Apple silicon. It comes as `Macron-<version>.dmg`, which holds `Macron.pkg`.

## After installing, in short

1. Open Steam. A Windows-only game now has a working Install button: install one and press Play. The first start of a game takes about a minute longer while its Windows environment is made.
2. To log its frame rate, turn the frame counter on before pressing Play, and off again afterwards:
   ```sh
   touch ~/Library/Application\ Support/Macron/show-fps   # on
   rm ~/Library/Application\ Support/Macron/show-fps      # off
   ```
3. Then, with the game closed, make the report and paste it back (into a chat or an issue):
   ```sh
   macron diagnose
   ```
   It takes about two minutes, prints the report and keeps it in `~/Library/Application Support/Macron/logs/diagnose-<time>.txt`. It sends nothing anywhere, and games run exactly the same whether you ever run it or not. What it holds: the macOS version and chip, whether SIP is on, the Steam build and whether Macron knows it, whether Steam Play came on (the end of `steamplay.log` and of the install log), the runtime and Steam bridge versions, what one call from a game into Wine and macOS costs on this Mac (it runs small timing programs that come with Macron), and, for each game started with the frame counter on, its backend and frame rates. It leaves out your Steam account (no account name, Steam ID, library or Steam settings) and writes `~` and `<user>` instead of your home folder and user name.

## What is still expected to be slow

Ultimate Custom Night, and other games that make many small draws a frame through Direct3D 9 or OpenGL, are expected to stay slow in this release. A night in Ultimate Custom Night makes about 32,000 OpenGL calls a frame, and every call from a Windows game into macOS goes through Wine and Rosetta: on the test machines (virtual Macs) each one costs about 14 µs, which alone is about half a second a frame. The fixes in the previous release (Wine patches 0003 to 0006) removed other costs and crashes but not this one. Whether a real Mac pays as much per call is exactly what `macron diagnose` measures, and it decides the fix (sending many OpenGL calls in one go), which is being worked on. Direct3D 10 and 11 games go through DXMT, which makes far fewer calls a frame.

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
2. As you, not as root, runs `macron runtime install`: downloads Wine, DXMT, DXVK and the Steam bridge at pinned versions (about 265 MB) and unpacks them into `~/Library/Application Support/Macron` (about 2 GB as Finder counts it, less on disk because the second Wine tree is an APFS clone).
3. Closes Steam if it is open, then runs `macron steam enable`:
   - adds Macron to `~/Library/Application Support/Steam/compatibilitytools.d` and makes it the compatibility tool for every app in Steam's `config.vdf`, after backing that file up;
   - switches on the Steam Play code that ships, switched off, in Steam for Mac. For that it puts its library into `/Applications/Steam.app`, names it in the app's `Info.plist`, and signs Steam.app again ad hoc, because macOS only loads a library into an app whose signature allows it. Valve's three files that this changes (`Info.plist`, `steam_osx`, `CodeResources`) are kept in `~/Library/Application Support/Macron/steam-app-backup` and go back byte for byte on uninstall. `steamplay/README.md` in the repository has the details.
4. Opens Steam again if it was open.

Because Steam.app's signature changes, macOS forgets Steam's Accessibility and input-monitoring permissions and asks again the next time Steam wants them (they are for controllers through Steam Input). If macOS asks whether the installer or Terminal may modify apps (App Management), allow it; if it blocked the change, the log says so and `macron steam enable` in Terminal tries again.

If Steam is a build Macron does not know yet (Steam updates itself), or any step fails, the install undoes itself and the log says why. Everything the install does is written to `/Library/Logs/Macron-install.log` (open it with Console or `cat`).

## Using it

Open Steam. A Windows-only game in your library now has a working Install button; Steam downloads it into its normal library, and Play starts it through Macron. The first start of each game takes about a minute longer while its Windows environment is made. Steam's Stop button stops it. Games that have a Mac version install and start exactly as before.

- **A log of every start** is in `~/Library/Application Support/Macron/logs/steam-<app id>/`.
- **Frame rate:** create the file `~/Library/Application Support/Macron/show-fps` (`touch` it) and start the game. Direct3D 8 and 9 games then log their frame rate into that log (`trace:fps:… @ approx 59.9fps`), which `macron diagnose` sums up; Direct3D 10 and 11 games show Apple's Metal HUD over the game. Delete the file to turn it off.
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
