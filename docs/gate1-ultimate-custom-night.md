# Ultimate Custom Night on a Mac sandbox (gate 1)

Measured on 2026-11-15 on one GitHub macOS 26 runner (sandbox `sbx_qsjpicyv`: virtual Apple M2 Pro, 5 cores, "Apple Paravirtual device", Metal but no hardware OpenGL), with the game as Steam downloaded it for the test account (app 871720, 32-bit Clickteam Fusion 2.5, Direct3D 9 through `mmf2d3d9.dll`) and runtime `wine-11.21-macron.5`, started by the `macron` the sandbox had (main before the Steam bridge: since 981005f, a runtime with the bridge starts Steam games through `steam.exe`, which these numbers do not cover). The MacBook was not used. Every start is reported, not the best ones.

## The short version

**The game is slow on the sandbox because its menu and its nights draw about 2,100 and 3,900 separate quads every frame, and every one of them goes through Wine's Direct3D 9 to OpenGL path one at a time.** The title screen is 4 draws a frame and runs at 12 to 14 fps; the menu runs at 1.1 to 1.4 fps and a night at 0.6 to 0.8 fps on Macron's path (`MACRON_OPENGL=apple-software`).

- **Patch 0003 (buffer-map copies) was not this game's cost.** Its frames map no buffers: 0 copies a frame with Gcenx's stock `wined3d.dll` and with Macron's. Each draw's 96 bytes of vertices and 12 bytes of indices go through `glBufferSubData` in both builds.
- **Patch 0004 (no command-stream thread) is worth 2× here:** menu 1.36 against 0.58 fps, night 0.62 against 0.36, and one core instead of 1.6 to 1.9. With only that thread turned back on (`csmt=0x1`), the numbers are the same as stock wined3d's.
- **Patch 0006 (the mode switch) decides whether the game starts at all:** with stock `wow64cpu.dll`, 3 of 3 starts never reached a window. With 0006, all 18 starts on Wine's OpenGL path did, and so did the 3 software-fallback starts.
- **The sandbox's GPU limits Apple's OpenGL, not Metal.** The same load in a native program runs at 2 to 2.5 fps on Apple's software OpenGL renderer and at 100 to 165 fps on Metal on the same virtual GPU. So 60 fps cannot be shown on this sandbox through OpenGL, which is the only way Wine's Direct3D 9 draws here.
- **Macron's side is not free either.** Our own 32-bit Direct3D 9 probe, at the night's draw count, runs at 1.2 fps through Wine where the native program runs at about 3 fps on the same renderer: Wine and wined3d add about 140 µs a draw on this machine.
- **Turning Apple's software renderer on is better for this game than leaving it to Fusion's own software display.** That display is faster only on the title screen (27 to 32 fps against 12 to 14). Its menu is 0.15 to 0.2 fps, and its night is 0.05 fps and drawn wrong.
- **What the M3 showed (5 fps on the title) is not explained by anything the sandbox can reproduce.** The frame rate on a real Apple GPU is still for Thalles to see with the release.

## How it was measured

Scripts: `tools/clickteam-games/ucn-*.sh`, `ucn-summary.py`, `ucn-glcount.py`, `loadprobe.m`.

- **Starting the game.** Every start was made the way Steam's Play makes it, from the game's folder:

  ```sh
  SteamAppId=871720 STEAM_COMPAT_DATA_PATH=<copy> macron waitforexitandrun "<game folder>/Ultimate Custom Night.exe"
  ```

  `<copy>` was a fresh APFS clone of the game's prefix, so no start inherited another's state. Every Wine process was killed by name before and after each start. Steam was not started. `ucn-setup.sh` made two prefix copies from Steam's, one with the Mac driver's `AllowSoftwareRendering=y` and one without (see "A trap" below). It also extracted Gcenx's stock `wined3d.dll` (i386 `cd98b20e...`, x86_64 `15c387b5...`) and `wow64cpu.dll` (`539bea7e...`) from the cached wine-devel 11.21 archive, next to macron.5's (`834ee2e3...`, `607111a5...`, `d1603e33...`). A configuration's DLLs were copied into the DXMT tree and the prefix, and macron.5's were put back afterwards.
- **Three places per start** (`ucn-run.sh`). The places are:
  - the title screen ("WARNING! … PRESS ENTER");
  - the character-select menu (Enter);
  - a night, in the office. GO! opens a controls screen with its own GO!, and the second GO! starts the night.

  At each place: 15 s to settle (25 s for the night, which loads first), then 20 s measured, with a screenshot before and after. Wine took input only once it was the active app (`drive activate wine`), and even then a key or click was lost now and then. So each step was checked by reading the window with drive's OCR ("PRESS ENTER" gone, "Power Generator" shown, then gone) and tried again, up to 5 times. That check reached all three places in 11 of 12 starts; in e's start 2 the GO! click never opened the controls screen. The three starts of a ran before the check existed (the same keys and clicks, unchecked), and their screenshots show the title, the menu and the office.
- **Frame rate with Direct3D** (configurations a, b, c, d). This is wined3d's own counter (`WINEDEBUG=-all,+fps`): one `wined3d_cs_exec_present … @ approx N fps` line every 1.5 s or every frame, whichever is longer, each timestamped as it arrived. Per place, the table gives the median of the samples inside the 20 s window.
- **Frame rate of Fusion's software display** (configuration e). Without a Direct3D device, the game presents each frame as one `StretchBlt` of the whole 1024×768 window from a memory DC (`WINEDEBUG=-all,+bitblt`: `NtGdiStretchBlt … dst … log=0,0 1024x768 … rop=cc0020`, one a frame, no other blits to the window). The measure is those lines per second. It counts frames the game hands to the window, as wined3d's counter does, and costs one trace line a frame.
- **CPU** is each process's CPU time over the 20 s window (`ps`), in % of one core. "Game" is the game's Wine process. wineserver was 0.7 to 2.3% in every run, and every other Wine process together 1.6 to 3.3%, so neither appears in the tables.
- **What a frame is made of** (`ucn-glcount.py`). One start per wined3d build with `WINEDEBUG=-all,+opengl,+fps` and everything traced, measured over 8 s at each place. It counts the OpenGL calls per presented frame, the bytes through `glBufferSubData` and `glTexSubImage2D`, and opengl32's wow64 buffer-map copies (`Copying 0x…`).
- **The layers** (`ucn-layers.sh`). The same load without the game, on the same sandbox, right after the game runs:
  - `loadprobe` is a native program that draws N textured, alpha-blended quads into a 1024×768 render target and that target to a 1024×768 window. Before each draw it uploads that draw's vertices and indices at the next offset of one buffer and sets two matrices and a vec3, as wined3d does for the game. It draws with `glDrawElementsInstancedBaseVertex`, on Apple's software OpenGL renderer and on Metal, built for arm64 (native) and x86_64 (under Rosetta, which is how a Wine process loads Apple's OpenGL).
  - The Direct3D 9 frame-rate probe (`scenarios/d3d9-fps-probe`, `up` mode: one `DrawPrimitiveUP` per sprite of 40 to 88 px) ran in Wine through macron at 1024×768 with the same draw counts, 64-bit and 32-bit.

## Frame rates

fps is the median of wined3d's samples in the 20 s window (e: full-window blits per second); CPU is the game process. Every start of every configuration is listed. Screenshots of the game's window: `docs/proof/gate1-ucn/` (a: title, menu, night; e: night).

**a. macron.5 as shipped, `MACRON_OPENGL=apple-software`** (Apple's software renderer through Wine patch 0002)

| start | title fps | title CPU | menu fps | menu CPU | night fps | night CPU |
|---|---|---|---|---|---|---|
| 1 | 12.52 | 92% | 1.38 | 103% | 0.62 | 105% |
| 2 | 14.19 | 91% | 1.36 | 102% | 0.58 | 105% |
| 3 | 12.27 | 95% | 1.14 | 102% | 0.76 | 104% |
| 0 (earlier start; its "night" was the controls screen) | 14.21 | 93% | 1.23 | 101% | (controls 8.43) | |

**b. a with Gcenx's stock `wined3d.dll`** (no 0003, no 0004)

| start | title fps | title CPU | menu fps | menu CPU | night fps | night CPU |
|---|---|---|---|---|---|---|
| 1 | 12.61 | 163% | 0.56 | 182% | 0.35 | 160% |
| 2 | 9.91 | 177% | 0.58 | 167% | 0.39 | 182% |
| 3 | 10.91 | 162% | 0.58 | 185% | 0.36 | 188% |

**c. a with `WINE_D3D_CONFIG=csmt=0x1`** (0003 kept, the command-stream thread back on)

| start | title fps | title CPU | menu fps | menu CPU | night fps | night CPU |
|---|---|---|---|---|---|---|
| 1 | 13.91 | 182% | 0.54 | 177% | 0.34 | 172% |
| 2 | 11.13 | 171% | 0.57 | 190% | 0.34 | 185% |
| 3 | 11.59 | 176% | 0.70 | 174% | 0.41 | 180% |

**d. a with Gcenx's stock `wow64cpu.dll`** (no 0006): no start reached a window.

| start | outcome |
|---|---|
| 1 | exited with 5 before a window |
| 2 | `Unhandled page fault on read access to 00002DC1 at address 7BF41239`, the mode-switch fault of "The mode switch under Rosetta" in PROGRESS.md, then no window within 240 s |
| 3 | no window within 240 s |

**e. No switch: Fusion's own software display.** These starts used the prefix without `AllowSoftwareRendering`, as on any start without the switch: wined3d gets no pixel format, `mmf2d3d9.dll` and `mmf2d3d8.dll` fail, and the game falls back to its own software display.

| start | title fps | title CPU | menu fps | menu CPU | night fps | night CPU |
|---|---|---|---|---|---|---|
| 1 | 27.38 | 98% | 0.15 | 103% | 0.05 | 104% |
| 2 | 32.09 | 98% | 0.15 | 102% | not reached | |
| 3 | 32.15 | 99% | 0.20 | 103% | 0.05 | 103% |

Start 2 of e: five GO! clicks did not bring up the controls screen within 12 s each (this menu shows a frame every 5 to 7 s), and the script found the game gone afterwards, so its night was not measured. In the night of starts 1 and 3, the picture is wrong: the office is blown out to white with a pale outline over it (`docs/proof/gate1-ucn/e-night.jpg`), where a draws it correctly (`a-night.jpg`). The menu and the title look right in both.

Also measured, not as frame rates: one start of a with macOS's `sample` attached to the game (title 8.83, menu 0.49, night 0.43 fps). The sampler slows the game. It also cannot see inside a Rosetta process: every thread's leaf frame is in Rosetta's translated code. So it gave no profile.

**Summary per configuration** (median of the starts' medians):

| configuration | title | menu | night | CPU of the game |
|---|---|---|---|---|
| a. macron.5, apple-software | 12.5 | 1.36 | 0.62 | 0.9 to 1.05 cores |
| b. stock wined3d | 10.9 | 0.58 | 0.36 | 1.6 to 1.9 cores |
| c. csmt=0x1 | 11.6 | 0.57 | 0.34 | 1.7 to 1.9 cores |
| d. stock wow64cpu | no start reached a window (0 of 3) | | | |
| e. Fusion's software display | 32.1 | 0.15 | 0.05 (drawn wrong) | 1.0 core |

## What a frame is made of

OpenGL calls per presented frame, traced (`ucn-glcount.py` on `trace-m5` and `trace-stock`; frames counted from wined3d's counter; tracing slows the game, so these are counts, not speeds):

| place | build | draws (`glDrawElementsInstancedBaseVertex`) | `glBufferSubData` | `glUniformMatrix4fv` | `glUniform3fv` | texture binds | buffer-map copies | texture uploads |
|---|---|---|---|---|---|---|---|---|
| title | macron.5 | 4.0 | 8.0 | 4.2 | 2.0 | 4.1 | 0 | 0 |
| title | stock | 4.0 | 8.0 | 4.0 | 2.0 | 4.0 | 0 | 0 |
| menu | macron.5 | 2,115 | 4,231 (256 KiB) | 3,867 | 1,926 | 198 | 0 | 2.5 (30 KiB) |
| menu | stock | 2,132 | 4,263 (260 KiB) | 3,917 | 1,959 | 185 | 0 | 4.1 (92 KiB) |
| night | macron.5 | 3,869 | 7,740 (409 KiB) | 7,691 | 3,833 | 49 | 0 | about 1 (500×540) |

- In the night, 8,237 of the 8,260 traced draws were one quad each: 6 indices (`count 6`), 96 bytes of vertices and 12 bytes of indices uploaded just before. The rest drew 2 or 3 quads.
- `get_object` (opengl32 looking up a handle on the Unix side) adds about 6 lines per draw. With the calls above, a draw is about 13 trips through opengl32's 32-to-64-bit thunk: roughly 27,000 a frame in the menu and 50,000 in a night.
- With stock wined3d the night ran at 0.1 fps traced and gave one frame-counter sample, too few to count. Its menu shows the same calls as macron.5's, and no copies.
- So the copies that 0003 removes (410 MB a frame on the 400-sprite probe) do not happen in this game. Neither does any large texture upload. The cost is the number of draws.

## Where the time goes: the same load, layer by layer

`ucn-layers.sh`, all on the same sandbox, at 1024×768. Native programs are `loadprobe` with per-draw uniforms and wined3d's ring upload; "strips" are 1×768 quads (the night's draws are as many as about four full-screen layers of 1-pixel columns, the shape of a panorama drawn column by column, which is what Fusion's perspective effects do; the trace does not give the quads' sizes, so 64×64 quads are measured too).

| load | native arm64, Apple software GL | native x86_64 (Rosetta), Apple software GL | native arm64, Metal | native x86_64, Metal | Wine, D3D9 probe 64-bit | Wine, D3D9 probe 32-bit (WoW64) | the game (a) |
|---|---|---|---|---|---|---|---|
| 4 full-screen quads (title) | 15.2 | 17.1 | 435 | 351 | 18.9 (4 sprites) | 20.2 (4 sprites) | 12.3 to 14.2 |
| 2,100 strips (menu) | 4.69 | 4.25 | 249 | 139 | 1.04 (sprites) | 2.23 (sprites) | 1.14 to 1.38 |
| 3,900 strips (night) | 2.07 | 2.47 | 165 | 104 | | | 0.58 to 0.76 |
| 3,900 quads of 64×64 | 2.90 | 3.36 | 166 | 109 | 0.53 (sprites) | 1.19 (sprites) | |
| 3,900 quads of 1×1 (per-draw cost only) | 68.6 | 64.3 | 167 | 101 | | | |

How each 3,900-strip draw reaches the buffer, Apple software GL: wined3d's ring (next offset of one buffer) 2.07 / 2.47 fps, rewriting one small buffer 2.93 / 3.20, orphaning it first 3.08 / 3.63 (arm64 / x86_64).

What the layers say:

- **The renderer.** Apple's software OpenGL renderer needs 410 to 490 ms for the night's load and 210 to 230 ms for the menu's, natively and under Rosetta alike. Its per-draw overhead is small (3,900 one-pixel quads run at 64 to 69 fps, about 4 µs a draw). The time is in rasterising thousands of thin primitives and in full-screen fill: 4 full-screen layers take 59 to 65 ms. **The title screen is the renderer and nothing else:** 15 to 17 fps native, 12 to 14 in the game.
- **Rosetta, for the renderer:** none measurable. The x86_64 build is as fast as the arm64 one (sometimes faster).
- **Metal on the same virtual GPU** draws the night's load at 104 (x86_64) to 165 (arm64) fps with half a core to one core. So the virtual GPU is not what holds the game back; the lack of a hardware OpenGL renderer is. On this sandbox, a game that reaches the GPU only through OpenGL cannot be shown at 60 fps with this many draws.
- **Wine and wined3d.** The 32-bit probe at 3,900 sprites runs at 1.19 fps (845 ms) where the native program with the same draw count and quad size runs at 2.9 to 3.4 fps (300 to 345 ms). So Wine, wined3d, the WoW64 thunks and Rosetta on that code add about 500 ms a frame, about 130 to 140 µs a draw, on this renderer. The 64-bit probe (no WoW64 thunks, but the command-stream thread on, since 0004 applies only to WoW64) is slower still at 0.53 fps. So the 32-to-64-bit thunks are not the dominant part, and the command-stream thread is a large one (as configurations b and c show on the game).
- **The game against the probe.** A night frame takes 1.3 to 1.7 s in the game. The renderer alone needs 0.41 to 0.49 s for that load (strips take 1.4 to 1.5 times as long as the probe's 64×64 quads on it). The Wine side, measured with the probe at that draw count, is about 0.5 s. The remaining 0.3 to 0.8 s is the game itself and the state the probe does not change (Fusion's shaders, two matrices a draw where the probe sets none). The sandbox gives no profiler inside a Rosetta process, so that rest is not split further.

## What it means for the M3's 5 fps

None of the sandbox's numbers explains a title screen at 5 fps on a real GPU:

- the title is 4 draws and no buffer maps, with any wined3d;
- the command-stream thread does not slow the title here (c: 11 to 14 fps);
- with a hardware renderer, 4 full-screen draws are nothing (Metal: 351 to 435 fps here).

What the M3 had and the sandbox does not is a Retina display, with the window at 1920×1243 points (3840×2486 pixels if the drawable is at full resolution), and Apple's hardware OpenGL instead of the software one. Both are unmeasured. The menu and the nights, at 2,100 to 3,900 draws, carry a per-draw cost on Macron's side (about 140 µs a draw with this renderer, part of which may be waiting on the software renderer) that would by itself keep a night under 60 fps unless it is far smaller on a hardware renderer. 60 fps would need about 4 µs a draw for everything. How much of the 140 µs remains with a hardware renderer can only be measured on a real Mac.

## What a fix would be

No small Wine patch follows from these numbers, so none is written:

- The draws are the game's: Fusion issues one draw per quad. wined3d does not merge them, and merging consecutive `DrawIndexedPrimitiveUP` calls with identical state inside wined3d is a real change to its draw path, not a small patch.
- With wined3d's ring upload, a frame takes about 1.5 times as long on Apple's software renderer as with orphaning the buffer before each draw (native: 2.07 against 3.08 fps on arm64, 2.47 against 3.63 under Rosetta). That points at how Apple's OpenGL handles `glBufferSubData` into a buffer that queued draws still read. Whether a hardware renderer behaves the same is unknown, so a change there would be tuned to the sandbox. It could be a wined3d patch later, measured on a real GPU first.
- The path that removes the OpenGL renderer from the picture is Direct3D 9 on Metal. Metal does this load at 100 to 165 fps here. Today none of Macron's Metal routes takes Direct3D 9: DXVK's d3d9 cannot make a device on MoltenVK (no geometryShader, no shaderCullDistance), wined3d's Vulkan renderer fails in 32-bit on MoltenVK (README), and DXMT is Direct3D 10/11 only. That is the next layer for gate 1 if a real GPU shows the OpenGL path too slow.

## Should Macron turn on Apple's software renderer by itself?

On a Mac without a hardware OpenGL renderer, for this game: yes. Apple's software renderer through Wine's Direct3D 9 (a) is faster than leaving the game to Fusion's own software display (e) everywhere except the title screen:

| place | a (apple-software) | e (software display) |
|---|---|---|
| title | 12.3 to 14.2 fps | 27.4 to 32.2 fps |
| menu | 1.14 to 1.38 fps | 0.15 to 0.20 fps |
| night | 0.58 to 0.76 fps, drawn right | 0.05 fps, drawn wrong |

Both use one core. Conditions, for whoever makes the change:

- Turn it on only when there is no accelerated renderer (`CGLChoosePixelFormat` with `kCGLPFAAccelerated` fails), which is what patch 0002's fallback does inside winemac. Every real Mac has one, so for players this changes nothing; it matters for virtual machines and CI.
- Whether `AllowSoftwareRendering=y` changes anything on a Mac *with* a hardware renderer (the software formats are listed too) was not measured. That is one more reason to set it only when the accelerated check fails.
- Neither path is playable on the sandbox. Choosing a makes the sandbox show the path real Macs take.

## A trap: `MACRON_OPENGL=apple-software` outlives the launch

macron writes `AllowSoftwareRendering=y` into the prefix's registry when `MACRON_OPENGL=apple-software` is set, and never takes it out. A later start without the switch writes no registry values (the profile has none), so the value stays and the next start uses Apple's software renderer anyway. This is the state of Steam's prefix for 871720 on `sbx_qsjpicyv` now: an investigation start an hour before this work set it, so pressing Play there takes Wine's Direct3D path on the software renderer, not the default software display. README.md calls the switch a user switch "like Proton's `PROTON_USE_WINED3D`", which suggests it should last one launch. A fix would write the value as `n` (or delete it) when the switch is off. That is not done here; it changes macron.

## Taking the frame rate from Steam's Play button

For the owner thread, on `sbx_qsjpicyv` (or any sandbox where Steam's prefix for 871720 has `AllowSoftwareRendering=y`, which a single `MACRON_OPENGL=apple-software` start leaves behind):

```sh
touch ~/Library/Application\ Support/Macron/show-fps
# press Play in Steam; Enter on the warning screen, GO! on the menu, GO! on the controls screen
log=$(ls -t ~/Library/Application\ Support/Macron/logs/steam-871720/*.log | head -1)
grep -a 'trace:fps:wined3d_cs_exec_present' "$log"
rm ~/Library/Application\ Support/Macron/show-fps
```

`show-fps` makes macron start Wine with `WINEDEBUG=-all,+fps`, so the log gets wined3d's counter. The switch is the registry value, because Steam's Play cannot pass `MACRON_OPENGL`. Without the value, Play takes the default path (Fusion's software display on a sandbox). That path prints no `+fps` lines, and its frame rate can only be counted with `WINEDEBUG=-all,+bitblt` as in configuration e. On a real Mac with a hardware renderer, the value makes no difference to which renderer is used, and the same commands give the real number.

## What a sandbox cannot show

- The frame rate on a real Apple GPU, with Apple's hardware OpenGL, a Retina display and the window at the M3's size. That is for Thalles, with the release and `show-fps` (README.md).
- Whether Macron's per-draw cost (about 140 µs a draw here) is mostly waiting on the software renderer or mostly Wine and Rosetta CPU time. `sample` cannot look inside a Rosetta process, and no other profiler was tried.
- Whether the remaining third of the night's frame is Fusion itself or the state it sets.

## Reproducing

```sh
# on a Mac sandbox with the game installed by Steam and started once through macron:
~/ucn/scripts/ucn-setup.sh
~/ucn/scripts/ucn-batch.sh a b c d e trace-m5 trace-stock   # about 1 h 15 min
python3 ~/ucn/scripts/ucn-summary.py table ~/ucn/runs/*      # e: UCN_COUNT='NtGdiStretchBlt .*dst [0-9a-fx]+ log=0,0 1024x768.*rop=cc0020' ucn-summary.py result ...
python3 ~/ucn/scripts/ucn-glcount.py ~/ucn/runs/trace-m5 1
clang -O2 -arch arm64 -arch x86_64 -fobjc-arc -framework Cocoa -framework OpenGL -framework Metal -framework QuartzCore -o ~/ucn/loadprobe loadprobe.m
~/ucn/scripts/ucn-layers.sh   # needs probe32.exe and probe64.exe from scenarios/d3d9-fps-probe/build.sh in ~/ucn
```

The scripts expect the layout `~/ucn/scripts/` (this folder's `ucn-*` files), `~/work/bin/macron` and `~/work/bin/drive` (from `ci/steam-play/drive.swift`).
