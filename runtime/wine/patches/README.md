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

When the process is WoW64 and buffers cannot be mapped persistently, the patch maps buffers that the application cannot read, and whose system memory is not pinned, through heap memory the size of the mapped range, the way `UpdateSubresource` already works: the command stream thread uploads it on unmap with `glBufferSubData`, which takes the 32-bit pointer as it is, and the application's thread does not wait for the command stream. An upload of a whole buffer (`DISCARD`) also uses `glBufferSubData` instead of mapping the buffer object. A 64-bit program, or a host with persistent mapping, takes the same path as before.

A first version pinned dynamic buffers in system memory instead. It made every map wait for the command stream thread and hung the probe on the runner, so it was dropped.

## 0004: run the command stream on the application's thread in a WoW64 process

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

## Building

`.github/workflows/wine-build.yml` runs `../build-overlay.sh` on a macos-26 runner: it builds `winemac.so` (x86_64), `wined3d.dll` (i386 and x86_64), `wow64cpu.dll` (x86_64) and `opengl32` (`opengl32.so`, x86_64; `opengl32.dll`, i386 and x86_64) from the source above, unpatched and patched, compares them with Gcenx's, and packs the patched ones as an overlay (`lib/wine/x86_64-unix/{winemac,opengl32}.so`, `lib/wine/{i386,x86_64}-windows/{wined3d,opengl32}.dll`, `lib/wine/x86_64-windows/wow64cpu.dll`, plus this folder and Wine's licence under `share/doc/macron-wine/`). Run from `main`, it publishes the overlay as a release; `internal/wineruntime` pins it by URL and SHA-256, copies all of it into the DXMT Wine tree, and copies the Windows modules (`wined3d.dll`, `wow64cpu.dll`, `opengl32.dll`) and `opengl32.so` into the plain tree too, which keeps Gcenx's `winemac.so`.
