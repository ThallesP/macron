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
- **Deadlock's profile shows it 780,000 pixels**, 1098x711 on this display: 97 to 100 on the title, 6 ms less from a frame's start to the end of its GPU work than at the display's size. The published rc8, unpacked on this Mac with nothing set by hand, ran the title at 92.3 (89.3 to 93.8) with 19.1 ms, the GPU working 11.0 ms on a frame where it had worked 9.8 to 10.4 in the morning's two starts. The picture fills the screen edge to edge (the window is at 0,0 and 1920x1243 points in every start of the second build); its text is slightly soft, and at 1024x663 softer and slightly blocky.
- A player whose settings were made at the display's size keeps the scale they had (0.58), now of the smaller screen: fewer pixels than before, faster and blurrier, until they raise the game's own setting.

## Deadlock on the M3: in the practice map, and heat

The same two sessions and a third (thread "Deadlock on the M3: does a frame limit keep the practice map out of the heat collapse", files in `~/macron-perf/scale3/`). `+map new_player_basics` loads the game's offline practice map with no input: first person, in front of a shop, the interface drawn. **This is the first measurement of the game being played rather than of a menu.** A 1098x711 screen, each start from thermal level 0 after three minutes at it:

| | 0-30 s | 30-60 | 60-90 | 90-120 | 120-150 | 150-180 | 180-210 | 210-240 | level 1, level 2 from |
|---|---|---|---|---|---|---|---|---|---|
| no limit (two starts, 150 s) | 70.1, 75.4 | 70.0, 70.3 | 52.7, 52.7 | 29.3, 30.7 | 18.3, 14.1 | | | | 50 s, 65 s |
| `+fps_max 60` | 59.9 | 59.9 | 58.8 | 58.8 | 46.2 | 26.6 | 29.3 | 42.5 | 80 s, 100 s (back to 1 at 211 s) |
| `+fps_max 45` (two starts) | 44.2, 44.9 | 44.7, 45.0 | 45.0, 45.0 | 44.4, 45.0 | 44.9, 44.9 | 45.0, 45.0 | 45.0, 45.0 | 44.6, 43.3 | 131 s, 181 s |
| `+fps_max 36` | 35.9 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 36.0 | 170 s, never |

- **The smaller screen is worth 20 frames a second in play, and 8 ms.** The same map at the display's own size (`MACRON_RENDER_PIXELS=0`, two starts of 75 s at thermal level 0, thread "Deadlock on the M3: the practice map at the display's own size"): 53.3 and 54.6 frames a second, 32.4 and 31.9 ms from a frame's start to the end of its GPU work, the GPU working 22.8 and 22.1 ms on a frame. On the 1098x711 screen, six such starts: 73.4 to 74.4 frames a second, 24.1 to 24.7 ms, 15.9 to 16.4 ms. And with the published rc8, its own runtime and nothing set by hand (thread "Deadlock on the M3: the published rc8, unpacked and not installed, with nothing set by hand"): 76.7 frames a second and 22.8 ms, against 54.6 and 32.7 ms with `MACRON_RENDER_PIXELS=0`. The picture's text is sharp at the display's size and soft on the smaller screen.
- **Without a limit the game is at 14 to 30 frames a second a minute and a half after it starts**, with 70 to 92 ms from a frame's start to the end of its GPU work (24 ms while cool), and stays there. That, and not Wine or the sound driver, is what playing on this Mac was like.
- **Held to a rate the Mac can keep, it keeps it**: 36 for four minutes without reaching level 2; 45 for four minutes, the last one at level 2 with single stretches of 34 to 39. 60 is too much: level 2 at 100 s, then 26 to 46.
- The GPU works on a frame for 16 ms while cool whatever the limit (20 to 23 ms once warm), so at 45 frames a second it is busy nine tenths of the time. The game used 1.3 processor cores at 45 and 1.8 at 60, `wineserver` 0.05 to 0.1; everything else on the Mac together, the test's own tools among it, 0.6 to 1.4.
- **The game's own limit adds a frame of latency by this measure**: 40 to 43 ms from start to GPU done at 45, 49 to 54 ms at 36, 32 ms at 60. It waits after the frame has started.
- **Sound**: no frame missing in any report of any start (0016). The game's own `Audio device starved` lines come in the first 12 s of a start (0 to 4, while it loads) and when the Mac is throttled: 1 and 5 between 132 and 137 s in the two starts without a limit, 1 at 229 s in one of the two at 45 (its lowest stretch), none after loading at 36, at 60 or in the other at 45.

### How small a screen 100 frames a second would take in play

The published rc8 with `MACRON_RENDER_PIXELS` set by hand, the practice map, 75 s a start at thermal level 0 throughout, medians of 15 to 60 s (thread "Deadlock on the M3: smaller screens in the practice map, lowest quality, and the game's sound mix-ahead"). The game rendered each screen as it is (`mat_viewportscale` 1.0).

| the screen | frames a second (lowest to highest) | start to GPU done | the GPU worked on a frame | frames started apart | the interface's text |
|---|---|---|---|---|---|
| 1920x1243, the display's own (three starts, earlier) | 53.3 to 54.6 | 31.9 to 32.7 ms | 22.1 to 22.8 ms | 17.5 to 18.9 ms | sharp |
| 1098x711 (780,000; eight starts, one here) | 73.4 to 76.7 | 22.8 to 24.7 ms | 15.5 to 16.4 ms | 13.5 to 14.4 ms | soft, readable |
| 1098x711, the game's settings at their lowest | 79.4 (75.6 to 82.9) | 22.6 ms | 15.2 ms | 12.7 ms | soft, readable |
| 1024x663 (680,000) | 80.0 (75.4 to 86.1) | 22.7 ms | 14.5 ms | 12.9 ms | soft, readable |
| 962x623 (600,000) | 82.6 (79.4 to 86.3) | 21.9 ms | 13.8 ms | 12.7 ms | soft, readable |
| 878x568 (500,000) | 89.1 (77.7 to 94.3) | 20.1 ms | 12.5 ms | 11.6 ms | the clock hard to read |
| 806x522 (420,000), the game's settings at their lowest | 96.1 (89.0 to 103.5) | 17.9 ms | 11.5 ms | 10.4 ms | the clock hard to read |

- **100 frames a second in play is not on this Mac at a picture one can read.** A fifth of the display's pixels and the lowest settings give 96. From the profile's screen down to that one the GPU's work on a frame falls by 4 ms and the frame rate rises by 20, while the text goes from soft to hard to read. The profile stays at 1098x711: 680,000 pixels would be 4 frames a second more.
- **The game's quality settings are worth 3 to 5 frames a second** at the profile's screen (shader quality, shadows, motion blur, bloom and texture detail at their lowest: 79.4 against 73.4 to 76.7).
- One of the two starts at 806x522 never drew a frame: a black window, the console stopping 9.9 s after the window at the game's own server start, the process alive until it was stopped. Not seen in any other start of the day, at any size, the second one at 806x522 included; not looked into, since that size is not one to ship.
- **`+snd_mixahead 0.1` does not take the game's `Audio device starved` lines away**: one while loading (5.8 to 11.3 s) in four of the five starts that had it, as in the starts without it. The console variable exists (`soundsystem.dll`); the game prints nothing about it. The driver reported no frame missing in any start.

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

### DXMT's two readings of the GPU, in the practice map

`dxmt.latencyGpuModel` unset, `start` and `finish`, five starts of 75 s at thermal level 0 throughout (thread "Deadlock on the M3: DXMT's two GPU pacing models in the practice map"), medians of the lines between 15 and 60 s:

| | frames a second | start to GPU done | in flight | slept | the GPU worked on a frame | started frames apart |
|---|---|---|---|---|---|---|
| unset (two starts) | 74.3, 74.4 | 24.2, 24.6 ms | 1 | 0.0 ms | 15.9, 16.3 ms | 13.6, 14.1 ms |
| `start` (two starts) | 73.5, 73.9 | 24.7, 24.3 ms | 1 | 0.0 ms | 16.4, 16.0 ms | 14.4, 14.1 ms |
| `finish` | 73.4 | 24.1 ms | 1 | 0.0 ms | 16.1 ms | 14.1 ms |

**Nothing to remove while the Mac is cool.** One frame in flight, no wait for the encoder, and neither reading ever slept: the 24 ms are the game's own 6 to 9 ms, DXMT's 5 to 6 ms of encoding, 3 to 4 ms before the GPU starts and its 16 ms of work, the last two overlapping the next frame's (frames start 14 ms apart). Neither reading costs frames here either, which `finish` did in the test program. So the game's input lag in play on a cool M3 is the time a frame takes, and what shortens it is less work a frame.

### Where a frame's time goes once the Mac is throttled

The published rc8, the practice map, 300 s a start from a cool Mac, `dxmt.latencyGpuModel` unset, `finish` and `start` (thread "Deadlock on the M3: where a frame's time goes once the Mac is throttled, with DXMT's GPU pacing off, start and finish"). Medians of each 30 s of the first start, unset; thermal level 1 from 96 s, 2 from 116 s, 1 again from 232 s:

| | 0-60 s | 90-120 | 120-150 | 150-180 | 180-210 | 210-240 | 240-270 | 270-300 |
|---|---|---|---|---|---|---|---|---|
| frames a second | 76.9, 76.4 | 67.9 | 64.4 | 41.8 | 33.2 | 37.7 | 45.2 | 47.8 |
| start to GPU done | 23.0, 23.7 ms | 26.5 | 29.6 | 38.2 | 49.0 | 48.2 | 41.6 | 35.9 |
| the game's own time a frame ("CPU") | 8.2, 7.3 ms | 11.7 | 11.8 | 25.6 | 33.7 | 30.6 | 24.8 | 22.8 |
| DXMT's encoding | 5.4, 5.9 ms | 7.0 | 7.5 | 9.4 | 10.6 | 10.1 | 9.2 | 9.6 |
| before the GPU starts a frame | 3.3, 3.2 ms | 3.0 | 2.1 | 0.6 | 0.3 | 0.2 | 0.2 | 2.0 |
| the GPU's work on a frame | 15.1, 15.7 ms | 16.5 | 17.1 | 20.9 | 22.8 | 23.4 | 21.3 | 20.3 |

- **Throttled, the processor is what is slow, not the GPU.** The game's own time a frame goes from 7 to 8 ms to 25 to 34 ms (single lines up to 58 ms), DXMT's encoding from 5.5 to 10 ms, the GPU's work only from 15.5 to 23 ms, and a frame no longer waits for the GPU at all (0.3 ms). One frame in flight throughout. The 15 ms that the heat-following limit took off came from there: with fewer frames to make, the processor was slowed less.
- **So pacing by the GPU cannot help, and does not.** With `finish`: 30.4 and 30.9 frames a second and 61.5 and 62.2 ms in the two worst half-minutes, against 33.2 and 49.0 unset. With `start`: 31.1 and 33.2 frames a second, 65.8 and 43.2 ms. Neither ever slept. Level 2 lasted 111 to 116 s in all three starts and the frame rate came back to 44 to 48 at level 1.
- The sound driver reported no frame missing in any of the 27 half-minutes; the game's `Audio device starved` lines were at 6.1 s only.

### What the throttle does to a thread, by its scheduling policy

`tools/perf/qosrate.c` under Rosetta beside the game in its practice map (thread "The M3 throttled by the game, second try"): six threads, one for each policy, each doing the same piece of work every 10 ms. Thermal level 1 from 101 s after the game's window, 2 from 122 s, 1 again from 248 s. Median time for the piece of work:

| | an ordinary thread | user-interactive | user-initiated | utility | fixed priority 63 | time constraint |
|---|---|---|---|---|---|---|
| level 0, the first seconds | 3.8 ms | 3.8 ms | 4.0 ms | 5.9 ms | 2.9 ms | 2.9 ms |
| level 2 | 6.8 to 7.7 ms | 6.6 to 7.6 ms | 6.7 to 7.7 ms | 10.6 to 15.3 ms | 6.6 to 7.8 ms | 6.6 to 7.8 ms |

- **macOS slows every thread alike once it throttles: 2.7 times for the ones that had a performance core to themselves** (2.91 ms to 7.80 ms, and 7.80 again and again: a clock held down, not a queue). No policy Wine could give a game's threads escapes it. The same program alone, with every core busy for seven minutes, never took the Mac past level 0: it is the GPU's heat that brings the throttle, and the processor that pays.
- While cool, a thread at Wine's time-critical priority or with a time constraint did the work in 2.9 ms where an ordinary one took 3.8: with the performance cores 93% busy (the game and this program together), ordinary threads spend part of their time on the efficiency cores. The game alone leaves the performance cores less full than that.

## Deadlock on the M3 at the display's own size (from 2026-11-20, 15:00 UTC)

Thalles: speed is to come from Macron's, Wine's and DXMT's code, not from a smaller picture. The profile shows the game the display as it is again, and what follows is measured there: 1920x1243, the game's own settings (it renders 1112x720 and upscales with FSR2), the practice map, the published rc8 with `MACRON_RENDER_PIXELS=0`, each start from a cool Mac and 75 s long.

| | frames a second | start to GPU done | the GPU's work on a frame | frames started apart |
|---|---|---|---|---|
| without Apple's Metal HUD (three starts in two sessions, 10:57 to 12:00 UTC) | 53.3, 54.6, 54.6 | 32.4, 31.9, 32.7 ms | 22.8, 22.1, 22.3 ms | 17.5 to 18.9 ms |
| with it, `MTL_HUD_ENABLED=1` (three starts in two sessions, 15:18 to 16:08 UTC) | 64.9, 70.3, 71.1 | 26.4, 24.5, 24.2 ms | 18.7, 18.2, 17.8 ms | 14.3, 13.8, 13.4 ms |
| DXMT's display-synced limit at 60 (`d3d11.preferredMaxFrameRate=60`) | 29.5 | 42.7 ms | 28.3 ms | 35.5 ms |
| the game's own `mat_vsync` 1 | 71.1 | 24.2 ms | 17.8 ms | 13.4 ms |

- **The HUD is not what made the later sessions faster.** Alternated in one session (16:15 to 17:00 UTC, thread "with Apple's Metal HUD and without, alternating"): 73.0, 71.5 and 70.2 frames a second without it, 70.6, 68.7 and 70.3 with it. So the same scene with the same software ran at 53 to 55 in the morning and about 70 in the afternoon.
- **In every test of the day the game's window was behind another program's.** A test starts the game from the background, and macOS leaves what Thalles is working in on top: at 40 s into three starts the window list had a browser's or a code editor's window of 1920x1204 over the game, the menu bar drawn, and in the last one the lock screen. A capture of the screen showed the other program, not the game. So no number in this file is of the game in front, covering the screen, as it is played, and the GPU was shared with whatever was in front (the morning's 53 to 55 against the afternoon's 70 fits that, without proving it). (A start with a small window of the test's own put over the game ran at 56 against 69 for the one before it, but both were already behind a full-size window, a different program's each time, and the second ran warmer: it says nothing about the small window.) The HUD's word was `Direct` in all of them, so that word is not about reaching the display. There is no reading of the game in front at all: the sessions of 2026-11-18 (70.8 frames a second on the first screen) were test starts too. Putting the game in front takes his hand or a program that takes his focus away.
- **So the smaller screen's gain was overstated.** "54 to 74 to 77" compared two morning sessions' full size with the smaller screen; against the afternoon's 70 it is 5 frames a second for a soft picture. The profile's return to the display's size loses little.
- **DXMT's display-synced limit halves the frame rate and loses `Direct`**: 30 frames a second, the HUD's word `Composited`, the GPU's time a frame doubled. It is not a way to hold 60 here. The game's own vertical sync setting changes nothing under DXMT.
- **The HUD's `FPS` is not a count of frames the panel showed**: it read 55 to 57 in one session and 65 to 68 in the next for the same scene, the second above the panel's 60 Hz. The built-in panel shows 60 frames a second at most, whatever is rendered.
- **The `dlss` switch opens the game on a warning now**: "Graphics Driver Out-of-Date", a box over a black screen that needs a click (`Vendor 10de, 'NVIDIA GeForce RTX 4090', version 31.0.13.783 < 32.0.15.8108`). The version is wined3d's, which the game asks through Direct3D 9; DXMT's own answer has none. Behind the box the HUD showed MetalFX Temporal from 1112x720 to 1920x1243 and the GPU working 13.1 ms on a frame, of a scene nobody saw.
- **The frame rate follows the frame's latency, not the GPU's time alone**: the game asks for a frame latency limit of 1, so a frame may start only when the one before the last is done, and frames start a little over half of "start to GPU done" apart in every row of every table above (24.5 ms and 13.8; 32.4 and 17.5; 17.9 and 10.4).

### What a frame is made of: a Metal System Trace

`xcrun xctrace record --template 'Metal System Trace' --attach <the game's process>`, 3 s asked for and 6.9 s recorded, exported with `xctrace export` and read by a script (thread "a Metal System Trace of a frame, saved this time"; the trace and the scripts are in `~/macron-perf/rc15/` on that Mac). **The Mac was at thermal level 3 with its screen locked while it was taken** (61 frames in 6.9 s), so the times are a throttled Mac's and only their proportions are worth reading. A first attempt was lost because the recorder was stopped while it saved: saving 6 s of this game took over a minute and a half, and the recorder slowed the game to 45 frames a second while it ran.

- **A frame is 98 encoders in one large command buffer**: 75 render passes, 13 compute passes, 8 blits and 2 clears on the median, encoded by DXMT's encode thread, plus the HUD's own two.
- **The GPU's time a frame by kind of work: compute 40%, fragment 38%, vertex 22%** (14.1, 13.2 and 7.6 ms of 35.0 summed, the three overlapping in time).
- **Two compute passes are a third of the frame.** One near the end, just before the passes that draw the interface, takes a fifth to a quarter of all the GPU's time (7.2 to 8.8 ms on this throttled Mac); one in the middle about an eighth (3.6 to 4.4 ms). No other pass is over a tenth: the next are four render passes of 1.7 to 3.7 ms each, half vertex and half fragment (geometry-heavy: shadows or a depth pass), and the rest is small.
- The pass before the interface is where a game's temporal upscaler runs: Deadlock's FSR2, from 1112x720 to 1920x1243, as a Direct3D compute shader that DXMT translates. It fits the reading of the same day's start with the `dlss` switch, where MetalFX's temporal upscaler did that step instead and the GPU's work on a frame was 13.1 ms against 18.2 ms (behind a warning box, so of a scene nobody saw).
- Encoders, command buffers and textures carry no names (DXMT labels nothing), so a pass is known by its place in the frame only. The display's tables were empty: the display was asleep.

### What a frame asks of Wine's server

On macOS every wait on an event or a semaphore, every handle, every socket call is a request to Wine's server: a round trip for the calling thread and a turn of the one thread the server has. Nothing had counted them without `WINEDEBUG=+server`, which slows the server to a crawl. A test build of the server counts them by name when `MACRON_SERVER_STATS` names a file (Wine patch 0020, on branch `wine-0020`, not shipped). The published rc9 with that server, the display's own size, thermal level 0 (thread "how many requests to Wine's server a frame costs, and which"):

| | frames a second | requests a second | requests a frame | the server's share of a core |
|---|---|---|---|---|
| the practice map | 75.9 | 14,000 to 16,600 | 211 (204 to 223) | 9% |
| the title screen | 92 | 14,700 to 15,100 | 160 | |

Of the practice map's 945,363 requests in a minute: `socket_get_events` 47.6%, `select` 18.0%, `event_op` 17.1%, `close_handle` 5.6%, `create_event` 3.5%, `set_async_direct_result` 2.4%, `create_semaphore` 2.1%, `recv_socket` 1.6%, `release_semaphore` 1.3%, `send_socket` 0.8%.

- **Half of everything the server is asked is one question about sockets**: `socket_get_events` (what `WSAEnumNetworkEvents` asks), 7,000 to 7,900 times a second on the title screen and in the map alike, whatever the frame rate, against 256 packets received a second and 128 sent. On Windows that call costs a microsecond; here each is a round trip. Which thread and which module asks is the next session's question.
- **Waits and wake-ups are 80 a frame** (`select`, `event_op`, the semaphores, and the events and semaphores made and closed again: about 6,000 a second). An in-process path for them, as Linux has in ntsync, would take those round trips away: at something like 20 microseconds each for the caller, a millisecond and a half of a frame's 13, spread over the game's threads.
- **Who asks** (`sample` of the game for 5 s at one sample a millisecond, the title screen, thread "which thread asks Wine's server about sockets 7,000 times a second"): the socket question is all one thread's, the game's own "SteamNetworkingSockets" (Valve's networking library, which on Windows asks each of its sockets for news whenever one of them has some): 235 of its 679 samples were inside that request, a third of its time, and it runs an eighth of a core. It is not on a frame's path. The main thread had 51 of 679 samples in a request to the server (its events and waits), 7.5% of its time; the seven job threads 4 to 8 each; DXMT's threads next to none. `sample` could not name the Windows modules above: under Rosetta a stack into Wine's Unix side is cut at 512 frames of the system-call dispatcher.
- **The server itself is idle 96% of the time** (147 of 3,825 samples of its thread not waiting), at 9 to 11% of a core by `ps`. Nothing queues behind it.
- **So Wine's server is not where a frame's time goes.** An in-process path for waits would give the main thread back at most 7.5% of its time, a millisecond a frame; answering the socket question without the server would free a twelfth of a core that no frame waits for. Neither is built: the first is weeks of work on every wait in Wine for a millisecond, and the second changes what an online game's networking is told, which no offline test can vouch for.

### MetalFX's picture against FSR2's, seen

The published rc9, unpacked and not installed, four starts in the practice map alternating the `dlss` switch, the screen unlocked this time (thread "Deadlock on the M3 with the published rc9, second wait"; captures and crops in `~/macron-perf/rc9pub/shots/` on that Mac). No warning box in any of the eight captures; the console says driver `32.0.15.9144` with the switch.

| | frames a second | start to GPU done | before the GPU starts a frame | the GPU's work on a frame | thermal level |
|---|---|---|---|---|---|
| FSR2 | 71.5 | 23.6 ms | 2.3 ms | 17.9 ms | 0 |
| MetalFX, 1112x720 to 1920x1243 | 70.9 | 24.8 ms | 7.1 ms | 13.5 ms | 0, then 1 at 64 s |
| FSR2 | 54.4 | 33.0 ms | 3.6 ms | 22.5 ms | 2 from 60 s |
| MetalFX | 66.1 | 29.4 ms | 6.4 ms | 16.5 ms | 2 from 60 s |

- **On a cool Mac MetalFX is no faster and no sooner**: the same frame rate, and a frame a millisecond older. The GPU works a quarter less on a frame (13.5 ms against 17.9 ms), but the frame waits 7 ms before the GPU starts it where FSR2's waits 2 to 4 ms, in every start that has had the switch without a frame limit (6.4 to 7.1 ms in the five of them). With FSR2 the GPU works on two frames at once (17.9 ms of work, frames 13.5 ms apart); with MetalFX it takes them one after another (13.5 ms of work, 13.4 ms apart). What makes them wait is not known: MetalFX's temporal scaler keeps a history, and a frame may not be able to start before the one before it has left the scaler.
- **On a throttled Mac it is ahead**: 66 against 54 frames a second and 29 against 33 ms in this session's hot pair, as in the earlier two pairs. Less GPU work is less heat.
- **The picture: smoother and softer.** FSR2's is sharper, with a fine dot pattern over lit surfaces, a glow that breaks into squares of about 8 pixels, and stair-steps on slanted edges. MetalFX's has none of those, and loses fine distant detail: cornice teeth run together, window bars smear, far lettering is harder to read (a blur measure of 7.6 to 7.8 against 5.6 on the same facade). The interface is drawn after the upscale and is the same in both. Stills of a still camera cannot show ghosting on moving things.
- **So the switch stays a switch.** It trades distant detail for cleaner edges and a cooler GPU, and Thalles asked that nothing be gained by a worse picture: that trade is his to make, not a profile's.
- **The game could not be put in front.** With keyboard and mouse idle for five minutes, `NSRunningApplication.activate` on the game returned true and the browser's window stayed in front: macOS does not let a background program take the front from the one in use. A test cannot do it; only he can.
- **One gap in the sound**: 905 frames (20 ms) missing in one gap in the first half minute of one start, while the game loaded: the first frames missing in a start of the game with Macron's driver since its third build. The reserve starts at 58 ms and the stall while the game loads has been 28 to 58 ms in every start; this one was longer. What holds the driver's `NtSetEvent` that long is the next thing to find (a test build of the server logs its slow turns).

### The hitch twenty seconds into every start: the server saving the registry

The one gap in the sound above had a cause, and it was not the game loading. Wine's server writes the registry to disk thirty seconds after it starts, whole, on the one thread it has; that is twenty seconds after the game's window. A test server that logs its slow turns (Wine patch 0020) showed one turn of 152.5, 83.2 and 63.4 ms there in three starts of three, with no request in it, and Wine's audio timer held for as long: 3,270, 887 and 162 frames of sound missing. Every start of the day had it ("event set max" of 28 to 58 ms in its first half minute); the sound reserve covered it when it was shorter than 58 ms. Every thread of the game that needed the server waited as long, so it is a hitch in the picture too.

With the save written by a child process (Wine patch 0021; `runtime/wine/patches/README.md` has the table): 16.4, 14.3 and 13.9 ms, which is `fork()` under Rosetta, the audio timer held 10 to 16 ms, and no sound missing in three starts of three. The registry files are written as before.

The game's own `Audio device starved` lines are still there (3, 1 and 2 in those three starts): all but one in the first 11 s, while the map loads, and of 28 to 470 samples where they had been 512. They are not this.

### Five minutes with the published rc10: nothing else holds the server

The published `Macron-0.5.0-rc10.dmg`, unpacked and not installed, its own runtime (`wineserver` `5f3a995d...`, the file tested above), `MACRON_SERVER_STATS` on, the practice map at the display's own size, 300 s a start from a cool Mac (thread "Deadlock on the M3 with the published rc10: five minutes of the server's log, for any other stall").

| | frames a second, by half minute | start to GPU done | thermal level | turns of the server of 5 ms or more | sound missing |
|---|---|---|---|---|---|
| as it is | 72.6, 71.4, 69.4, 64.8, 64.6, 64.5, 60.7, 53.6, 51.1, 51.5 | 23 ms, then 26 to 28, then 31 to 38 | 1 from 120 s, 2 from 165 s | one: 19.4 ms at the registry save | none in nine reports |
| the game's `+fps_max 45` | 45.0 in all ten | 42 to 44 ms | 0 throughout | one: 12.0 ms at the registry save | none in nine reports |

- **The registry save's `fork()` is the only turn of 5 ms or more in ten minutes of the game**, and the files are written (new modification times at the save, `WINE REGISTRY Version 2`, no temp file, no zombie). Wine's audio timer was never held longer than 11 ms, and after the first half minute never longer than 3.4 ms.
- The server is asked 13,000 to 15,000 things a second throughout, half of them the networking thread's socket question; none of it takes a turn to 5 ms.
- **Held to 45 by the game's own limit the Mac stayed at thermal level 0 for the whole five minutes**, a steady 45.0. Unheld it went from 72 to 51 over the five minutes, at level 2 from 165 s.
- The game's `Audio device starved` lines: at 6.9 and 11.8 s in the first start and 5.8 s in the second, while the map loads, and none in the 288 s after.

### A frame on a cool Mac, traced with FSR2 and with MetalFX: where 100 would have to come from

Two Metal System Traces of the published rc10, 2 s each, the practice map at the display's own size, thermal level 0 throughout (thread "Metal System Traces of a frame with FSR2 and with MetalFX, on a cool Mac"; traces, exported tables and scripts in `~/macron-perf/rc16/` on that Mac). The recorder slows the game (to about 60 frames a second while it records), so the frame rates are from DXMT's lines just before it.

| | FSR2 | MetalFX |
|---|---|---|
| frames a second before the recorder | 72.6, 72.2, 73.0 | 77.5, 78.9, 75.9 |
| the GPU's time a frame, summed over its channels | 17.7 ms (compute 6.2, fragment 7.8, vertex 3.7) | 16.1 ms (compute 5.1, fragment 7.3, vertex 3.6) |
| from a frame's first GPU work to its last | 19.9 ms | 17.0 ms |
| the upscaler's own passes | 3.1 ms in one compute pass, and 1.2 ms in another | 0.5, 1.1 and 0.6 ms in three |
| the main command buffer's first pass waits for the GPU | 0.5 ms | 0.5 ms |
| two frames on the GPU at once | 1.8% of the time | 0.6% |

- **A frame is one chain on the GPU.** Its 95 passes are in one command buffer and run one after another, each starting when the one before is done (their waits grow from 0.7 ms for the first to 14 ms for the last), with a pass's vertex work overlapping the fragment work of the pass before and little else. Two frames' chains hardly ever overlap (1.8% of the time). At 72 to 73 frames a second a frame is 13.8 ms, and by DXMT's own line the chain takes about 12 of them (17.7 ms from the frame's first small blit to its last pass, less the 5 to 6 ms before the main command buffer is committed): the GPU is busy nearly nine tenths of the time with one frame at a time. 100 frames a second would take a chain under 10 ms, and the game's own threads to keep up (its main thread is at 70% of a core at 72).
- **There is nothing in the chain for Macron to take out.** The largest pass is the game's FSR2 (3.1 ms); the next are geometry passes of 1 to 2 ms each and a compute pass of 1.2 ms; 136 passes of under 0.15 ms make up 2.8 ms together. MetalFX in FSR2's place takes about 2 ms off the chain (5 frames a second in this session, none in the one before). The compute pass "in the middle of the frame" that looked like an eighth of the GPU's time on a throttled Mac is 1.2 ms on a cool one.
- **The 7 ms that a MetalFX frame seemed to wait is not a wait of the frame.** MetalFX finishes each frame with a pass of its own in a command buffer of its own, which Metal counts as the first of the next frame; it waits for that frame's turn (7.8 ms) and takes 0.6 ms. DXMT's log line reads it as the frame's start. The frame's own first pass waits 0.5 ms with either upscaler.
- **So 100 frames a second at the display's size is not in Wine's or DXMT's hands on this Mac**: the game asks the GPU for about 12 ms of work a frame, in an order that leaves the GPU little to do in parallel, and it allows itself one frame in flight. Under a recorder that slows the game the GPU was idle a third of the time; whether a faster processor side would fill it was not measured. What is left is less work a frame (the game's own settings, MetalFX, a smaller picture), or a faster GPU.

## What ships, and what is open

- 0003 and 0004 as described: frames wait for the display or the encoder no longer than they must. Shown on a sandbox for vsync and on the M3 for a window at its drawable ceiling.
- 0005: a game's DLSS reaches MetalFX. Shown on the M3 with Deadlock.
- **Open: a game the GPU holds back that does not ask for a limit itself** keeps three frames queued (79 ms at 51 frames a second in the test program). Neither reading of the GPU is safe to turn on. What a fix needs is a way to tell when an Apple GPU can take another frame without waiting, which Metal's command buffer times did not give for a simple program. Deadlock asks for a limit of one, and in its practice map neither reading did anything, good or bad (above).
- **Frames on a throttled Mac**: at thermal level 2 Deadlock's frames take 48 ms at 33 frames a second where a cool Mac's take 24 at 75, and the time is the game's own processor work, slowed four times ("Where a frame's time goes once the Mac is throttled"). A frame limit takes load off the processor and so some of that time, at the price of frames; pacing by the GPU does nothing. A thread's scheduling policy makes no difference to it (`tools/perf/qosrate.c`, above): what is left is less work a frame.
- **Open: vsync on a real display**, and whether the sleep ever makes a frame miss its refresh there.
