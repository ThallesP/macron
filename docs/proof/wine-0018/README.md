# Patch 0018: Wine's own sound tests

`tools/wine-tests/sound.sh`, three alternating rounds on sandbox sbx_dzv7m2rb (three cores, a 48000 Hz virtual output device), 2026-11-20: "before" is the `winecoreaudio.so` of `runtime-wine-11.21-macron.16` (0016), "after" the one the Wine build workflow made on the `wine-0017` branch with 0018 (SHA-256 `5d181a39...41e41f`, run 44696404284), which is also the file the M3 ran.

`wine-tests-three-rounds.txt` has every line. `mmdevapi:render` and `dsound:dsound` pass in all six runs. `winmm:wave` fails the same two checks (`wave.c:1281`, `wave.c:1310`) in all six, as it does with Gcenx's driver (`docs/proof/wine-0016/`). `dsound:dsound8` fails `dsound8.c:1800` once with each driver (before-1, after-2): a timing check on a sandbox, not a difference.

What the patch changes is on the M3, in `runtime/wine/patches/README.md` ("0018").
