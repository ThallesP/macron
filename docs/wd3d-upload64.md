# 64-bit wined3d uploads buffer maps through heap memory (Wine patch 0008)

A 64-bit Direct3D 9 program on Apple's OpenGL now fills its dynamic vertex buffers through heap memory that wined3d uploads with `glBufferSubData`, like a 32-bit one already did with patch 0003. Before, each `DrawPrimitiveUP` waited twice for wined3d's command stream thread, which mapped, flushed and unmapped the 512 KB streaming buffer with three OpenGL calls that patch 0007 cannot queue.

Branch `wd3d-upload64`, sandbox `sbx_y2bxfj2u` (macOS 26.7.2, "Apple M1 (Virtual)", 3 cores), 2026-11-16. Every number here is from that sandbox.

## What the trace showed

`probe64.exe up --sprites 3900 --sprite-size 1 --window 1024x768`, macron.6 as shipped (0001 to 0007), `MACRON_OPENGL=apple-software`, `WINEDEBUG=-all,+timestamp,+opengl,+d3d_perf`; the last 30 MB of the trace, 12,153 draws:

- The command stream thread (`wined3d_cs`) makes, for each draw: `glMapBufferRange`, `glFlushMappedBufferRange`, `glUnmapBuffer`, 6 `glBindBuffer` (each map, flush and unmap binds the buffer and unbinds it), then `glActiveTexture`, `glBindTexture`, `glBindSampler`, `glDrawArrays`, and 0.4 to 0.8 of each blend, colour-mask and enable call.
- The application's thread logs, for each draw, `Mapping resource ... flags 0x40001000 through the CS` (`WINED3D_MAP_WRITE | WINED3D_MAP_NOOVERWRITE`) and `Unmapping resource ... through the CS`, each followed by `Waiting for queue 1 to be empty`.

Why: Apple's OpenGL 4.1 has no `ARB_buffer_storage`, so wined3d cannot map buffers persistently. Then `adapter_gl_alloc_bo()` cannot create a buffer object from the application's thread, `wined3d_cs_map_upload_bo()` gives up, and `wined3d_device_context_emit_map()` maps the buffer on the command stream thread and waits for it (`wined3d_device_context_finish(..., WINED3D_CS_QUEUE_MAP)`). The unmap does the same. A 32-bit program never got here: patch 0003 sends its buffer maps through heap memory, because opengl32 copies maps in WoW64.

`glMapBufferRange` and `glUnmapBuffer` return a value, and `glFlushMappedBufferRange` touches mapped memory, so 0007 queues none of them: each is a transition of its own, and each replays the queued calls first.

### Where the 270 µs a draw went

Measured, not profiled: `sample` shows no symbols for the Rosetta-translated Windows threads, so the time was split with runs that each take one piece away.

| run (stock macron.6 wined3d, 64-bit probe) | fps | ms a frame | µs a draw |
|---|---|---|---|
| as shipped (command stream thread on) | 1.06 (3 starts: 1.14, 1.07, 0.98) | 943 | 242 |
| command stream thread off (`WINE_D3D_CONFIG=csmt=0x0`) | 2.94, 2.73 (2 starts) | 352 | 90 |

- **About 150 µs a draw: two synchronous trips to the command stream thread** (map, then unmap), each one also waiting for the previous draw on that thread (`wined3d_resource_wait_idle`).
- **About 80 µs a draw: the OpenGL calls of the map itself.** `glprobe64 --cost` times wined3d's sequence (bind, `glMapBufferRange` of the whole 512 KB buffer with `WRITE | FLUSH_EXPLICIT | UNSYNCHRONIZED`, a 32-byte write, unbind, bind, `glFlushMappedBufferRange`, unbind, bind, `glUnmapBuffer`, unbind, the draw) at 7.3 µs a call, 80 µs a draw. Three of those calls cross to the Unix side at 13 to 14 µs each (about 40 µs); the rest is Apple's OpenGL mapping and unmapping the buffer, with the queued draw replayed in between.

## What changed

`runtime/wine/patches/0008-wined3d-Upload-buffer-maps-through-heap-memory-without-persistent-maps.patch`. Patch 0003 added `d3d_info->copied_maps` (no persistent maps, and a WoW64 process) and used it in two places. 0008 renames it `heap_buffer_maps` and sets it whenever buffers cannot be mapped persistently, whatever the process:

- `wined3d_cs_map_upload_bo()`: a buffer the application cannot read, and whose system memory is not pinned, is mapped through heap memory the size of the mapped range. The map returns at once, and the unmap queues the upload to the command stream thread without waiting for it.
- `wined3d_context_gl_copy_bo_address()`: the command stream thread uploads that memory with `glBufferSubData`, which 0007 queues (up to 32 KB), instead of mapping the whole buffer object for a `DISCARD`.

In a WoW64 process `heap_buffer_maps` has exactly the value `copied_maps` had (`!persistent_map`, since the process is WoW64), so 32-bit programs take the same path as with 0003. On a host with persistent maps nothing changes. Vulkan and no3d adapters never set it. `runtime/wine/build-overlay.sh` applies every patch in the folder, so it builds 0008 with no change; the overlay holds the same files as before.

`tools/wow64-gl-probe/glprobe.c` has two new `--cost` rows: `up-draw-map-11-calls` (the stock sequence above) and `up-draw-subdata-3-calls` (with 0008: bind, `glBufferSubData` of 32 bytes, draw). `tools/gl-batch/measure-wined3d.sh` is `measure.sh` swapping `wined3d.dll` instead of `opengl32`.

### With 0008, per draw

The same trace with 0008 (`WINEDEBUG=-all,+timestamp,+opengl,+d3d_perf`, 25,375 draws): `glActiveTexture`, `glBindTexture`, `glBindSampler`, `glBindBuffer`, `glBufferSubData`, `glDrawArrays`, and the same 0.4 to 0.8 state calls: the 32-bit program's sequence, all of it queued by 0007. The application's thread logs `Mapping buffer ... through heap memory` and never waits for the command stream. What still crosses one by one is once a frame: `wglGetPixelFormat`, `glFenceSync`, `glClientWaitSync`, the swap.

## Measurements

All runs: `MACRON_OPENGL=apple-software`, `WINEDEBUG=-all`, started through `macron launch` (profiles `d3d9-fps-probe` and `wow64-gl-probe`), runtime macron.6 as `macron runtime install` fetches it. The two `wined3d.dll` builds (i386 and x86_64, swapped together into both runtime trees and every prefix):

- **stock**: macron.6's, 0001 to 0007 (i386 `ebdec7b5...`, x86_64 `bec64faf...`).
- **0008**: built on the sandbox by `build-overlay.sh` with 0001 to 0008 (i386 `a6ed4775...`, x86_64 `e563b066...`; overlay `9f69b038...`). Its checks all passed (`docs/proof/wd3d-upload64-sbx_y2bxfj2u/abi-check.txt`): `wined3d.dll` exports the same 377 symbols as Gcenx's in both architectures, and 0001, 0002, 0005, 0006 and 0007's checks still pass.

`tools/gl-batch/measure-wined3d.sh 3`: three rounds, the build order swapped every round, every Wine process killed between runs.

### Direct3D 9 with 3,900 one-pixel sprites

`probe32.exe` / `probe64.exe up --sprites 3900 --sprite-size 1 --window 1024x768 --warmup 5 --seconds 10`. The reference is `loadprobe` (`tools/clickteam-games/loadprobe.m`, `api=gl-sw draws=3900 quad=1x1 uniforms=1`): the same OpenGL work in a native Mac program without Wine, once a round.

| | run 1 | run 2 | run 3 | mean | share of loadprobe x86_64 |
|---|---|---|---|---|---|
| probe64, stock | 1.14 | 1.07 | 0.98 | **1.06 fps** | 0.015 |
| probe64, 0008 | 38.82 | 31.78 | 32.09 | **34.2 fps** | 0.49 |
| probe32, stock | 33.43 | 29.27 | 24.11 | **28.9 fps** | 0.41 |
| probe32, 0008 | 35.39 | 28.62 | 25.35 | **29.8 fps** | 0.42 |
| loadprobe, x86_64 (Rosetta) | 82.36 | 59.60 | 69.20 | **70.4 fps** | 1 |
| loadprobe, arm64 | 61.94 | 72.96 | 77.41 | **70.8 fps** | |

The 64-bit probe runs 32 times faster: 943 ms a frame becomes 29 ms, and it is now a little faster than the 32-bit one instead of 27 times slower. The 32-bit probe is unchanged within the sandbox's noise, as expected: its path is the same code. Diagnostic single starts the same hour gave the same picture (stock 1.25 and 1.26 fps, 0008 41.1 and 40.7 fps).

This sandbox is faster than `sbx_hzfsgybj` was for `docs/gl-batch.md` (probe32 with 0007: 28.9 fps here, 23.2 there; loadprobe x86_64 70.4 here, 63.1 there), so compare within this file.

### Cost of one OpenGL call

`glprobe32.exe` / `glprobe64.exe --rounds 1 --gets 1 --cost 20000`, a 3.2 core context, drawing into a framebuffer object; the time of 20,000 calls, `glFinish` included, divided by the number of calls. glprobe does not load wined3d, so both columns measure the same thing; they show the sandbox's state and its noise (one outlier round, the third 0008 glprobe32 run, raises that column). Mean of 3 runs, ns:

| call | 32-bit, stock round | 32-bit, 0008 round | 64-bit, stock round | 64-bit, 0008 round |
|---|---|---|---|---|
| `glGetIntegerv` (never queued) | 12,786 | 14,031 | 13,426 | 13,234 |
| `glUniform1f` | 133 | 213 | 127 | 140 |
| `glBindBuffer` | 205 | 258 | 203 | 226 |
| `glBufferSubData`, 64 bytes | 131 | 149 | 114 | 120 |
| `glUniformMatrix4fv` | 173 | 219 | 160 | 163 |
| the nine calls of a wined3d draw in Ultimate Custom Night, per call | 191 | 239 | 186 | 190 |
| `up-draw-map-11-calls`: stock 64-bit wined3d's upload and draw, per call | 10,935 | 12,415 | 7,304 | 7,669 |
| `up-draw-subdata-3-calls`: the upload and draw with 0008, per call | 202 | 207 | 182 | 182 |

Per draw, the stock 64-bit upload costs 11 × 7.3 = 80 µs and 0008's 3 × 0.18 = 0.55 µs, before counting the two trips to the command stream thread that 0008 also removes. (In 32-bit the map sequence is dearer still, 11 × 10.9 = 120 µs, because opengl32 copies the mapped range in WoW64: that is what 0003 took away.)

Every run's figures are in `docs/proof/wd3d-upload64-sbx_y2bxfj2u/results.txt`.

## Correctness

`macron-check run` for each scenario (the checks `macron-check sandbox` runs, on the one sandbox in use), with the 0008 build in both runtime trees and every prefix; the `wined3d.dll` hashes were recorded before and after each check and were 0008's every time:

| scenario | result | SSIM | colour distance | notes |
|---|---|---|---|---|
| `d3d9-fps-probe` (32-bit, Direct3D 9 → wined3d → `opengl32`, `mix` mode) | PASSED | 0.995 | 0.001 | Frame rate 11.8 fps against 11.2 for the 64-bit baseline (needs ≥ 0.80 of it, so ≥ 8.9); the throttled control was rejected; every negative control was rejected. |
| `d3d11-probe` | PASSED | 0.996 | 0.000 | |
| `openttd` | PASSED | 0.944 | 0.001 | 5 of 5 menu texts |

No check, threshold or reference was changed.

Two more runs, for what the scenario does not show:
- **The 64-bit picture.** `d3d9-fps-probe` compares the 32-bit probe's frame only. A copy of the scenario outside the repository, with `probe64.exe` as the program and `probe32.exe` as the baseline, passed with 0008 against the same Windows reference: SSIM 0.995, colour distance 0.001, every negative control rejected, 13.7 fps against 13.2.
- **The scenario with stock wined3d**, for the margin before: PASSED, 16.3 fps against a 64-bit baseline of 6.4.

That margin is the one thing 0008 changes in the checks. The frame-rate check asks the 32-bit probe for at least 0.80 of the 64-bit one; the reference ratio is from a Windows runner, 0.885. Before 0008, Apple's OpenGL made the 64-bit probe 2.5 times slower than the 32-bit one, so the check passed easily. With 0008 the two run about level, as on Windows, so the check now really tests what it was meant to test, with less room. `mix` mode, 3 alternating rounds (`mix --window 960x540 --warmup 5 --seconds 10`, `docs/proof/wd3d-upload64-sbx_y2bxfj2u/mix-results.txt`):

| | run 1 | run 2 | run 3 | mean |
|---|---|---|---|---|
| probe32, stock | 14.81 | 14.72 | 11.66 | **13.7 fps** |
| probe32, 0008 | 15.34 | 14.21 | 13.93 | **14.5 fps** |
| probe64, stock | 5.62 | 4.91 | 5.70 | **5.4 fps** |
| probe64, 0008 | 16.13 | 12.62 | 14.58 | **14.4 fps** |

The 32-bit probe is unchanged here too. Across these runs, its ratio to the 64-bit one with 0008 is 0.95 to 1.13.

Summaries: `docs/proof/wd3d-upload64-sbx_y2bxfj2u/*-summary.md`.

## Not done, and uncertain

- The patched overlay is not published and nothing is pinned: `internal/wineruntime/manifest.go` still pins macron.6 and `runtime/wine/release.env` is unchanged. Publishing means a new `RELEASE_TAG`, the Wine build workflow from `main`, and a new pin.
- A `NOOVERWRITE` map through heap memory starts with undefined contents, and the whole mapped range is uploaded on unmap. A program that maps a range with `NOOVERWRITE` and writes only part of it would have the rest overwritten with garbage. wined3d's own streaming buffer (`DrawPrimitiveUP`, `DrawIndexedPrimitiveUP`) maps exactly what it writes, as does the probe. 32-bit programs have had this behaviour since 0003, and 64-bit programs on Apple's OpenGL have it now; no program has been seen to do the partial write, but none beyond Macron's probes was run here (no Steam on this branch).
- 64-bit wined3d keeps its command stream thread; with 0008 the application's thread no longer waits for it on buffer maps. Texture maps (`LockRect` on a dynamic texture) still go through the command stream and wait, as before.
- Ultimate Custom Night is 32-bit, so 0008 does not change it.
- Once 0008 ships, `d3d9-fps-probe`'s frame-rate check has less room on the CI runners (the 32-bit probe at about 1.0 of the 64-bit one, against 0.80 needed, where it was about 2.5). Only one sandbox was measured; how the runners' noise sits against that margin will show when the Scenarios workflow runs on a build with 0008.
