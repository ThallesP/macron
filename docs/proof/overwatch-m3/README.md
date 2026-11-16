# Overwatch on Thalles's MacBook Air M3, 2026-11-16

Macron 0.4.0 from the release (runtime `wine-11.21-macron.6`, DXMT 0.80, the Steam bridge), macOS 26.5.2, 16 GB of memory, SIP on. Thalles signed in with his own Battle.net account and played: "I get 60fps", and "a few lags from time to time, not very common". He asked for the lags to be looked at and for the game to be started with what that needs; the session below was started from the session on his Mac the way Steam starts it (`SteamAppId=2357570`, `STEAM_COMPAT_DATA_PATH`, `macron waitforexitandrun Overwatch.exe`), with `show-fps` on and `MTL_HUD_LOG_ENABLED=1`, which makes Apple's Metal HUD write every frame's interval to stderr and so into macron's log. A script wrote one line a second about the Mac (swap, memory pressure level, thermal pressure level, DXMT's shader cache size). No screenshots; nothing from his accounts is in this folder, and the raw logs stay on his Mac.

## The session

01:59:15 to 02:08:01 local time, 17,903 frames; from 02:00:20 on he was in the practice range (265 s of play, one 148 s pause with the game in the background).

- **Frame rate: 57.5 fps average** over the 265 s, 53.7 to 58.8 per minute.
- **37 frames longer than 50 ms**, 8 of them longer than 100 ms, the longest 312 ms.

| what the Mac was doing in the 5 s around the frame | frames over 50 ms | median | longest |
|---|---|---|---|
| DXMT's shader cache grew, no swap-outs | 16 | 91 ms | 312 ms |
| a burst of swapping, cache unchanged | 10 | 61 ms | 80 ms |
| neither | 11 | 64 ms | 117 ms |

Seven of the eight frames over 100 ms are in the first row. This is a match in time, not a controlled test.

## Memory

- The game's process (`wine`, translated) had a footprint of **15 GB** on a 16 GB Mac. The HUD's own memory figure: 3.7 GB at the first frame, 8.5 GB 11 s later, 13.3 GB after 21 s, then level at 15.2 to 15.6 GB.
- `footprint` on it, three minutes in:

  | category | dirty |
  |---|---|
  | MALLOC_SMALL | 10 GB |
  | MALLOC_TINY | 1,288 MB |
  | WINE_RESERVE | 1,134 MB |
  | Owned physical footprint (unmapped) (graphics) | 914 MB |
  | MALLOC_MEDIUM | 697 MB |
  | IOAccelerator (graphics) | 214 MB |
  | Rosetta JIT | 128 MB |

  So about 12 GB is macOS's own `malloc` (libraries on Wine's Unix side), about 1.1 GB the game's Windows memory, about 1.1 GB graphics. `top` showed 14 GB of the 15 as compressed.
- The system was at memory pressure level 2 (warning) from eight seconds after the first frame, reached 4 (critical) once, and swap in use went from 5.5 GB to 9.5 GB during the session.
- Thermal pressure: 0 for the first three minutes, 1 at 02:02:28, 2 at 02:03:59, back to 1 at 02:07:02. The frame rate per minute did not follow it in this session.

## Not established

- Which library allocates the 12 GB. DXMT's Unix side is the suspect (it starts in the first 20 s after the first frame, before any map is loaded); nothing here shows it.
- Whether the "neither" hitches have their own cause.
- On quit, macron's log ends with `macron: LAUNCH_FAILED: starting Wine: exec: WaitDelay expired before I/O complete` although the game had run and closed normally.
- The log also has DXMT's `err: CreateGeometryShaderWithStreamOutput: not supported, expect problem` once per start; Thalles reported no wrong pictures.
