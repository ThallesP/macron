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

`wined3d.dll` is a Windows module, built for i386 and x86_64. `../check-pe-exports.sh` checks on every build that an unpatched build and the patched one export exactly what Gcenx's does.

## Building

`.github/workflows/wine-build.yml` runs `../build-overlay.sh` on a macos-26 runner: it builds `winemac.so` (x86_64) and `wined3d.dll` (i386 and x86_64) from the source above, unpatched and patched, compares them with Gcenx's, and packs the patched ones as an overlay (`lib/wine/x86_64-unix/winemac.so`, `lib/wine/{i386,x86_64}-windows/wined3d.dll`, plus this folder and Wine's licence under `share/doc/macron-wine/`). Run from `main`, it publishes the overlay as a release; `internal/wineruntime` pins it by URL and SHA-256, copies all of it into the DXMT Wine tree, and copies `wined3d.dll` into the plain tree too, which keeps Gcenx's `winemac.so`.
