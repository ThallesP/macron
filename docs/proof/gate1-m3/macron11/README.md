# Request U4: Wine patch 0011 on the real GPU (Thalles's MacBook Air M3), 2026-11-16 09:38 UTC

`macron` built from branch `wine-0011` at 55feea6, runtime `wine-11.21-macron.11` in its own `MACRON_HOME`. Ultimate Custom Night started as Steam starts it with no input (its menu), an empty workspace in front, sound off.

| start | menu, frames a second (first five readings left out) | the game's process, CPU |
|---|---|---|
| 1. no extra environment, two minutes | median **59.9** (86 readings: min 46.2, p10 52.1, p90 60.0, max 60.1) | 162 to 190% of a core after the first 5 s |
| 2. `WINE_D3D_CONFIG=csmt=0`, one minute | median **31.0** (46 readings: min 30.0, p10 30.3, p90 32.0) | 56 to 61% |

Start 1's readings: 50 of 86 are 59.2 or more, 16 are between 56 and 59.2, 20 are below 56. The first 20 s after the window are the lowest (46 to 55); `fps-default.txt` has every reading in order.

So 0011 does by default what `csmt=0x1` did in U3, and `csmt=0` is the way back to one thread (31 fps, as before).

`menu-default.png` and `menu-csmt0.png`: the same menu as before, every portrait and button in place (by eye). No crash, no error in macron's output, and each start ended when asked (`macron kill`).

Start 3: `sample-default.txt` is 8 s at 2 ms of the whole process, any 8 s (no dip was watched for), with `loaddll-default.txt` from the same start.

A night is still not measured on a real Mac.
