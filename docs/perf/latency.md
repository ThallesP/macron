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

2026-11-19 and 20, thirteen starts in two sessions (threads "Deadlock on the M3: what limits the frame rate" and "Deadlock on the M3: sound with the new driver against the old, and a lighter load against the heat"; the files are in `~/macron-perf/fps/` and `~/macron-perf/audio/` on that Mac). All on the game's first screen: every screenshot is the "Early Development Build" dialog over black, `+map hero_testing` crashed the game (an access violation 17 s after its window) and `+map dl_hideout` showed the same dialog. **So these are that screen's numbers, not a match's.** Runtime `macron.14` with `macron.15`'s server, no Wine traces, DXMT's `Latency:` lines; the Mac had 15 of its 16 GB in use and 9 to 10 GB of swap throughout. "Start to GPU done" is the time from a frame's start, when its input is read, to the end of its GPU work.

| start | frames a second, cool (10 to 40 s) | start to GPU done | later |
|---|---|---|---|
| the game's own settings: 1920x1243, FSR2 from a 0.58 scale (5 starts) | 66.7 to 67.8 | 25.5 to 26.4 ms | 40, 24, 21 and 19 at 120 to 150 s, with 43 to 86 ms |
| half the rendered pixels (`mat_viewportscale` 0.41) | 83.3 | 20.6 ms | 64 at 70 to 100 s |
| that and every quality setting at its lowest | 84.0 | 20.4 ms | 47 at 70 to 100 s |
| **a 1280x829 picture**, the same 0.58 scale | **118.0** (0 to 30 s), 112 (30 to 60 s) | **14.8 ms** | 102.5 at 60 to 90 s, 71 after |
| 1920x1243, capped at 60 | 59.5 | 33.3 ms | 21, 12 and 21 from 90 s on (thermal level 2 from 60 s) |
| 1920x1243, half the rendered pixels, capped at 60 | 60.0 | 31 to 33 ms | 58 until 150 s, then 40, 40, 47 |
| **1280x829, capped at 60** | **60.0** | 31 to 32 ms | **60.0 for all 240 s**, level 2 from 162 s on |

- **The picture's size is what costs, not the scale it is rendered at.** Half the rendered pixels at the same picture took the GPU's time a frame from 15.5 ms to 11.7 ms; lowest quality on top, to 11.0 ms. A picture of 44% the size took it to 8.6 ms and the frame rate from 67 to 118. What the game does at the picture's size (its FSR2 pass, what follows it, the interface) is the larger part of a frame here.
- **100 frames a second is there at 1280x829**, for about a minute and a half from a cool start.
- **The fanless Mac decides the rest.** Every uncapped start reached thermal level 2 between 61 and 97 s after its window and then fell to a third of its frame rate; `pmset -g therm` named no CPU limit in any sample. Capped at 60, the full-size picture still reached level 2 at 60 s and fell to 12 to 21. The small picture capped at 60 is the one start that held: 60.0 in every 30 s of four minutes.
- **The lag follows the frame rate**: 14.8 ms at 118, 26 ms at 67, 78 to 86 ms at 19 to 24. A cap adds to it (31 to 33 ms at 60, where the uncapped 67 has 26): the game waits between frames after it has read its input.
- The game's CPU side is not the limit: its busiest thread took 38 to 71% of a core, `wineserver` 5 to 8%.
- 1280x829 was a window in the middle of a 1920x1243 desktop. Whether the game's own full-screen mode at such a size runs the same was not tried: it changes the display's mode, which a test on somebody's desktop may not do.

## What ships, and what is open

- 0003 and 0004 as described: frames wait for the display or the encoder no longer than they must. Shown on a sandbox for vsync and on the M3 for a window at its drawable ceiling.
- 0005: a game's DLSS reaches MetalFX. Shown on the M3 with Deadlock.
- **Open: a game the GPU holds back that does not ask for a limit itself** keeps three frames queued (79 ms at 51 frames a second in the test program). Neither reading of the GPU is safe to turn on. What a fix needs is a way to tell when an Apple GPU can take another frame without waiting, which Metal's command buffer times did not give for a simple program. For a game they did show a wait (Deadlock: 5 to 19 ms between scheduling and start), so `start` may work there; it has not been run on one.
- **Open: vsync on a real display**, and whether the sleep ever makes a frame miss its refresh there.
