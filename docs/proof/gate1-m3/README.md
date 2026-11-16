# Ultimate Custom Night on a real Apple GPU (Thalles's MacBook Air M3), 2026-11-16

Taken on Thalles's own Mac at his request ("feel free to launch the game and do all the diagnostics"), with Macron 0.4.0 from the release (runtime `wine-11.21-macron.6`, so Wine patches 0001 to 0007), macOS 26.5.2, SIP on, not a virtual machine. He had installed 0.4.0 himself, started the game from Steam and said of the OpenGL batching: "it didn't improve at all".

## What was measured

- `diagnose.txt`: `macron diagnose`. **The transition cost the sandboxes showed is a virtual-machine cost.** A macOS system call under Rosetta is 0.18 µs here (5.6 to 5.9 µs on the sandboxes), the `thread_set_tsd_base` trap 0.16 µs (5.7 to 6.1), a Wine system call 0.62 to 0.89 µs (20 to 25). So 32,000 OpenGL calls a frame cost about 15 ms of transitions here, not 0.45 s, and batching them (patch 0007) could not be what the game was waiting for.
- `menu-fps.txt`, `menu.png`: the game started as Steam starts it (`SteamAppId=871720`, `STEAM_COMPAT_DATA_PATH`, `macron waitforexitandrun`), frame counter on, Thalles at the character-select menu: **5.0 to 5.2 fps**, the game's process at about 75% of a core.
- `night-fps-with-sampler.txt`, `night.png`, `sample-game-thread.txt`: the same in a night, with macOS's `sample` attached for 10 s (the sampler itself halves the frame rate: about 5 fps before it attaches, 2.4 to 2.8 while it runs).
- `loadprobe-results.txt`: `tools/clickteam-games/loadprobe.m`, a native program with no Wine in it, drawing a night's load (3,900 small quads, vertices uploaded before each draw, uniforms set per draw) on the same GPU.

## The cause

Apple's OpenGL on Apple silicon is a layer over Metal (`AppleMetalOpenGLRenderer`, "OpenGL 4.1 Metal"). In it, `glBufferSubData` on a buffer that draws already issued still use does two things every time (`sample-game-thread.txt`; the game's thread has 1,502 samples, 1,163 listed under the thread and 338 under Metal's command queue, which runs on the same thread):

- `gldBufferSubData` → `flushResource` → `flushContext` → `-[_MTLCommandQueue submitCommandBuffer:]`: 318 samples. It submits the context's command buffer.
- `gldBufferSubData` → `getPreprocessingCommandBuffer` → `-[_MTLCommandQueue commandBufferWithUnretainedReferences]` → `_dispatch_semaphore_wait_slow`: 574 samples. It asks for a new command buffer and waits for the GPU to finish an earlier one.

So 892 of 1,502 samples (59%) are inside `glBufferSubData`, and 574 (38%) are a wait. (An earlier version of this file divided by 1,163 and said 77% and 49%.) wined3d uploads each draw's vertices and indices with `glBufferSubData` (through `wow64_flush_batch` → `wow64_ext_glBufferSubData` with patch 0007); a night is about 3,900 draws a frame. The draws themselves are 43 samples.

The Metal HUD (`libMTLHud.dylib`) is in those stacks because `show-fps` turns it on. It is not the cause: the 2026-11-14 measurement on this Mac, taken with `WINEDEBUG=fps` alone, was the same 5 fps, and `loadprobe` below shows the same cost with no HUD and no Wine.

## The same thing without Wine, and what avoids it

`loadprobe` on the M3, 3,900 draws a frame, 1024x768, built for x86_64 (Rosetta, as Wine runs) unless noted:

| how each draw's vertices are uploaded | frames a second |
|---|---|
| `glBufferSubData` at the next offset of a large buffer (`vb=ring`, what wined3d does) | 3.0 |
| the same, arm64 | 3.1 |
| `glBufferSubData` at offset 0 of a small buffer (`vb=sub0`) | 3.0 |
| `glBufferData(NULL)` first, then `glBufferSubData` (`vb=orphan`) | 60 |
| `glMapBufferRange` with `GL_MAP_UNSYNCHRONIZED_BIT \| GL_MAP_INVALIDATE_RANGE_BIT`, `memcpy`, `glUnmapBuffer` (`vb=mapring`) | 120 |
| the same with `GL_MAP_FLUSH_EXPLICIT_BIT` (`vb=mapring-flush`) | 120 |
| the same map without `GL_MAP_UNSYNCHRONIZED_BIT` (`vb=mapring-sync`) | 1.6 |
| Metal, for comparison (`api=metal`) | 40 |
| `vb=ring` with 4 draws a frame | about 1,450 |

So the cost is per upload, it is the same with and without Rosetta, and an unsynchronised map avoids it completely: 3 fps becomes 120.

## What an unattended start shows

Started with no input, the game goes to its character-select menu by itself, so starts without a person measure the menu: 4.9 to 5.2 fps on 0.4.0. A `+opengl` trace there (2026-11-16, 3.5 s): 9,186 draws a second at 4.1 fps with the trace on, so about 2,240 draws a frame, each with two `glBufferSubData` (64,284 uploads for 32,142 draws). With `WINEDEBUG` set to anything but `-all`, wined3d also calls `glGetError` after every OpenGL call (194,397 in that window), which empties the batching queue each time; frame rates taken with debug channels on are lower than a normal start's.

## Other paths tried on this Mac

- `MACRON_BACKEND=dxvk`: DXVK 1.10.3 logs `DxvkAdapter: Failed to create device` twice and the game's window stays white. The same as on the sandboxes.
- `MACRON_BACKEND=wined3d WINE_D3D_CONFIG=renderer=vulkan`: the game exits with 3 before it opens a window.

## What this means for the patches

- Patch 0007 (batching) is harmless here but was aimed at a cost this Mac does not have.
- 32-bit programs' uploads reach Apple's OpenGL as `glBufferSubData` (the game's `+opengl` trace has no buffer maps), and patch 0008 (on `main`, not released) gives 64-bit programs that same upload path instead of buffer maps. On a real Apple GPU that is the slow path: 0008 as it stands should be expected to make 64-bit Direct3D 9 games slower on real Macs, whatever it does on the sandboxes' software renderer.
- The fix is in how wined3d's streaming uploads reach Apple's OpenGL: an unsynchronised write into the buffer (for 32-bit programs done on Wine's Unix side, so no pointer above 4 GB crosses to the program), with the buffer orphaned when Direct3D discards it.

Nothing from Thalles's Steam account is in these files; `macron diagnose` replaces the home folder and user name, and the two pictures are the game's own window.
