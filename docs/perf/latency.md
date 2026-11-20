# Input latency under DXMT: what was measured

2026-11-18. A player reported input lag in Deadlock, and Thalles felt some too. This is what was measured while looking for it, and what Macron's DXMT does about it since `runtime-dxmt-0.80-macron.2` (`runtime/dxmt/README.md` has the patches and their options).

## The measure

A frame's input is read when the game starts the frame, and nothing of it can be on screen before the GPU has finished it. So the measure is the time from a frame's start to the end of its GPU work, and the number of earlier frames still unfinished at that start (the queue).

- `tools/perf/gpuload.c` ends every frame with an event query and, at the start of every frame, counts the earlier frames whose query has not finished (`RESULT queue`). It checks once a frame, so its time is up to one frame late.
- DXMT's own reading (`dxmt.logLatency`, patch 0004) takes each frame's start on the game's thread and the end of its GPU work on DXMT's finish thread.
- `tools/perf/latency.sh` runs the program for three loads and a list of settings, in alternating order, and prints both.

Loads: `gpu` (full-screen passes, the GPU is the limit), `cpu` (thousands of small draws), `sync` (`gpu` with vsync). Settings: `off` (no sleep), `sleep`, `limit1` (`dxgi.maxFrameLatency=1`), and for the two readings of the GPU that are not on by default, `finish` and `start`.

## On a sandbox (virtual GPU, three cores)

A sandbox shows that something works and what it does, never how fast. 1280x720, `PASSES=12`, `DRAWS=6000`, 12 s a run.

| | frames a second | queue | start to GPU done |
|---|---|---|---|
| DXMT 0.80 as released, `gpu` | 86 to 100 | 2.6 to 2.8 | 38 to 39 ms |
| DXMT 0.80 as released, `sync` | 60 | 2.7 | 66 ms |
| patched, `sync`, `off` (0003 alone) | 60 | 2.0 | 50 ms |
| patched, `sync`, `sleep` | 59 | 1.3 | 35 ms |
| patched, `gpu`, `off` | 82 to 97 | 2.6 to 2.7 | 39 to 43 ms |
| patched, `gpu`, `sleep` (the default: the GPU is not paced) | 94 | 2.7 | 39 ms |
| patched, `gpu`, `limit1` | 24 to 29 | 0.6 | 53 to 60 ms |

So counting a frame until it is on screen (0003) takes 16 ms off a vsync'd program there, and the sleep another 16 ms, at the same frame rate. A limit of one frame costs two thirds of the frame rate.

Two things the sandbox taught on the way:

- The first versions of the sleep waited with `Sleep()` and a loop of yields, then with `NtDelayExecution`. The `gpu` load fell from about 90 frames a second to 60 and then to 30, and a sleep of a few milliseconds took 41: under Wine both yield the processor before they wait, and on a busy three-core machine a yield costs a whole time slice. The sleep now waits on an address (`RtlWaitOnAddress`) and spins its last 300 microseconds.
- The virtual GPU's command buffer times are not a real GPU's (it reported 0.0 ms of work on a frame), and the `start` reading cost a quarter of the frame rate there (62 to 65 against 82 to 90).

## On Thalles's M3 (MacBook Air, no fan)

Test build 1, in which the sleep still paced the GPU by the `finish` reading. `PASSES=40`, 15 s a run. The Mac was cool at the start of each block; within a block it warmed up, and a warm run of the `gpu` load drops from about 50 frames a second to about 29 whatever the setting. Only runs made cool compare, and those are few: read the `gpu` rows as what they are.

| | frames a second | queue | start to GPU done |
|---|---|---|---|
| `cpu`, `off` (4 runs) | 120 | 3.0 | 33 ms |
| `cpu`, `sleep` (4 runs) | 120 | 1.0 | 16.6 to 16.8 ms |
| `cpu`, `limit1` (2 runs) | 120 | 1.0 | 17.5 ms |
| `gpu`, `off`, cool (4 runs) | 41 to 51 | 3.0 | 78 to 84 ms |
| `gpu`, DXMT as released, cool (1 run) | 51 | 3.0 | 79 ms |
| `gpu`, `finish`, cool but second in its block (4 runs) | 20 to 33 | 1.2 to 1.5 | 47 to 92 ms, p95 207 to 586 ms |
| `gpu`, `limit1` (2 runs) | 30 to 34 | 1.0 | 43 to 58 ms |
| `gpu`, any setting, warm | 27 to 31 | as above | 58 to 136 ms |

- **The `cpu` load is held by the encoder, not by the program.** DXMT's log: the encoder took 7.7 ms a frame and frames waited 16 to 18 ms for it; 120 a second is the ceiling a window's drawables set (docs/perf/README.md, "a 120-a-second ceiling"). The sleep took the queue from 3.0 frames to 1.0 and the time from 33 ms to 16.6 ms at the same frame rate, in every run. This is the case 0004 now acts on, and the only one.
- **The GPU works on two frames at once.** With the GPU as the limit, Metal reported 33 to 39 ms of work on each frame while frames started 20 ms apart. Pacing the program as if the GPU took one frame at a time (`finish`) fed it one at a time: the queue fell, the frame rate fell with it, and frames stalled for a fifth to half a second. That reading is off by default since.
- **Metal shows no wait to pace by.** For this program the time between a command buffer's scheduling and its start on the GPU was 0.0 ms however many frames were queued (the queue is inside the 33 ms). So the `start` reading never acted: frame rate and queue as with `off`.
- **The patched build costs nothing by itself:** `off` and DXMT as released ran alike (51 frames a second, 78 to 79 ms).
- Not measured there: vsync with the display as the limit. The `sync` load was GPU-bound at this `PASSES` (26 to 49 frames a second).

### Deadlock on the M3

On its "Early Development Build" dialog, from a cool start, test build 1. (Every screenshot of that screen, with any runtime, is the dialog over plain black: what the game draws there is not a scene anyone has seen, so these are the numbers of that screen and not of a match.)

- **Deadlock asks for a frame latency limit of 1** (`Maximum frame latency: the application asks for 1`, the line 0004 adds). So it never had the three-frame queue; PROGRESS.md's entry of 04:30 UTC, which read the game's silent console as "never asks", was wrong.
- With the sleep off: 74 to 77 frames a second, 22 to 26 ms from a frame's start to the end of its GPU work, one frame in flight. With the sleep (`finish`): 76 to 80 and 20 to 24 ms.
- **It does not hold that for two minutes.** In both starts the frame rate fell from the 70s to 30 within about 100 s (thermal pressure level 2 at the end), the GPU's time a frame went from 13 ms to 31 ms, and the time from start to GPU done to 50 to 70 ms. The Mac had been running tests for an hour; earlier the same day, cooler, the game held about 70 for the 75 s it was watched.

So for Deadlock on this Mac the lag is not a queue DXMT adds. What is left to look at: the frame rate once the Mac is warm, the path from the mouse to the game, and what CrossOver does differently, none of which is measured.

### Deadlock with DLSS on the M3

Test build 2 (the patches as published, with its own certificate), `DXMT_ENABLE_NVEXT=1`, the same dialog screen.

- NGX loads DXMT's `nvngx.dll` (`Loaded NGXCore from path (C:\Windows\System32\nvngx.dll)`), the game no longer backs down from DLSS, and the Metal HUD shows MetalFX's temporal scaler from 1112x720 to 1920x1243.
- First 60 s of each start: DLSS from a cool Mac 65.1 frames a second, 14.31 ms of GPU time a frame; FSR2 started at thermal level 1, 51.6 and 18.50 ms; a DLSS start at level 1, 31.5. The starts differ in temperature and in the render scale the game chose (0.667 against 0.579), so this says DLSS runs, not what it gains.
- The screen behind the dialog is black in every screenshot, with FSR2 and DLSS and with the runtime of rc4 alike, so the picture under DLSS is not judged.
- With the pinned overlays and nothing but the `dlss` switch file (`runtime-wine-11.21-macron.14`, `runtime-dxmt-0.80-macron.2`): the same NGX lines and HUD, 54.5 frames a second and 15.16 ms of GPU time over the first 60 s; two starts without DLSS in the same hour ran at a median 57.7 and 58.75 with 27.3 and 27.75 ms from a frame's start to the end of its GPU work. At the same input size DLSS on MetalFX is no faster here than the game's FSR2.

## The mouse's path

2026-11-20. What was left unmeasured above: the way from a mouse movement to the game. `tools/perf/inputlag.c` is a Windows program that registers for raw mouse input as a game does and takes, for every `WM_INPUT`, the time since the message's own time stamp, which Wine's Mac driver sets from the macOS event's. `tools/perf/mouselag.m` takes the same for a native Cocoa app, and `tools/perf/mousegen.c` moves the pointer 125 times a second. Only on sandboxes (sbx_yvtm7nsq, sbx_l6pifmpi): moving the pointer of a Mac somebody is using is not something a test may do, so **nothing here is from the M3**. 15 s a run.

| | events a second | ms since the event, median | 95th percentile | longest |
|---|---|---|---|---|
| native Cocoa app (4 runs) | 57 | 4.2 to 5.4 | 15.3 to 15.8 | 18.8 to 19.8 |
| native, five busy processes | 58 | 6.6 | 15.3 | 16.9 |
| Macron, each message handled as it arrives (6 runs) | 30 to 34 | 4 to 6 | 17 to 19 | 21 to 23 |
| Macron, the same with three ordinary and two high-priority busy threads (4 runs) | 29 to 31 | 3 to 5 | 17 to 18 | 19 to 23 |
| Macron, messages read once every 8 ms | 26 to 27 | 10 | 20 to 22 | 27 to 42 |
| Macron, messages read once every 16 ms | 21 to 22 | 12 to 16 | 25 to 30 | 36 to 39 |

- **Wine's part of the path is not measurable against the window server's.** From the macOS event through Cocoa's thread, Wine's event queue, `wineserver` and the message, a program that handles each message as it arrives has it as soon as a native app has its event, with every core busy too. `MACRON_WINESERVER_PRIORITY=normal` made no difference to it there (two runs each way).
- **A program that reads its messages once a frame waits for the read**: half a frame on average, as on Windows.
- **Fewer events than the native app is the generator's doing, most likely.** It steps two points right and left; the virtual machine's window server hands on 57 of the 125 steps a second, with coalescing on or off; a merged pair ends where it began, and Wine's server drops a movement to where the pointer already is.
- **Raw input needs the game's window to be the foreground window.** One run whose window was not (0 of 678 reads) got no `WM_INPUT` at all.

**A game that reads raw input on a thread of its own** (SDL 3 does: a time-critical thread with a message-only window, `dwFlags` 0; `--thread 1` does the same). Under Wine's Mac driver a movement enters the Windows side only while the thread that owns the window under the pointer reads its messages, where on Windows the raw thread has it when the mouse makes it. In every run 0 or 1 of about 350 movements reached the raw thread between two reads. What that costs depends on the registration:

| raw input thread, 12 ms frame | when a frame's read had the movement, median | 95th percentile | reached the raw thread after the read was over |
|---|---|---|---|
| with legacy mouse messages, as SDL registers (2 runs) | 14 ms | 25 to 26 ms | 0 and 1 of about 370 |
| with `RIDEV_NOLEGACY` (2 runs) | 18 to 19 ms | 36 ms | 235 of 394, 243 of 414 |

With legacy messages the main thread is still fetching its `WM_MOUSEMOVE` when the raw thread has its message, so the frame sees the movement in the same read. Without them the read is over first, and six movements in ten are seen a frame late. Deadlock plays through SDL 3, so it is in the first row. **Not built: a way to bring movements in when they happen** (a thread of the driver's that sends them to the server as Cocoa gets them). It would help a game that registers with `RIDEV_NOLEGACY` and reads on another thread; none is known to be affected.

So on the sandboxes the mouse's path is not where a player's lag is. On the M3 the measured part is the frame: 26 ms from a frame's start to the end of its GPU work at 67 frames a second, and 43 to 86 ms once the Mac is at thermal level 2 (below). Deadlock itself has no raw input thread there (78 threads, none named `SDLRawInput`).

## Deadlock on the M3: the size of the picture, and heat

2026-11-19 and 20, seventeen starts in three sessions (threads "Deadlock on the M3: what limits the frame rate", "Deadlock on the M3: sound with the new driver against the old, and a lighter load against the heat" and "Deadlock on the M3: the released rc7 runtime, and the smaller picture a second time"; the files are in `~/macron-perf/fps/`, `~/macron-perf/audio/` and `~/macron-perf/rc7/` on that Mac). All on the game's first screen: every screenshot is the "Early Development Build" dialog over black, `+map hero_testing` crashed the game (an access violation 17 s after its window) and `+map dl_hideout` showed the same dialog. **So these are that screen's numbers, not a match's.** Runtime `macron.14` with `macron.15`'s server, and in the third session the released v0.5.0-rc7's (`macron.16`); no Wine traces, DXMT's `Latency:` lines; the Mac had 15 of its 16 GB in use and 9 to 10 GB of swap throughout. "Start to GPU done" is the time from a frame's start, when its input is read, to the end of its GPU work.

| start | frames a second, cool (10 to 40 s) | start to GPU done | later |
|---|---|---|---|
| the game's own settings: 1920x1243, FSR2 from a 0.58 scale (5 starts) | 66.7 to 67.8 | 25.5 to 26.4 ms | 40, 24, 21 and 19 at 120 to 150 s, with 43 to 86 ms |
| half the rendered pixels (`mat_viewportscale` 0.41) | 83.3 | 20.6 ms | 64 at 70 to 100 s |
| that and every quality setting at its lowest | 84.0 | 20.4 ms | 47 at 70 to 100 s |
| **a 1280x829 picture**, the same 0.58 scale (2 starts) | **118.0 and 117.6** (0 to 30 s), 112 and 116 (30 to 60 s) | **14.8 and 14.5 ms** | 102.5 and 106 at 60 to 90 s; then 71, and 81 and 50 |
| a 1440x932 picture | 103.8 (0 to 30 s), 102.2 (30 to 60 s) | 16.2 ms | 97.8 at 60 to 90 s, then 54 and 34 |
| 1280x829, capped at 100 | 99.6 and 99.8 | 19.2 and 18.8 ms | 98.1 at 60 to 90 s, then 82, 40, 46, 66, 82 (level 2 from 75 to 181 s) |
| 1920x1243, capped at 60 | 59.5 | 33.3 ms | 21, 12 and 21 from 90 s on (thermal level 2 from 60 s) |
| 1920x1243, half the rendered pixels, capped at 60 | 60.0 | 31 to 33 ms | 58 until 150 s, then 40, 40, 47 |
| **1280x829, capped at 60** | **60.0** | 31 to 32 ms | **60.0 for all 240 s**, level 2 from 162 s on |

- **The picture's size is what costs, not the scale it is rendered at.** Half the rendered pixels at the same picture took the GPU's time a frame from 15.5 ms to 11.7 ms; lowest quality on top, to 11.0 ms. A picture of 44% the size took it to 8.6 ms and the frame rate from 67 to 118. What the game does at the picture's size (its FSR2 pass, what follows it, the interface) is the larger part of a frame here.
- **100 frames a second is there at 1280x829 and at 1440x932**, for about a minute and a half from a cool start. Capped at 100 it is a steady 100 for those 90 s, and it costs 4 ms of lag against the uncapped 118.
- **The fanless Mac decides the rest.** Every uncapped start reached thermal level 2 between 45 and 115 s after its window and then fell to a third of its frame rate; `pmset -g therm` named no CPU limit in any sample. Capped at 60, the full-size picture still reached level 2 at 60 s and fell to 12 to 21. The small picture capped at 60 is the one start that held: 60.0 in every 30 s of four minutes.
- **The lag follows the frame rate**: 14.8 ms at 118, 26 ms at 67, 78 to 86 ms at 19 to 24. A cap adds to it (31 to 33 ms at 60, where the uncapped 67 has 26): the game waits between frames after it has read its input.
- The game's CPU side is not the limit: its busiest thread took 38 to 71% of a core, `wineserver` 5 to 8%.
- **A drawn scene, at last.** The notice on the first screen is the game's own (`CCitadelIntroPopupsController::CheckShowIntroPopups` in `client.dll`) and is governed by the console variable `deadlock_early_development_warning_disabled`, which its "Don't show this again" box sets in `cfg/machine_convars.vcfg`. With `+deadlock_early_development_warning_disabled 1` as a launch argument the game goes to its title screen with no input: "PRESS ANY KEY TO START" over a drawn city at night, seen from a rooftop, with smoke and birds moving. That needs a key, so it is as far as a test goes; `+map dl_hideout` loads the hideout behind it. At a 1280x829 picture that scene ran at 119.2, 120.0 and 120.0 frames a second in its first three half-minutes, 12.2 to 12.9 ms from a frame's start to the end of its GPU work, 8.3 to 8.4 ms of GPU work a frame, then 109 and 68 at thermal level 2 (thread "Deadlock on the M3: past the first dialog without a click"). 120 is the ceiling a window's drawables set here (docs/perf/README.md), so the scene is no heavier than the black screen behind the notice was.
- 1280x829 was a window in the middle of a 1920x1243 desktop. Whether the game's own full-screen mode at such a size runs the same was not tried: it changes the display's mode, which a test on somebody's desktop may not do.

## Deadlock on the M3: a smaller screen that fills the display

2026-11-20, eleven starts in two sessions (threads "Deadlock on the M3: a smaller picture that fills the screen" and "Deadlock on the M3: the smaller picture's second build, the sound thread in the real-time band, and a start in the practice map"; files in `~/macron-perf/scale/` and `~/macron-perf/scale2/` on that Mac). Wine patch 0017 shows the game a screen of fewer pixels and scales the picture up to the display, whose mode stays what it was (`runtime/wine/patches/README.md`, "0017"). The released rc7 runtime with the `winemac.so` under test; `+deadlock_early_development_warning_disabled 1`, so the title screen over its drawn city; no `video.txt`, so the game chose its settings each time. Medians of the first two or three half-minutes, the Mac at thermal level 0:

| the screen the game is shown | what it rendered | frames a second | start to GPU done |
|---|---|---|---|
| 1920x1243, the display's own | 1112x720, upscaled | 75.0, 74.5 | 22.9 to 23.9 ms |
| 1440x932 | 1112x720, upscaled | 90.3, 89.0 | 18.9 to 19.5 ms |
| 1280x829 (two starts) | 1112x720, upscaled | 92.6, 93.6; 93.4, 92.7 | 18.6 to 19.1 ms |
| **1098x711** (two starts) | 1098x711 as it is | **97.3, 100.1, 100.1; 100.2, 99.2** | **16.5 to 18.1 ms** |
| 1024x663 | 1024x663 as it is | 107.2, 104.0 | 15.9 to 16.2 ms |
| 1440x932 with a `video.txt` made at the display's size | 835x540, upscaled | 102.0, 100.9 | 17.1 to 17.8 ms |

- **The game renders 720 lines whatever screen it has** and upscales to the screen (`mat_viewportscale` 0.579, 0.773 and 0.869 for the first three rows: 720 lines each time). So a smaller screen takes away the work at the screen's size and nothing else, until the screen is below 720 lines and is rendered as it is. `+mat_viewportscale 0.58` on the command line changed nothing: the game wrote its own 0.869.
- **Deadlock's profile shows it 780,000 pixels**, 1098x711 on this display: 97 to 100 on the title, 6 ms less from a frame's start to the end of its GPU work than at the display's size. The picture fills the screen edge to edge (the window is at 0,0 and 1920x1243 points in every start of the second build); its text is slightly soft, and at 1024x663 softer and slightly blocky.
- A player whose settings were made at the display's size keeps the scale they had (0.58), now of the smaller screen: fewer pixels than before, faster and blurrier, until they raise the game's own setting.

## Deadlock on the M3: in the practice map, and heat

The same two sessions and a third (thread "Deadlock on the M3: does a frame limit keep the practice map out of the heat collapse", files in `~/macron-perf/scale3/`). `+map new_player_basics` loads the game's offline practice map with no input: first person, in front of a shop, the interface drawn. **This is the first measurement of the game being played rather than of a menu.** A 1098x711 screen, each start from thermal level 0 after three minutes at it:

| | 0-30 s | 30-60 | 60-90 | 90-120 | 120-150 | 150-180 | 180-210 | 210-240 | level 1, level 2 from |
|---|---|---|---|---|---|---|---|---|---|
| no limit (two starts, 150 s) | 70.1, 75.4 | 70.0, 70.3 | 52.7, 52.7 | 29.3, 30.7 | 18.3, 14.1 | | | | 50 s, 65 s |
| `+fps_max 60` | 59.9 | 59.9 | 58.8 | 58.8 | 46.2 | 26.6 | 29.3 | 42.5 | 80 s, 100 s (back to 1 at 211 s) |
| `+fps_max 45` (two starts) | 44.2, 44.9 | 44.7, 45.0 | 45.0, 45.0 | 44.4, 45.0 | 44.9, 44.9 | 45.0, 45.0 | 45.0, 45.0 | 44.6, 43.3 | 131 s, 181 s |
| `+fps_max 36` | 35.9 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 170 s, never |

- **Without a limit the game is at 14 to 30 frames a second a minute and a half after it starts**, with 70 to 92 ms from a frame's start to the end of its GPU work (24 ms while cool), and stays there. That, and not Wine or the sound driver, is what playing on this Mac was like.
- **Held to a rate the Mac can keep, it keeps it**: 36 for four minutes without reaching level 2; 45 for four minutes, the last one at level 2 with single stretches of 34 to 39. 60 is too much: level 2 at 100 s, then 26 to 46.
- The GPU works on a frame for 16 ms while cool whatever the limit (20 to 23 ms once warm), so at 45 frames a second it is busy nine tenths of the time. The game used 1.3 processor cores at 45 and 1.8 at 60, `wineserver` 0.05 to 0.1; everything else on the Mac together, the test's own tools among it, 0.6 to 1.4.
- **The game's own limit adds a frame of latency by this measure**: 40 to 43 ms from start to GPU done at 45, 49 to 54 ms at 36, 32 ms at 60. It waits after the frame has started.
- **Sound**: no frame missing in any report of any start (0016). The game's own `Audio device starved` lines come in the first 12 s of a start (0 to 4, while it loads) and when the Mac is throttled: 1 and 5 between 132 and 137 s in the two starts without a limit, 1 at 229 s in one of the two at 45 (its lowest stretch), none after loading at 36, at 60 or in the other at 45.

### A frame limit that follows the heat: tried, and not shipped

The table above suggested a rule: when macOS's thermal pressure level reaches 2, hold the game to a rate the Mac can keep, before macOS takes more. It was built (branch `dxmt-0006`: a DXMT patch that takes the limit in the sleep before a frame starts, about two thirds of the rate at level 2, a tenth lower every 20 s while the level stays, higher again at level 1; its rule tested by itself and on a sandbox with the level set by hand) and run on the M3 against the same build with it off (thread "Deadlock on the M3: a frame limit that follows the heat, on and off, in the practice map"; 480 s and 300 s in the practice map, each from a cool start):

| | 0-90 s | 90-120 | 120-150 | 150-180 | 180-210 | 210-240 | 240-270 | 270-300 | level 2 |
|---|---|---|---|---|---|---|---|---|---|
| the limit on: frames a second | 74, 73, 71 | 64.2 | 42.5 | 36.5 | 29.5 | 26.6 | 24.1 | 25.4 | 125 to 255 s, and from 416 s |
| start to GPU done | 24 to 25 ms | 28.0 | 25.5 | 31.1 | 35.2 | 39.8 | 41.6 | 39.1 | |
| the limit off: frames a second | 73, 72, 70 | 61.3 | 48.2 | 32.9 | 33.5 | 43.9 | 51.0 | 52.3 | 100 to 211 s, and from 302 s |
| start to GPU done | 23 to 26 ms | 28.6 | 38.5 | 47.6 | 48.9 | 40.0 | 37.3 | 34.1 | |

- **It lost frames and kept the Mac no cooler.** From 120 s on the game averaged 30 frames a second with the limit (to 480 s) and 44 without (to 300 s), and the Mac was at level 2 for a little over half of that time either way: about two minutes each time, whatever the game did meanwhile. Climbing back at level 1 from 24 it was at level 2 again at 35 frames a second. Once the Mac is warm through, no rate worth playing at keeps it under level 2, and macOS's own throttling (33 frames a second at level 2 in this start, 44 to 52 at level 1) is no worse than a limit.
- **The fall to 14 to 18 frames a second did not come back** in either start. It was seen in the fifth and sixth starts of a session that waited only for level 0 between starts, not three minutes more; these two had the longer wait. What a Mac that has played for half an hour does is not something a test of eight minutes shows.
- **What the limit did do is make frames younger**: at the same 33 frames a second, 31 ms from a frame's start to the end of its GPU work against 48 ms; at 42 against 48 frames a second, 25.5 ms against 38.5. A game that the GPU holds back starts each frame as early as it can and the frame waits; started later, it does not. That is the open item below about GPU pacing, and the reason for the next test.

## What ships, and what is open

- 0003 and 0004 as described: frames wait for the display or the encoder no longer than they must. Shown on a sandbox for vsync and on the M3 for a window at its drawable ceiling.
- 0005: a game's DLSS reaches MetalFX. Shown on the M3 with Deadlock.
- **Open: a game the GPU holds back that does not ask for a limit itself** keeps three frames queued (79 ms at 51 frames a second in the test program). Neither reading of the GPU is safe to turn on. What a fix needs is a way to tell when an Apple GPU can take another frame without waiting, which Metal's command buffer times did not give for a simple program. For a game they did show a wait (Deadlock: 5 to 19 ms between scheduling and start), so `start` may work there; it has not been run on one.
- **Open: vsync on a real display**, and whether the sleep ever makes a frame miss its refresh there.
