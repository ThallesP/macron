# v0.5.0-rc8, tested before publishing, and on the M3 after

`sbx_2voiamte/`: the published file, `Macron-0.5.0-rc8.dmg` of Release run 44722729979 (main at 6a87bdc, SHA-256 `82eeb33fba99e67906642594958ffea9caddebf71e70e408367c4482b7182eb5`), on a fresh sandbox.

- `gate3-all.log` and the `gate3-*` files: `ci/steam-play/gate3.sh` end to end, the same steps as rc7's, passed. Its screenshots stay on the sandbox's side (they show the test account's library).
- `after-gate3.txt`: the package installed again. The runtime it installs is `wine-11.21-macron.17-...`; `winemac.so` in the DXMT tree is SHA-256 `0d0827e0...` and `winecoreaudio.so` in both trees `5d181a39...`, ad hoc signed: the files Thalles's M3 measured (`runtime/wine/patches/README.md`, "0017" and "0018"). Then `tools/perf/sndload.c --policy sdl`, 30 s with every core busy, as shipped and with `MACRON_AUDIO_CLIENT=fixed`: the stream reports "timer and client in the real-time band" and "timer in the real-time band". The numbers are a sandbox's (three cores, a machine that stops for 100 ms now and then) and do not tell the two apart. The three "screen" runs in this file used a copy of the test program from before it had `--borderless` and say nothing; `after-screen.txt` replaces them.
- `after-screen.txt`: `tools/perf/gpuload.c --borderless 1` through `/usr/local/bin/macron`. As installed, Windows sees the sandbox's 1024x768 screen and 33 modes; with `MACRON_RENDER_PIXELS=442368` a 768x576 screen and 3 modes, the window still at 0,0 and 1024x768 points on the Mac, and clicks at 950,300, 700,650 and 1000,740 on the Mac arrive at 712,225, 525,487 and 750,555; with `MACRON_RENDER_PIXELS=0` as without it. Started as Steam app 1422450 (Deadlock's profile), the screen is the full one: a sandbox's chip is "Apple M1 (Virtual)", which is not one of the chips the profile names. That the profile's size reaches the game on a real one is in `internal/launch`'s tests and in the M3's first session with the profile (`docs/perf/latency.md`).

Nothing of the test account is in these files (checked against its tokens before committing).

## After publishing, on Thalles's M3 (not installed)

The published DMG (SHA-256 as above), unpacked with `pkgutil --expand-full` into `~/macron-perf/rc8/`, its `macron` run with `MACRON_HOME` set to a new folder: it fetched the runtime `wine-11.21-macron.17-dxmt-0.80-macron.2-...`, whose `winemac.so` and `winecoreaudio.so` are the two files named above. Three starts of Deadlock, 75 s each, thermal level 0 throughout, nothing clicked, no `MACRON_RENDER_PIXELS` in the environment for the first two (thread "Deadlock on the M3: the published rc8, unpacked and not installed, with nothing set by hand"):

| | macron's line | the game's `video.txt` | frames a second | start to GPU done |
|---|---|---|---|---|
| the title screen | `on an Apple M3 the game is shown a screen of about 780000 pixels, scaled up to fill the display (MACRON_RENDER_PIXELS=0 turns that off)` | 1098x711, scale 1.0 | 92.3 (89.3 to 93.8) | 19.1 ms |
| the practice map | the same | 1098x711, scale 1.0 | 76.7 (73.5 to 80.2) | 22.8 ms |
| the practice map, `MACRON_RENDER_PIXELS=0` | none | 1920x1243, scale 0.579 | 54.6 (52.0 to 55.9) | 32.7 ms |

The window is at 0,0 and 1920x1243 points in all three, the display's mode unchanged, the picture edge to edge with nothing cut off; text is soft in the first two and sharp in the third. Every sound report says "timer and client in the real-time band" and 0 frames missing. The installed Macron on that Mac was not touched (the times and sizes of all its files before and after, and its package receipt).
