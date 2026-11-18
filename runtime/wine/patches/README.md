# Macron's patches to Wine

These patches apply to Wine 11.21 (tag `wine-11.21`, https://gitlab.winehq.org/wine/wine/-/tree/wine-11.21). Like Wine, they are licensed under the GNU Lesser General Public License 2.1 or later (see `COPYING.LIB` in Wine's source). Every build Macron ships is published with them on the `runtime-wine-*` releases of this repository.

## 0001: export `macdrv_functions` for DXMT

DXMT (Direct3D 10/11 over Metal) draws each frame into a Metal view inside the window's client area. To make that view, its `winemetal.so` looks up the Mac driver with `dlsym(RTLD_DEFAULT, "macdrv_functions")`, a table of function pointers in the layout CrossOver uses, and reads `hwnd`, `cocoa_window`, `cocoa_view`, `client_cocoa_view` from the start of the struct that `get_win_data` returns. Upstream Wine builds `winemac.so` with hidden visibility, so DXMT finds nothing: its device works but its swap chain logs `Failed to create metal view, it seems like your Wine has no exported symbols needed by DXMT` and nothing reaches the screen.

Exporting Wine's own functions would not be enough, because Wine 11's `struct macdrv_win_data` starts with `hwnd`, `cocoa_window`, `client_view`, `rects`: DXMT would take part of `rects` for the client view. Wine 11 also no longer keeps a client-area view per window; it makes one only for an OpenGL or Vulkan surface.

The patch:

- adds `dlls/winemac.drv/dxmt.c`, which exports `macdrv_functions` with DXMT's layout. Its `get_win_data` locks the window data the way Wine's does and returns a `struct macdrv_dxmt_win_data` (DXMT's layout) kept inside Wine's `struct macdrv_win_data`. Its `client_cocoa_view` is a view over the client area, made on first use, moved and resized with the window, and disposed with the Cocoa window. Child windows, which have no Cocoa window of their own, get an empty struct, so DXMT gets no view instead of crashing. Two entries DXMT does not use are NULL (`macdrv_init_display_devices`, `on_main_thread`).
- makes `macdrv_view_release_metal_view` clear the view's cached Metal view, so a second swap chain on the same window gets a new Metal view instead of the one DXMT released.

Patch 0001 changes only `winemac.so`. The Windows side of the driver, its unix call table and every other module are untouched, which is what lets Macron ship the patched `winemac.so` as a drop-in for Gcenx's wine-devel 11.21 build (upstream 11.21 plus one change to `dlls/win32u/vulkan.c`). `../check-abi.sh` checks that on every build.

## 0002: fall back to the software renderer when `AllowSoftwareRendering` is on

`init_gl_info()` reads the OpenGL version from a context that `init_context()` makes with `kCGLPFAAccelerated`. On a Mac without an accelerated renderer, such as GitHub's macOS runners (a paravirtual GPU), that fails, and with it all of Wine's OpenGL, even when `HKCU\Software\Wine\Mac Driver\AllowSoftwareRendering` is `y` and the software renderer's pixel formats are listed. With the setting on, the patch retries with Apple's generic float renderer. A Mac with an accelerated renderer takes the same path as before. Macron uses it only on the runners, so the frame-rate check can run the path a Direct3D 9 game takes on a real Mac (wined3d on Apple's OpenGL, in Wine's 32-on-64 mode) instead of a Windows OpenGL DLL that never leaves the 32-bit side.

## 0003: upload buffer maps through heap memory when buffer maps are copied

A 32-bit program on 64-bit Wine (WoW64) cannot use a pointer above 4 GB. When the host OpenGL can neither map buffers at a client-chosen address (`GL_MESA_map_buffer_client_pointer`) nor persistently, Wine's `opengl32` emulates `glMapBufferRange` for such programs: `wow64_map_buffer` copies the mapped range into low memory on map, and `wow64_unmap_buffer` copies it back on unmap. Apple's OpenGL has neither, so every 32-bit Direct3D game on a Mac goes through this. Without persistent maps, wined3d serves a `DISCARD` or `NOOVERWRITE` map of a buffer (each `DrawPrimitiveUP`, each dynamic vertex buffer lock) by mapping the whole buffer object, without an invalidate flag, so each one copies the whole streaming buffer (512 KB or more) twice. Ultimate Custom Night (32-bit, Direct3D 9) presented about 5 frames a second at about 185% CPU on an M3 this way.

When the process is WoW64 and buffers cannot be mapped persistently, the patch maps buffers that the application cannot read, and whose system memory is not pinned, through heap memory the size of the mapped range, the way `UpdateSubresource` already works: the command stream thread uploads it on unmap with `glBufferSubData`, which takes the 32-bit pointer as it is, and the application's thread does not wait for the command stream. An upload of a whole buffer (`DISCARD`) also uses `glBufferSubData` instead of mapping the buffer object. A 64-bit program, or a host with persistent mapping, takes the same path as before (0008 gives 64-bit programs this path too).

A first version pinned dynamic buffers in system memory instead. It made every map wait for the command stream thread and hung the probe on the runner, so it was dropped.

## 0004: run the command stream on the application's thread in a WoW64 process

**Reverted by 0011** (2026-11-16): the crashes it worked around were 0006's far jumps, and on a real Mac the command stream's own thread is what takes Ultimate Custom Night's menu to 60 fps once 0010 removed the per-draw waits.

On macOS, under Rosetta, a 32-bit Direct3D program on 64-bit Wine crashes or stops making progress when wined3d's command stream runs on its own thread (CSMT, wined3d's default). The crash is the application's thread jumping to an address made of two 32-bit pointers read as one 64-bit value (`rip=7a83c7307a83c670`). Measured on GitHub's runner with the frame-rate probe in `mix` mode, which locks a dynamic texture every frame and so waits for the command stream thread: with the thread, 7 of 7 runs crashed or stalled; without it (`WINE_D3D_CONFIG=csmt=0x0`), 6 of 6 ran at 8.4 to 8.7 frames a second, the fastest of all configurations. The patch turns the command stream thread off by default in a WoW64 process; the `csmt` setting (registry or `WINE_D3D_CONFIG`) still overrides it. 64-bit programs keep it.

It lowers the failure rate but does not remove the crash. In a second batch without the command stream thread, 4 of 12 runs still crashed while the device was being created: the fault is in 32-bit `opengl32.dll` (`addr=7A77ADEF`, the module at `7A6D0000`), reading `0BEC988B0014A018`, again two 32-bit values read as one 64-bit address, as if 32-bit code came back from a call into Wine's 64-bit side still in 64-bit mode. Over both batches without the thread, 4 of 20 runs crashed and 2 of 20 stalled before their device was ready; with the thread, 7 of 8 crashed or stalled. 64-bit runs and 32-bit runs through Mesa llvmpipe, which never calls into the 64-bit side for OpenGL, did not crash. Whether a Mac with a real GPU sees the crash is not known: the runner draws with Apple's software renderer, whose shader compiler generates x86 code inside the Rosetta-translated process. See PROGRESS.md.

`wined3d.dll` is a Windows module, built for i386 and x86_64. `../check-pe-exports.sh` checks on every build that an unpatched build and the patched one export exactly what Gcenx's does.

## 0005: don't retain an OpenGL context that is being freed

When the last reference to an OpenGL context goes away off the main thread, the Mac driver hands the context to the main thread in a block while it is being freed, and the main thread releases it after the memory is gone. In detail: `-[NSOpenGLContext dealloc]` calls `-clearDrawable`; Wine's `WineOpenGLContext` overrides it to run `[super clearDrawable]` on the main thread with `OnMainThread(^{ ... })`, and copying that block retains `self`. `OnMainThread` wakes the waiting thread before the main thread releases the block, so the waiting thread finishes `dealloc` and frees the context, and the main thread's release then runs `objc_release` on freed memory. The process dies with `c0000005 in system thread at 0x7ff81ad7a5e2` (`objc_release + 0x22`, read by resolving the address in an x86_64 process on the same runner). This happens in `wglDeleteContext` or thread cleanup when no main-thread block still holds the context, which is what wined3d does while it creates its adapter: a context on a hidden window, then deleted.

How it was found, on GitHub's runner with Apple's software renderer (`MACRON_OPENGL=apple-software`):

- `tools/wow64-gl-probe/glprobe.c` does what wined3d does to set up OpenGL (a hidden window, a legacy context, `wglCreateContextAttribsARB` for a 3.2 core context, `glGetString`, `glGetStringi`, `glGetIntegerv`), 20 rounds per start. Started through `macron launch` (profile `wow64-gl-probe`), its 32-bit build crashed in 23 of 30 starts, every one before it finished.
- `+seh` put every traced crash (7 of 7, then 6 of 6) on the thread without a TEB, which on macOS is the Cocoa main thread, inside `objc_release`. With `NSZombieEnabled=YES` the message names the object: `-[WineOpenGLContext release]: message sent to deallocated instance`.
- A diagnostic build that logged every retain and release of a `WineOpenGLContext` with its backtrace showed, in every one of 10 starts, two contexts retained at a count of 0: `-[WineOpenGLContext dealloc]` → `-[NSOpenGLContext dealloc]` → `-[WineOpenGLContext clearDrawable]` → `OnMainThread`.
- The 64-bit build of the same program exited 0 in 20 of 20 starts, yet 11 of them logged the same fault while the process was exiting. So the bug is not specific to WoW64; a 32-bit program under Rosetta hits it while still running.

The patch marks the context as deallocating and, in that state, reaches the main thread through a `__block` variable, which a block does not retain under manual reference counting. Measured with starts alternating between the shipped `winemac.so` (0001 and 0002) and one built the same way with 0005: the 32-bit program crashed in 23 of 30 starts before and 0 of 30 after; with the trace, contexts retained at a count of 0 went from 2 per start to 0 in 3 starts.

0005 does not fix the crash described under 0004, which is the main failure of the Direct3D 9 frame-rate probe: in the same alternating batch the 32-bit probe failed 14 of 30 starts before and 12 of 30 after. That crash is a mode switch between 32-bit and 64-bit code that does not take effect: the 32-bit thread enters Wine's 64-bit `unix_call_32to64` (`wow64cpu.dll + 0x1239`) still in 32-bit mode, or comes back to 32-bit `opengl32.dll` still in 64-bit mode. PROGRESS.md ("The 32-bit OpenGL crashes") has the evidence.

Like 0001 and 0002, 0005 changes only `winemac.so`, which `../check-abi.sh` checks. The runtime puts the patched `winemac.so` in its DXMT tree only, so profiles on the plain tree (`backend: wined3d`) keep Gcenx's.

## 0006: switch between 32-bit and 64-bit code with `iret` instead of far jumps

In a 32-bit program on 64-bit Wine (WoW64), every system call and every Unix call (how a Windows module such as `opengl32.dll` calls its Unix side) leaves 32-bit code for 64-bit code in `wow64cpu.dll` and comes back. Wine 11.21 does both with far jumps:

- in: the thunk that `Wow64Transition` and `__wine_unix_call_dispatcher` point at is `ljmp *addr` (`ff 2d`, a 6-byte far pointer to `syscall_32to64` or `unix_call_32to64` and the 64-bit code selector);
- out: `syscall_32to64` and `unix_call_32to64` go back with `ljmp *(%r14)` (`41 ff 2e`) to the saved 32-bit `eip` and `cs`. Only the path after a context change (`WOW64_CPURESERVED_FLAG_RESET_STATE`) and the thread's first entry use `iretq`.

Under Rosetta a far jump between the two code segments sometimes runs its target in the old mode. The 32-bit Direct3D 9 frame-rate probe on Apple's software renderer hits it while it creates its device, both ways (PROGRESS.md, "The 32-bit OpenGL crashes"):

- in: `Unhandled page fault on read access to 00002DC1 at address 7BF41239`. `wow64cpu.dll + 0x1239` is `movl cs32_sel(%rip),%edx` in `unix_call_32to64`; read as 32-bit code, `8b 15 c1 2d 00 00` loads from the absolute address `0x2dc1`. The thread arrived through the thunk's `ljmp` still in 32-bit mode.
- out: the fault at the start of `set_context_attribute()` in 32-bit `opengl32.dll` (`mov %fs:0x18,%eax` decoded as 64-bit code), and the jumps to two 32-bit values read as one address in 0004's notes: the `ljmp` back left the thread in 64-bit mode. Without `+seh` this direction mostly shows as a silent exit 5, because Wine cannot dispatch the exception.

The patch enters 64-bit code with `iretl` (the thunk becomes `pushfl; pushl $cs64; pushl $target; iretl`, the same 12 bytes) and returns with `iretq` on the fast path as well, building the frame the reset-state path already builds. The stacks the 64-bit code sees are the same as before: `iretl` in 32-bit mode pops `eip`, `cs` and the flags, and the `iretq` frame sits in the 0x28 bytes `BTCpuSimulate` reserves.

Measured on a GitHub macos-26 runner (sandbox sbx_uqstb45i, runtime macron.4, `MACRON_OPENGL=apple-software`): `probe32.exe mix --warmup 1 --seconds 1` through `macron launch`, 40 rounds, each round one start per build in rotating order, every Wine process killed and the prefix replaced by a fresh copy before each start, `wow64cpu.dll` swapped in both runtime trees and the prefix:

| `wow64cpu.dll` | failed starts | how |
|---|---|---|
| macron.4's (Gcenx's build, far jumps both ways) | 25 of 40 | 10 crash, 6 exit 1, 6 exit 5, 3 stall |
| `iretq` out, `ljmp` in | 14 of 40 | 7 crash, 7 exit 1 |
| `iretl` in, `iretq` out (this patch) | 0 of 40 | |
| `lret` in, `iretq` out | 0 of 40 | |

In a second batch with `WINEDEBUG=+seh`, 12 rounds: 3 of 12 failed with macron.4's, 3 of 12 with `iretq` only on the way out, all six the page fault at `unix_call_32to64`'s `movl cs32_sel(%rip)` (the way in); 0 of 12 with each of the two other builds. So changing only the way out removes the exit-5 starts (the way out) but not the crash on the way in, and the far jump in is the more frequent failure. `lret` in did as well as `iretl`; the patch uses `iret` both ways, one form for both directions. The `wow64cpu.dll` that `../build-overlay.sh` then built with 0001 to 0006 (overlay SHA-256 `869bbd8d...`, not published) failed 0 of 15 starts in another alternating batch, macron.4's 10 of 15.

No regression measured: the 32-bit probe with 10 s measured ran at 8.97 fps on average with the patch (6 of 6 starts, 8.57 to 9.11) against 8.93 with macron.4's (4 starts, 8.46 to 9.11; its other 2 of 6 failed), and the 64-bit probe, which does not load `wow64cpu.dll`, passed 20 of 20 starts with each build. Why Rosetta misses the mode change on a far jump is not known. `tools/wow64-gl-probe` never reproduced it with plain system calls, nor with system calls next to a thread that keeps starting threads (each one writes an LDT entry for its `%fs`, and the 32-bit code selector is an LDT entry too on macOS).

`wow64cpu.dll` is a Windows module (x86_64 only); `../check-pe-exports.sh` checks that it exports what Gcenx's does. The runtime copies it into both Wine trees, like `wined3d.dll`.

## 0007: queue OpenGL calls that return nothing and replay them in one Unix call

Every OpenGL call a Windows program makes goes from `opengl32.dll` to `opengl32.so`. On macOS under Rosetta that transition costs about 14 µs: Wine's dispatcher switches `%gs` with two `thread_set_tsd_base` Mach traps and saves and restores MXCSR. Ultimate Custom Night's night frame makes about 32,000 such calls (PROGRESS.md, "Gate 1, what the sandbox shows").

The patch adds `dlls/opengl32/batch.c`. `UNIX_CALL` on the Windows side now goes through `opengl_unix_call()`, which queues some calls per thread in a 128 KB queue and replays the queue in one Unix call (`flush_batch`, or `wow64_flush_batch` for a 32-bit program) before any other call. A call is queued when it returns nothing and its arguments are values or point to data whose size the call gives (`glBufferSubData`, `glUniform*v`, matrices, fixed-size vectors); that data is copied.

Draws are queued only while no vertex array in the process was ever given a client pointer. Elements draws also need an element array buffer known to be bound.

Everything else replays the queue first: calls that return something (`glGetError` among them), calls that touch client memory later, context changes, `wglSwapBuffers`, `glFlush`, `glFinish`, thread exit. The replay calls the same Unix entry points the calls would have reached directly, with the params the Windows side built, so order and behaviour are unchanged. `dlls/opengl32/batch_table.h` says which calls may be queued. `../gl-batch-table.py` generates it from `unixlib.h` and `unix_thunks.h`. `WINE_OPENGL_BATCH=0` turns batching off.

Measured on sandbox sbx_hzfsgybj (Apple M1 virtual machine, macOS 26.7.2, `MACRON_OPENGL=apple-software`, through `macron launch`), three alternating rounds; `docs/gl-batch.md` has every number:
- **Queued calls:** 0.13 to 0.27 µs, against 13 to 19 µs before, in 32-bit and 64-bit programs. `glGetIntegerv`, never queued, still costs about 15 µs.
- **Direct3D 9 probe, 32-bit:** with 3,900 one-pixel sprites (`up --sprite-size 1 --window 1024x768`), 1.58 fps before and 23.2 fps after.
- **Same probe, 64-bit:** 0.56 to 0.95 fps. Its draws map buffers, and a map can never be queued.
- **Without Wine:** the same OpenGL work in a native x86_64 program (`loadprobe`) ran at 63 fps.

The `d3d9-fps-probe`, `d3d11-probe` and `openttd` scenario checks passed with it.

The patch changes the numbering of `opengl32`'s Unix calls, so `opengl32.dll` (i386, x86_64) and `opengl32.so` must come from the same build. The overlay carries all three, and the runtime puts them in both Wine trees. `../check-unix-lib.sh` checks that `opengl32.so` imports, exports, links and searches what Gcenx's does. `../check-pe-exports.sh` checks that the DLLs export what Gcenx's do.

## 0008: upload buffer maps through heap memory whenever buffers cannot be mapped persistently

Apple's OpenGL has no `ARB_buffer_storage`, so wined3d cannot map buffers persistently. Then it cannot create a mapped buffer object from the application's thread either, and a 64-bit program's `DISCARD` or `NOOVERWRITE` buffer map (each `DrawPrimitiveUP`, each dynamic vertex buffer lock) goes through the command stream: the application's thread waits for the command stream thread to map the whole buffer object, and waits again for it to flush and unmap it. `glMapBufferRange`, `glFlushMappedBufferRange` and `glUnmapBuffer` each cross to the Unix side on their own, because 0007 cannot queue them. An `+opengl,+d3d_perf` trace of the 64-bit Direct3D 9 probe shows exactly that for every draw: three such calls and six binds on the command stream thread, and two `Waiting for queue 1 to be empty` on the application's thread. About 242 µs a draw: about 150 µs for the two waits, about 80 µs for the map's OpenGL calls.

0003 already sent such maps through heap memory in a WoW64 process. 0008 renames 0003's `copied_maps` to `heap_buffer_maps` and sets it whenever buffers cannot be mapped persistently, in any process. The map returns heap memory the size of the mapped range at once, and the command stream thread uploads it on unmap with `glBufferSubData`, which 0007 queues. In a WoW64 process the flag has the value it had with 0003, so 32-bit programs take the same path as before; a host with persistent maps, the same path as before too.

Measured on sandbox sbx_y2bxfj2u (Apple M1 virtual machine, macOS 26.7.2, `MACRON_OPENGL=apple-software`, through `macron launch`), three alternating rounds; `docs/wd3d-upload64.md` has every number:
- **Direct3D 9 probe, 64-bit:** with 3,900 one-pixel sprites (`up --sprite-size 1 --window 1024x768`), 1.06 fps before and 34.2 fps after.
- **Same probe, 32-bit:** 28.9 and 29.8 fps, unchanged within noise.
- **Per draw:** stock 64-bit wined3d's upload and draw (`glprobe --cost`, `up-draw-map-11-calls`) 80 µs; with 0008 (`up-draw-subdata-3-calls`) 0.55 µs.
- **Without Wine:** the same OpenGL work in a native x86_64 program (`loadprobe`) ran at 70 fps.

The `d3d9-fps-probe`, `d3d11-probe` and `openttd` scenario checks passed with it, and so did a copy of `d3d9-fps-probe` that compares the 64-bit probe's picture (SSIM 0.995). The 64-bit probe in the scenario's `mix` mode went from 5.4 to 14.4 fps, about level with the 32-bit one (14.5), so the scenario's frame-rate check (32-bit at least 0.80 of 64-bit) now has less room than before.

`wined3d.dll` is a Windows module; `../check-pe-exports.sh` checks that it exports what Gcenx's does.

## 0009: write `NOOVERWRITE` and `DISCARD` buffer uploads without waiting for the GPU

Measured on Thalles's MacBook Air M3 (`docs/proof/gate1-m3/`): Ultimate Custom Night ran at 5 fps in its menu and in a night, with 77% of its thread inside `glBufferSubData`. Apple's OpenGL on Apple silicon is a layer over Metal; `glBufferSubData` into a buffer that issued draws still use submits the context's command buffer (`flushContext`) and then waits for the GPU for a new one. With 0003 (and 0008 for 64-bit), every streaming write (each `DrawPrimitiveUP`, each `NOOVERWRITE` or `DISCARD` lock of a dynamic buffer) reached the host OpenGL exactly that way, about 3,900 a frame in a night. A native program on the same GPU drawing that load ran at 3 fps with `glBufferSubData`, 1.6 with a synchronised map, and 120 with `glMapBufferRange(GL_MAP_UNSYNCHRONIZED_BIT | GL_MAP_INVALIDATE_RANGE_BIT)`. The 0.6 to 0.9 µs a Wine system call costs there was never the problem; 0007's batching only helps once nothing waits.

What changes:
- **wined3d keeps the map's flag.** The heap branch of `wined3d_cs_map_upload_bo()` records `UPLOAD_BO_NOOVERWRITE` or `UPLOAD_BO_DISCARD`, and `wined3d_buffer_update_sub_resource()` passes it on as `WINED3D_MAP_NOOVERWRITE` or `WINED3D_MAP_DISCARD` (`DISCARD` then means the whole buffer is discarded, even though only the mapped range is written).
- **`wined3d_context_gl_copy_bo_address()`** writes such an upload with `glBufferWriteUnsynchronizedWINE`. For `DISCARD` it first orphans the buffer object with `glBufferData(NULL)`, so draws already issued keep the old storage; without `ARB_buffer_storage` every buffer object is its own GL buffer, and one that is not (a suballocation) keeps `glBufferSubData`. Uploads without either flag (`UpdateSubResource`, initial data) keep `glBufferSubData`.
- **opengl32: `glBufferWriteUnsynchronizedWINE(target, offset, size, data)`**, handed out by `wglGetProcAddress` and loaded by wined3d's `load_gl_funcs()`. It is a Unix call of its own, after `flush_batch`: on the Unix side it maps the range unsynchronised, copies the data and unmaps (`glBufferSubData` if the map fails), so a 32-bit program never sees a 64-bit pointer. It returns nothing, so 0007 queues it with its data like `glBufferSubData` (`gl-batch-table.py` knows it; `make_opengl` generates its table entries). `WINE_OPENGL_UNSYNCHRONIZED=0` hides it, and wined3d falls back to `glBufferSubData`.
- **Software vertex processing.** Ultimate Custom Night (Clickteam Fusion) creates its device with `D3DCREATE_SOFTWARE_VERTEXPROCESSING` (flags `0x24` in a `+d3d9` trace), uses a dynamic `D3DPOOL_SYSTEMMEM` vertex buffer that it locks with `NOOVERWRITE` before each draw (and `DISCARD` when it wraps), and a system memory index buffer. With software vertex processing wined3d pinned a system memory copy of every buffer, its own streaming buffers and d3d9's GPU copies of system memory buffers included, so none of them took the heap path and each draw still made two `glBufferSubData` (vertices and indices; a `+opengl` trace of the game with the first half of 0009 showed 2 per draw and no unsynchronised write). Pinning is there so that an application's maps keep their pointer and their data across `DISCARD`; it is no longer applied to buffers without map access or to wined3d's streaming buffers.
- **System memory vertex buffers.** d3d9 copies the drawn range of such a buffer into its GPU copy before each draw (`copy_sub_resource_region`, same offsets). wined3d now notes on a buffer the GPU cannot read how the application wrote it since its last copy to that destination: only `NOOVERWRITE` locks, a `DISCARD` lock, or anything else (`wined3d_client_resource.copy_writes`, noted before `sanitise_map_flags()` drops those flags for such buffers). The next copy to the same destination at the same offsets is then an unsynchronised write, an orphan and write, or stays `glBufferSubData`.
- **A use after free.** A d3d9 program may draw from a buffer it still has mapped (`wined3d_device_context_flush_mapped_buffer()`). On the heap path that early upload carried `UPLOAD_BO_FREE_ON_UNMAP`, so the heap memory was freed while the program still wrote to it, and the unmap uploaded it again. The early upload no longer frees it, and the upload at unmap waits (`glBufferSubData`), because draws issued in between read that range.

The sandboxes have no hardware OpenGL, and Apple's software renderer there does not wait in `glBufferSubData`, so their frame rate cannot show the change. What they can show is what reaches the host OpenGL: the `d3d9-fps-probe` scenario's `uploads` check (`docs/checks.md`) traces the 32-bit and 64-bit probes and requires no `glBufferSubData` and no synchronised map while they measure, and a control run with `WINE_OPENGL_UNSYNCHRONIZED=0` must show them. The pictures still have to match the Windows reference: a write that landed in the wrong place, or one the GPU read too early, shows up there. The frame rate on the M3 is Thalles's to measure.

## 0010: map CPU-only buffers on the client thread, and write streaming uploads through `GL_COPY_WRITE_BUFFER`

From the M3's sample with 0009 (`docs/proof/gate1-m3/macron9-raw/`, resolved with `tools/clickteam-games/sample-functions.py`), Ultimate Custom Night's menu at 31 fps:
- **Maps through the command stream.** The game locks its `D3DPOOL_SYSTEMMEM` vertex buffer before each draw, and d3d9 maps the system memory index buffer before each draw to stream the indices: three maps per draw, each a round trip through wined3d's command stream. With the command stream on its own thread (`csmt=0x1`) each is also a wait for everything queued, which is why that ran no faster on the M3 (the game's thread spent 23% yielding to and waking the command stream). A buffer only the CPU uses lives in its system memory and nowhere else, so such maps are now done on the client thread (`wined3d_cs_map_cpu_buffer()`): reads and `NOOVERWRITE` writes without waiting, other writes, and any map after the command stream was given writes into the buffer, after waiting until it is done with it.
- **Index uploads changed the vertex array object.** Each index upload bound the index buffer to `GL_ELEMENT_ARRAY_BUFFER`, which is vertex array state: Apple's OpenGL rebuilt its vertex array state at the next draw (`buildVertexArrayState`, `buildVertexArrayDescriptor` in the sample), and wined3d bound the draw's index buffer again. Unsynchronized uploads are written through `GL_COPY_WRITE_BUFFER` instead.

On the sandbox (sbx_m3nfabjb, `MACRON_OPENGL=apple-software`), the probe in `--sysmem` mode at 3,900 one-pixel draws: with the command stream on the game's thread no change beyond the sandbox's noise (macron.9 29.0, 20.8, 20.4 fps; 0010 26.4, 24.8, 18.3); with `csmt=0x1` 1.4, 1.2, 1.5 fps before and 5.4, 6.0, 6.8 after (the remaining cost there is the sandbox's 14 µs per wake). `d3d9-fps-probe` and `d3d9-sysmem-probe` pass with it, and so does `d3d9-sysmem-probe` with `csmt=0x1`. Published as the test overlay `runtime-wine-11.21-macron.10` (branch `wine-0010`); whether it helps is the M3's to say.

## 0011: run the command stream on its own thread in WoW64 again

Reverts what 0004 changed. On Thalles's M3 with 0010 (`docs/proof/gate1-m3/macron10/`), Ultimate Custom Night's menu ran at a median 31.0 fps with the command stream on the game's thread and **60.0** with `WINE_D3D_CONFIG=csmt=0x1` (p10 50.1; the two threads together near two cores). With macron.9 the command stream's own thread gave 30.0, because the game's thread waited for it at each of three maps per draw; 0010 removed those waits. 0004's other reason, crashes and stalls of 32-bit programs with the command stream on its own thread under Rosetta, was the mode switch that 0006 fixed: with 0010, 20 of 20 starts of the probe in `--sysmem` mode with `csmt=0x1` measured on a sandbox, and `d3d9-sysmem-probe` passed with it. On the sandboxes (a virtual machine, where every wake of the other thread is a 14 µs system call) the command stream's own thread is slower than on the game's thread; the sandboxes are not what Macron is for. `WINE_D3D_CONFIG=csmt=0` puts it back on the application's thread.

## 0013: don't show wineboot's wait window

When wineboot installs `wine.inf` into a prefix (when the prefix is made, when `share/wine/wine.inf` is newer than the prefix's `.update-timestamp`, and on every `wineboot -u`), it shows "The Wine configuration in … is being updated, please wait…" until the install is done. On a Mac that window turns wineboot into a Dock app with Wine's name and icon, in front of the game that is starting; Thalles watched it for 36 seconds on his first start of Overwatch after installing v0.5.0-rc3. winemac makes a process a regular Dock app only when one of its windows is shown (`transformProcessToForeground`), so without the window wineboot stays out of the Dock, the menu bar and the Force Quit list. The patch skips the window; the install runs exactly as before, in the same message loop. `MACRON_WINEBOOT_WAIT_WINDOW=1` brings the window back, which is how the sandbox check shows that it would see the window (`docs/startup/README.md`).

The number 0012 is taken by a test patch for Ultimate Custom Night's menu (branch `wine-0012`, test overlay `runtime-wine-11.21-macron.12`) that is not on `main` until the M3 says whether it helps. The patches touch different files, so 0013 applies with or without it.

## 0014: a thread registered for audio gets time-critical priority

A game registers the thread that fills its sound buffer with Windows' Multimedia Class Scheduler: `AvSetMmThreadCharacteristicsW(L"Pro Audio", ...)` or `L"Audio"`. Windows then runs that thread ahead of every ordinary one, which is what lets a game keep a buffer of two or three periods without gaps. Wine's `avrt.dll` is a stub that returns a handle and changes nothing, so the thread competes with the game's own worker threads, and when those keep every core busy it fills the buffer late and the sound crackles.

Deadlock on Thalles's M3 (2026-11-18, on its first dialog, built-in speakers at 44.1 kHz): the game opens a shared WASAPI stream with a 512-frame period and a 1536-frame buffer (34.83 ms), and the thread that fills it calls `AvSetMmThreadCharacteristicsW(L"Pro Audio")`. Over 60 s its refills came a median 11 ms apart, p95 17 ms, p99 29 ms, the longest 161 ms; 41 came later than the whole buffer. Thalles heard the crackle.

The patch gives a thread of the tasks `Audio` and `Pro Audio` `THREAD_PRIORITY_TIME_CRITICAL`, and its earlier priority back on `AvRevertMmThreadCharacteristics`. Wine 11.21's server carries that to macOS as a fixed (non-timeshare) priority of 63, above every timeshare thread; Wine's own audio timer thread, which wakes the game's thread once a period, already runs there (`create_unix_thread` in `dlls/mmdevapi/unixlib.h`). Threads of the other tasks (`Games`, `Playback`, `Capture`, `Window Manager`, ...) are registered and left alone: a game's render thread registered as `Games` computes for a whole frame, and at a fixed priority it would hold the sound threads back instead. Handles are slots of a fixed table; a thread that ends without reverting frees its slots. The patch also fixes `AvSetMmMaxThreadCharacteristics`, which refused every call with two task names (`if (!task1 || task2)`).

What it does not do: Windows puts these threads in its realtime range, above a process's own time-critical threads, and Wine's server refuses that range to a process of normal priority class. So a game with busy time-critical threads of its own can still delay its sound thread. `tools/perf/sndload.c` measures late wake-ups of such a thread under load, with and without the registration.

**On the M3 it changed nothing by itself** (2026-11-18): Deadlock's refills later than the buffer were 16 and 15 in a minute with the patch and 9 and 18 without, and `sndload` under load had 83 and 87 late wake-ups registered against 104 and 82 not. The thread was woken late, and no priority of its own changes that; 0015 is about the waking.

On a sandbox (sbx_ttkjpgu5, three cores, overlay `macron.14`) `tools/perf/sndload.c` opens the same stream (1536 frames, 32 ms at 48 kHz) and keeps the cores busy with three ordinary threads and six at `THREAD_PRIORITY_HIGHEST`, mixing for 4 ms a wake-up. Three alternating 20 s runs each way: unregistered, the thread woke 1268 to 1370 times instead of about 1875, intervals p95 27 to 36 ms, 36 to 86 of them longer than the buffer; registered as `Pro Audio` (priority 0 before, 15 after, 0 after reverting), 1851 to 1869 times, p95 11.5 to 13 ms, 14 to 19 longer than the buffer. Those 14 to 19 are the sandbox's: with a lighter load (three threads at `HIGHEST`) both ways had 15 to 28 of them in that session and 1 to 2 in an earlier one, so it is the virtual machine stalling, not the thread's priority. What the patch does for Deadlock is the M3's to say.

`avrt.dll` is a Windows module, built for i386 and x86_64; `../check-pe-exports.sh` checks that it exports what Gcenx's does.

## 0015: the server runs at the top of the application priority band

Every wait a Windows thread wakes from, every event, semaphore and mutex it signals and every input message goes through `wineserver`'s one thread (Gcenx's Wine has neither esync nor msync). macOS runs that thread like any other, at timeshare priority 31, so on a busy Mac it queues behind the game's own threads, and whatever it was about to wake waits with it, whatever that thread's own priority. Patch 0014 could not help for that reason: it raised the thread that is woken, not the one that wakes it.

Measured on Thalles's M3 (2026-11-18) with `tools/perf/sndload.c` in its `--stream 0` mode: a time-critical thread signals every 10 ms, another time-critical thread waits for the signal, and eight ordinary and four high-priority threads keep the eight cores busy, 15 s a run.

| | wake-ups later than 35 ms | signal to wake-up, p95 | p99 | longest |
|---|---|---|---|---|
| event, server as upstream (5 runs) | 27 to 48 | 20 to 25 ms | 44 to 57 ms | 118 to 192 ms |
| event, server raised (2 runs) | 0 | 0.02 to 0.03 ms | 0.03 to 0.04 ms | 0.07 ms |
| `WakeByAddressSingle`, no server (4 runs) | 0 | 0.01 ms | 0.01 to 0.02 ms | 0.03 ms |

With only three busy threads the upstream server still let 3 and 7 wake-ups in 15 s arrive later than 35 ms (up to 169 ms); raised, none (longest interval 11.08 ms against a 10 ms period).

The patch gives the server's thread what the server itself gives a time-critical thread of a normal process: not timeshare, importance 32, which is priority 63 (`ps -M` shows `63R`). `MACRON_WINESERVER_PRIORITY=normal` in the environment leaves it as upstream; that is how the two rows above were taken with one build. Where a process runs under a QoS clamp, as everything started over ssh on the sandboxes does (priority 20), the thread becomes fixed-priority at the clamp.

**What it did not change: Deadlock's sound on its first screen.** Four starts on the M3, alternating: refills later than the buffer in a minute were 11 and 17 raised, 8 and 22 as upstream, and the long ones were the sound thread waiting for its wake-up either way (61 to 137 ms). `sndload` with a real stream, where Wine's own timer thread does the signalling, went from 81 and 90 late wake-ups to 53 and 61: better, not gone. So something in Wine's sound timer path is late by itself; that is open. Frame rate in those starts: a median 62.4 and 60.1 frames a second raised, 65.5 and 64.4 as upstream, on a Mac that goes from thermal level 0 to 2 in each start, so no difference can be read from four of them.

`wineserver` is a Unix program. `../check-unix-lib.sh` checks that an unpatched build links and searches what Gcenx's does (Gcenx's is linked against the `libinotify` its tree ships, for folder change notifications, so the build is too) and that the patch adds no import but `mach_thread_self`.

## Building

`.github/workflows/wine-build.yml` runs `../build-overlay.sh` on a macos-26 runner: it builds `winemac.so` (x86_64), `wined3d.dll` (i386 and x86_64), `wow64cpu.dll` (x86_64), `opengl32` (`opengl32.so`, x86_64; `opengl32.dll`, i386 and x86_64) `wineboot.exe` (x86_64), `avrt.dll` (i386 and x86_64) and `wineserver` (x86_64) from the source above, unpatched and patched, compares them with Gcenx's, and packs the patched ones as an overlay (`bin/wineserver`, `lib/wine/x86_64-unix/{winemac,opengl32}.so`, `lib/wine/{i386,x86_64}-windows/{wined3d,opengl32,avrt}.dll`, `lib/wine/x86_64-windows/{wow64cpu.dll,wineboot.exe}`, plus this folder and Wine's licence under `share/doc/macron-wine/`). Run from `main`, it publishes the overlay as a release; `internal/wineruntime` pins it by URL and SHA-256, copies all of it into the DXMT Wine tree, and copies the Windows modules (`wined3d.dll`, `wow64cpu.dll`, `opengl32.dll`, `wineboot.exe`, `avrt.dll`), `opengl32.so` and `wineserver` into the plain tree too, which keeps Gcenx's `winemac.so`.
