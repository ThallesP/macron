# Request U3: Wine patch 0010 on the real GPU (Thalles's MacBook Air M3), 2026-11-16 08:40 UTC

`macron` built from branch `wine-0010` at d7a78e6, runtime `wine-11.21-macron.10` in its own `MACRON_HOME`. Ultimate Custom Night started as Steam starts it with no input, so it sits in its menu; an empty workspace in front, sound off, Thalles asleep. Four starts: a minute each with the frame counter, and 20 s each with `WINEDEBUG=-all,+loaddll` and an 8 s `sample` at 2 ms.

| configuration | menu, frames a second (46 readings each, the first five left out) | the game's process, CPU |
|---|---|---|
| default (command stream on the game's thread) | median **31.0** (min 30.0, p10 30.3, p90 31.7, max 32.6) | 54 to 66% of a core |
| `WINE_D3D_CONFIG=csmt=0x1` | median **60.0** (min 47.8, p10 50.1, p90 60.0, max 60.1) | 184 to 190% of a core |

For comparison: 0.4.0 4.9 to 5.2; `macron.9` 31.0 default and 30.0 with `csmt=0x1`.

With `csmt=0x1` the first 12 s read 48 to 55 and then the readings sit at 60 with dips: of the 46, 33 are 59.2 or more, 9 are between 56 and 59, 4 are 47.8 to 50.1. The two threads together use nearly two cores, so there is little room left: a night draws about 1.7 times as much as this menu and was not measured (it needs input).

Pictures: `menu-default.png` and `menu-csmt.png` show the same menu as `../menu.png` from 0.4.0, every portrait and button in place; compared by eye only.

Files: `fps-*.txt` (the frame counter's lines), `cpu-*.txt` (seconds, %CPU of the game's process every 5 s), `sample-*.txt` (the whole `sample` output of the process) with `loaddll-*.txt` from the same start.

No crash and no error in macron's output in any of the four starts.
