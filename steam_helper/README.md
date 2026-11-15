# `steam.exe` (Proton's `steam_helper`)

Steam starts a Windows game through Macron, and Macron starts it through this program, as Proton does on Linux. In the game's Windows environment it plays the part of the running Steam client: it writes `HKCU\Software\Valve\Steam\ActiveProcess\pid` (so `SteamAPI_IsSteamRunning()` finds a live process), asks the Steam bridge (`lsteamclient`'s `steamclient_init_registry`) to write the user's language and the app's keys, writes `steamapps\libraryfolders.vdf`, creates the events and windows some games look for, starts the game, and returns its exit code. `steam.exe steam://...` hands the URL to the Mac client.

It is Proton's `steam_helper/steam.c` (ValveSoftware/Proton, tag `proton-11.0-2e`, commit `bc896217ffcd67f892f952b010f898fe7bd0f0a1`), fetched at build time by `../lsteamclient/fetch.sh` and built by `../lsteamclient/build.sh`. Proton 11's helper is plain C and includes no Steamworks SDK header.

## Licence

BSD 3-Clause, Valve's (`LICENSE` here is Proton's `LICENSE.proton`, which `fetch.sh` checks against Valve's).

## What Macron changes

`patches/0001-steam_helper-Forward-to-the-Mac-Steam-client.patch`:

- `steam.exe steam://...` runs `/usr/bin/open steam://...`, and options (`steam.exe -foo`) run `/usr/bin/open -a Steam --args -foo`, instead of Linux's `steam-runtime-steam-remote` or `steam`. macOS only passes `--args` to Steam when it is not running yet.
- no SteamVR setup: there is no `vrclient` on a Mac, and Proton's helper only logged that it was missing.
