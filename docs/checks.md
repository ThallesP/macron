# How a scenario is checked

A scenario (`scenarios/<name>/scenario.json`) is one Windows program, how to get it, how to know it reached its menu, and a reference picture of that menu taken on real Windows. `macron-check` runs it.

## The run on a Mac

1. Download each of the program's `downloads` from its pinned URL, check its SHA-256 and unpack it `into` its folder (or build it, for programs that live in the repo).
2. Move the mouse pointer to the screen's top-left corner (some games draw their own pointer and tooltips), then screenshot the desktop before anything starts. This is one of the negative controls.
3. `macron runtime install`, then `macron launch --game <profile> --cwd <cwd> -- <exe> <args>`. The scenario's `args` and `files` (written before the first start, into the game's folder or its Windows environment) are test setup (a fixed window size, answers to a first-start dialog), passed like Steam launch options; the profile holds what any player of the game needs. The game goes through the launcher, its profile in `fixes/games/` and its own Windows environment, the same way Steam will start it.
4. Wait until the scenario's "reached" condition holds: a window whose title matches `window.title` is on screen and, if set, the game has printed `reached.output`. Then wait `reached.settleSeconds`. Fail if the game exits first or `reached.timeoutSeconds` passes.
5. Capture the window with `screencapture -l`, drop the macOS title bar, and keep the `window.width` x `window.height` picture (scaled on Retina screens). A window of another width fails.
6. Read the text in the picture with macOS's Vision framework, once as captured and once scaled up 2x (Vision misread Xonotic's stylised "Credits" as "Eredits" at 1x in one CI run). Every string in `compare.text` must be in one of the two readings (case and spacing ignored).
7. Compare the picture with the reference and run the negative controls (below). Save everything to the evidence folder: `frame.png`, `window.png`, `screen.png`, `desktop-before.png`, `game.log`, `runtime-install.log`, `windows.json`, `report.json`, `summary.md`.

## The measure

Defined in `internal/imagecheck` before any Macron output was looked at, and the same for every scenario:

- Both pictures are scaled to 320x240 by averaging. Regions listed in `compare.ignore` (fractions of the picture) are painted grey in both. Use them only for parts that move by themselves on real Windows (an animated background), shown by the reference capture's own 5-second stability score.
- **SSIM** on luminance, 8x8 windows every 4 pixels, averaged over the windows where either picture has structure (standard deviation of at least 6 grey levels). Flat areas are left out because they agree with almost anything and would hide a broken frame. Must be **at least 0.70**.
- **Colour distance**: both pictures scaled to 16x12, mean absolute difference per channel, as a fraction of 255. Must be **at most 0.10**.
- **Menu text**: every string in `compare.text` is found by OCR.

The thresholds are constants in the code. A scenario cannot loosen them.

## Negative controls

Each run also compares these with the reference. Each must fail the picture measure on its own (without the text check), or the run fails, because then the measure could not tell a broken result from a good one:

- a black frame,
- the reference with its colours inverted,
- the reference upside down,
- the reference shifted right by a fifth of its width (a misplaced viewport),
- the desktop captured before the game started,
- another game's reference (`otherGame`), once there is another game.

The unit tests in `internal/imagecheck` and `internal/check` check the same things on synthetic pictures, including that the same picture with a one-pixel offset, softer filtering, noise and slightly different colours still passes.

## References

The `Reference` workflow runs `macron-check reference <scenario>` on a GitHub Windows runner: the original program, natively, with the same profile files and arguments, the pointer parked in the same corner, waiting for the same condition, capturing the window's client area with `BitBlt`. It captures three times (at the reached condition, 5 seconds later and 30 seconds later) and reports how similar the later ones are to the first, with the scenario's ignore regions applied. Those scores are the evidence for an ignore region: a part of the picture may be ignored only if it changes by itself on real Windows. The picture is committed as `scenarios/<name>/reference.png` with its source (run URL) in `reference.source`. A reference is never made from Macron's output.

# How a frame rate is checked

A frame-rate scenario is a scenario with a `frameRate` block. It runs a probe of ours (`scenarios/d3d9-fps-probe/`) that draws a fixed 2D scene as fast as it can and prints its own frame times. Gate 1 needs it: Ultimate Custom Night (32-bit, Clickteam Fusion 2.5, Direct3D 9) cannot run in CI because it needs a Steam account, so the probe takes the same path (a 32-bit program, Direct3D 9, Wine's builtin `d3d9` on `wined3d`, through `macron launch` and the game's profile) and CI checks its frame rate instead.

## The probe

`probe32.exe` and `probe64.exe` are the same C program built for 32-bit and 64-bit (`build.sh`: MinGW-w64 GCC for both; without an `i686-w64-mingw32-gcc` on the machine the 32-bit one is built with `zig cc -target x86-windows-gnu`, fetched by `uvx`). It opens a 1280x720 window, creates a Direct3D 9 HAL device (hardware vertex processing, software if that fails) with `D3DPRESENT_INTERVAL_IMMEDIATE`, so it measures the drawing path and not the display's refresh, and draws every frame:

1. a full-window backdrop (a 256x256 texture stretched, no blending),
2. 400 sprites (`--sprites`), each its own draw of two triangles with pretransformed vertices, alpha-blended, with a different texture from the previous sprite (64 textures of 64x64), every fifth one additive, every third one semi-transparent, the filter alternating between point and linear,
3. a 512x256 panel texture and a full-window semi-transparent vignette.

`--window WxH` makes the window's client area smaller than the 1280x720 back buffer, which `Present` then scales, as Fusion does with "Resize display to fill window" (Ultimate Custom Night ran full screen at 1920x1243 points on the MacBook). The scenario uses 960x540 because the desktops of GitHub's macOS and Windows runners are 1024x768, and both systems shrink a 1280-wide window to fit (Windows captured 1038x720 of it).

The picture is the same every frame, so a screenshot taken at any time can be compared with the Windows reference; the work is redone every frame all the same. Modes (the first argument) change how the draws reach Direct3D:

| Mode | Each sprite | Per frame |
|---|---|---|
| `up` | `DrawPrimitiveUP` | nothing else |
| `vb` | lock a dynamic vertex buffer (`D3DLOCK_DISCARD` for the first quad of a frame, `D3DLOCK_NOOVERWRITE` after), `DrawPrimitive` | nothing else |
| `tex` | `DrawPrimitiveUP` | the panel texture rewritten with `LockRect(D3DLOCK_DISCARD)` |
| `mix` (default) | `DrawPrimitiveUP` | the panel rewritten with `LockRect`, the scene drawn into a render-target texture, then that texture drawn to the back buffer |

`mix` is the guess at Clickteam Fusion 2.5's Direct3D 9 renderer (`mmf2d3d9.dll`). Its source is not public, so it rests on what Clickteam and its users have written: Fusion 2.5 keeps every image as its own texture and draws objects one by one in display order, so "it would switch to a new texture almost every draw call" (Clickteam's blog, "Sprite Sheets", 2017, explaining what Fusion 3 would change); in the hardware-accelerated runtimes backdrops are redrawn every frame like active objects (ClickFusion Academy, "1000 Backdrop Objects vs 1 Active Object"); and Fusion games such as Ultimate Custom Night redraw text and counters into textures and use effects that draw the frame through an intermediate surface (the Perspective object, which Steam users of Five Nights at Freddy's games blame for slowdowns). The exact draw calls (`DrawPrimitiveUP` against a vertex buffer, how often states change) are not known; `up`, `vb` and `tex` separate those costs so that the real game's numbers can be matched to one of them.

It warms up for 2 seconds (`--warmup`, at least 60 frames), then measures for 10 seconds (`--seconds`) and prints

```
RESULT adapter <Direct3D 9 adapter description> (vendor 0x<id>, device 0x<id>)
RESULT fps <frames/s> p50_ms <x> p95_ms <y> p99_ms <z> frames <n>
```

Each line goes out in one `WriteFile`, because Wine, MoltenVK and the probe share the output: in one sandbox run MoltenVK printed 271,367 warnings and cut a line the C runtime wrote in pieces (`RESULT prese[mvk-warn] ...`). The check finds a `RESULT` line even when another writer's unfinished line precedes it; a line cut in two does not count.

A frame time is the time between two returns of `Present`, from `QueryPerformanceCounter`; the frame rate is the frames counted divided by the measured time; the percentiles are nearest-rank over every measured frame. Then it keeps drawing for `--hold` seconds (so the picture can be captured without disturbing the measurement) and exits 0. A failure prints `RESULT <step>: FAILED (...)` and exits 1. `--throttle-ms N` sleeps N ms after every frame; `--no-present` draws without presenting. Both exist to prove the check can fail.

## The measure

Defined in `internal/check/framerate.go` before any of Macron's numbers were compared with anything:

- The scenario is reached when the probe prints `RESULT fps`; then the window is captured and compared with the reference exactly as in "The measure" above (SSIM, colour distance and every negative control). That is the proof that the frames were presented.
- **Frame rate**: the `RESULT fps` line must be there, well formed, with at least one frame, and its frame rate at least `frameRate.minFps`.
- **Against a baseline**: with `frameRate.baselineExe` (another build of the same probe next to the scenario's exe, such as `probe64.exe`) and `frameRate.minShareOfBaseline`, the check also runs the baseline through macron with the same arguments on the same machine, and the threshold becomes the higher of `minFps` and `minShareOfBaseline` times the baseline's frame rate. The baseline must print a well-formed `RESULT fps` line with at least one frame, or the check fails. This is how a check on a runner, whose software renderer sets the absolute number, still catches a cost that only one build pays: the 32-bit probe against the 64-bit one on the same path.
- `frameRate.minFpsSource` says where the threshold came from. It is set from the Windows runner's numbers (the `Frame rate (Windows)` workflow and the `Reference` workflow's `frameRate`), never from Macron's output alone. A scenario without a threshold (`minFps` and `minShareOfBaseline` both 0) cannot pass and cannot run in CI.

## Programs that crash while starting

A scenario may set `startAttempts` (1 to 3). When the program exits before it reaches the scenario's condition, the check starts it again, up to that many times in all, and each failed start is written into the report's notes and the summary with its exit status and log. A program that reaches its condition is never started again, so a slow run, a wrong picture or a crash after the condition still fails. The frame-rate scenario uses it because the 32-bit probe crashes while creating its Direct3D device in some runs on the runner (PROGRESS.md, "Gate 1 findings"): that crash is a separate, open problem, and the notes keep it visible instead of letting it fail one push in five.

## Negative control

After the measured run (and the baseline, if there is one), the check starts the probe again through macron with `--throttle-ms` set to `ceil(2000 / threshold)`, the threshold being the one in effect after the baseline (100 ms while no threshold is set), so it cannot run faster than half the threshold. That run must report a frame rate and fail the threshold, or the check fails: otherwise the measure could not tell a slow run from a fast one. The unit tests in `internal/check` also feed it a missing line, a failed `Present`, broken numbers and zero frames, and check that each fails.

## Reading the numbers

GitHub's macOS runners have a virtual GPU without hardware OpenGL, so the scenario draws OpenGL with Mesa llvmpipe (`MACRON_OPENGL=llvmpipe`), and the Windows runner draws Direct3D with the Microsoft Basic Render Driver (WARP). Both are software renderers: the numbers show relative cost (32-bit against 64-bit, one backend against another, a regression against the last run), not what a Mac with a real GPU reaches. With `MACRON_OPENGL=apple-software` the Mac side runs Wine's own OpenGL on Apple's software renderer instead of Mesa, which keeps the path a 32-bit Direct3D 9 game takes on a real Mac (Wine's 32-on-64 OpenGL, where buffer maps are copied). The frame rate on a real Apple GPU is not measured by any check; the release notes say how a user can see it.
