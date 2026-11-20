# v0.5.0-rc9, tested before publishing

`sbx_thj2oyif/`: the published file, `Macron-0.5.0-rc9.dmg` of Release run 44809469105 (main at 879d7f6, SHA-256 `d81d7223848d3c4b31c77646d3529644bc8753f203dbb4890a4c9d286163ae90`), on a fresh sandbox.

- `gate3-all.log` and the `gate3-*` files: `ci/steam-play/gate3.sh` end to end, the same steps as rc8's, passed. Its screenshots stay on the sandbox's side (they show the test account's library).
- `after-gate3.txt`: the package installed again. The runtime it installs is `wine-11.21-macron.18-...`; `wined3d.dll` is SHA-256 `56e069cf...` in both Wine trees (the build of patch 0019 from main), and `winemac.so` and `winecoreaudio.so` are rc8's. Through `/usr/local/bin/macron`:
  - `tools/perf/d3d9ver.c` (what a game reads as the display driver's version): without the `dlss` switch `"NVIDIA GeForce 6800" ... version 32.0.13.783`, with it `"NVIDIA GeForce RTX 4090" ... version 32.0.15.9144`, and macron's line that DXMT answers as an NVIDIA GPU. rc8 answered 31.0.13.783 either way, which Deadlock refuses as out of date when it is told the card is NVIDIA's.
  - `tools/perf/gpuload.c --borderless 1` started as Steam app 1422450 (Deadlock's profile): Windows sees the sandbox's whole 1024x768 screen and all its 33 modes, and macron prints nothing about a smaller screen. The profile no longer asks for one.

The file of patch 0019 that Thalles's M3 ran (`dlss-test1`, SHA-256 `b5d9c78d...`) was the same source built on the test branch; the one in this release was built on main. What it did there is in `runtime/wine/patches/README.md`, "0019".

Nothing of the test account is in these files (checked against its tokens before committing).
