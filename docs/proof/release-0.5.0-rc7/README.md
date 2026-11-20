# v0.5.0-rc7, tested before publishing

`sbx_z6tfpkor/`: the published file, `Macron-0.5.0-rc7.dmg` of Release run 44672933648 (main at 874e873, SHA-256 `7281e0db1e3799f8e9f20d88803a2c809eeacfb7b0df0ba9e99a92876412f02b`).

- `gate3-all.log` and the `gate3-*` files: `ci/steam-play/gate3.sh` end to end on a fresh sandbox, the same steps as rc6's, passed. Its screenshots stay on the sandbox's side (they show the test account's library).
- `after-gate3.txt`: the package installed again and, through `/usr/local/bin/macron`, what rc7 adds. The runtime it installs is `wine-11.21-macron.16-...`; `winecoreaudio.so` in both Wine trees is SHA-256 `3be2a877...`, ad hoc signed, which is the file Thalles's M3 measured (`docs/proof/wine-0016/m3/README.md`). Then `tools/perf/sndload.c --policy sdl`, 30 s with every core busy, in each of the driver's modes.

What those four runs show is that the installed driver runs in each mode and reports. Their numbers are a sandbox's: three cores, a 48000 Hz virtual sound device, and a machine that stops for 100 to 190 ms now and then (the client's longest wait for a wake-up was 106 to 195 ms in every run, whatever the mode). Frames CoreAudio asked for and did not get: as upstream has it, 86,482 of 1,413,323 in 246 gaps; as shipped, 5,660 in 3 gaps, all in the first 15 s while the reserve grew to its limit, and none after; a 48000 Hz stream as shipped, 1,926 in 1 gap; with the timer left in the application band, 1,231 in 3 gaps. Scenarios run 44672916510 is green on 874e873.
