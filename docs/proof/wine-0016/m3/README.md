# Patch 0016 on Thalles's M3 (MacBook Air, no fan)

Two sessions on 2026-11-19 and 20 (local time), Opus 5.5 delegates, no synthetic input, the installed Macron untouched. Their full reports, with every `macron-audio:` line as printed, are the last messages of the T3 threads "Deadlock on the M3: sound with the new driver against the old, and a lighter load against the heat" and "Deadlock on the M3: the revised sound driver against the old and its own two timer modes"; the files are on that Mac in `~/macron-perf/audio/` and `~/macron-perf/audio3/`. What is below is taken from those lines.

The patch had three builds. The first never left the sandbox (Wine's own tests found its position and latency wrong). The second (`audio-test2`) ran here in session 1, the third (`audio-test3`, SHA-256 `3be2a877...`) in session 2, and the third is what `runtime-wine-11.21-macron.16` and v0.5.0-rc7 carry, byte for byte.

The test runtime there is `macron.14` with `macron.15`'s `wineserver` (SHA-256 `5c2f6788...`, the raised one), and the `winecoreaudio.so` under test in both Wine trees. One file gives every side: `MACRON_AUDIO=upstream` is the driver as Wine has it ("old"), `MACRON_AUDIO_TIMER=fixed` the engine with its timer thread left in the application band. Output: the built-in speakers at 44100 Hz, so the stream's rate is the device's. "Missing" is frames CoreAudio asked for and did not get while the client was feeding the stream.

## Every core busy, no game

`tools/perf/sndload.c --policy sdl --rate 44100 --seconds 30` (eight ordinary and four high-priority busy threads on eight cores, the sound thread registered as Pro Audio and mixing 2 ms a wake-up), thermal level 0 before each run.

| driver | missing of pulled | gaps | the client's wake-ups, 95th and 99th percentile (period 11.61 ms) |
|---|---|---|---|
| old, session 1 | 335,833 and 327,100 of 1,411,584 and 1,412,608 (24%, 23%) | 448, 431 | 28.8 and 46.7 ms; 28.8 and 44.7 ms |
| old, session 2 | 336,852 and 346,606 of 1,411,072 and 1,411,584 (24%, 25%) | 440, 467 | 28.6 and 48.7 ms; 29.3 and 47.1 ms |
| second build (`audio-test2`: a reserve from one period, at most 100 ms, the timer in the application band) | 2,786 and 2,925 (0.2%) | 7, 5 | 20.9 and 35.9 ms; 22.6 and 42.8 ms |
| third build, timer in the application band | 1,384 and 1,354 (0.1%) | 1, 1 | 21.3 and 36.0 ms; 21.2 and 39.6 ms |
| **third build** (`audio-test3`, SHA-256 `3be2a877...`) | **0 and 0** | **0, 0** | **11.69 and 11.72 ms; 11.69 and 11.73 ms** |

The timer thread, third build: with a time constraint it set the event more than 2 ms after CoreAudio had asked once in each 15 s (the longest 13 to 103 ms, once a run); in the application band 365 to 420 times of about 1,170 in each 15 s, up to 136 ms. `NtSetEvent` itself took 0.05 to 0.13 ms from the real-time thread, and 25 and 38 ms once in each run.

## The event's regularity, nothing else running

`tools/perf/evcadence.c`, second build against old, 6 s a state: the engine's event came every 11.61 ms in all four states (95th percentile 12.2 to 12.4 ms, longest 13.7 to 17.4 ms); the old driver's every 11.3 to 11.5 ms (95th percentile 15.5 to 16.0 ms, longest 16.9 to 47.8 ms).

## A thread's wake-up by its policy, without Wine

`tools/perf/wakelat.c`, 12 s a run, a real-time thread signalling a semaphore every 10 ms:

| the woken thread | 8 busy threads | 12 busy threads |
|---|---|---|
| fixed, importance 32 (priority 63), native and under Rosetta | longest 0.004 and 0.008 ms | 0.005 and 0.014 ms |
| time constraint, native and under Rosetta | 0.007 and 0.011 ms | 0.007 and 0.005 ms |
| timeshare, native and under Rosetta | median 0.5 and 0.6 ms, longest 9.8 ms, 494 of 1,201 over 1 ms | longest 0.16 and 1.09 ms |

So a thread that sets itself to priority 63 with Mach's own calls is woken at once on this Mac, under Rosetta too. Wine's sound timer thread, which the server puts at that priority from outside, was not. Why the two differ is not known.

## Deadlock

On its first screen (the "Early Development Build" dialog; nothing clicked), no Wine traces, each start from thermal level 0. Every one of the ten starts reached level 1 between 30 and 77 s after its window and level 2 between 45 and 97 s; in session 2 the frame rate then fell from 62 to 68 to as low as 11 to 26.

Session 2, 210 s a start, one report each 30 s (frames missing, gaps):

| start | 0-30 s | 30-60 | 60-90 | 90-120 | 120-150 | 150-180 | 180-210 | all |
|---|---|---|---|---|---|---|---|---|
| old 1 | 2,560 (2) | 0 | 1,536 (1) | 0 | 54,784 (45) | 10,240 (14) | not printed | 69,120 frames, 1.57 s, in 62 gaps |
| old 2 | 2,048 (1) | 0 | 2,048 (3) | 4,096 (6) | 32,256 (26) | 18,944 (19) | 1,024 (1) | 60,416 frames, 1.37 s, in 56 gaps |
| third build, timer in the application band, 1 | 0 | 0 | 0 | 6,144 (2) | 0 | 0 | 0 | 6,144 in 2 gaps |
| third build, timer in the application band, 2 | 0 | 0 | 0 | 0 | 2,560 (1) | 0 | not printed | 2,560 in 1 gap |
| **third build 1** | 0 | 0 | 0 | 0 | 0 | 0 | not printed | **0** |
| **third build 2** | 0 | 0 | 0 | 0 | 0 | 0 | not printed | **0** |

- The third build's reserve was 2,560 frames (58 ms) for the first 90 s of both starts, then 1,792 and 1,280 (29 ms): it gave back while the Mac was throttled, and nothing was missing after. In the first 30 s the queue went down to 0 and to 512 frames: the stall while the game loads (`NtSetEvent` took 51 to 61 ms once in each of the six starts, whatever the driver) used the starting reserve up.
- The timer thread with a time constraint: at most 0.13 ms from CoreAudio's request to the event in every 30 s after the first, at thermal level 2 too. In the application band: more than 2 ms for 622, 1,026 and 638 of about 2,400 events in the throttled 30 s periods of one start, up to 216 ms.
- With the reserve at its 200 ms limit (8,820 frames) after those two gaps, the start with the timer in the application band lost nothing more.

Session 1, 150 s a start: old 5,632 frames in 7 gaps and 20,480 in 19; the second build 1,536 in 1 gap (120 s printed) and 5,120 in 3, the last two with its reserve at its 100 ms limit and the timer thread up to 135 ms late.

**What the driver does not change:** the game's own console line `[SoundSystemLowLevel] Audio device starved 512 samples`. In session 2 it appeared 1 to 4 times in the first seconds of every start and 1 to 10 times between 103 and 181 s, when the Mac was at level 2 and the game at 11 to 40 frames a second, with every driver (old: 1 and 2 at the start, 2 and 2 later; third build: 4 and 3, then 3 and 1; timer in the application band: 3 and 4, then 10 and 2). That is the game's mixer not having sound ready when its own sound thread asks, upstream of anything the driver sees.
