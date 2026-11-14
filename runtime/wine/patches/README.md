# Macron's patches to Wine

These patches apply to Wine 11.21 (tag `wine-11.21`, https://gitlab.winehq.org/wine/wine/-/tree/wine-11.21). Like Wine, they are licensed under the GNU Lesser General Public License 2.1 or later (see `COPYING.LIB` in Wine's source). Every build Macron ships is published with them on the `runtime-wine-*` releases of this repository.

## 0001: export `macdrv_functions` for DXMT

DXMT (Direct3D 10/11 over Metal) draws each frame into a Metal view inside the window's client area. To make that view, its `winemetal.so` looks up the Mac driver with `dlsym(RTLD_DEFAULT, "macdrv_functions")`, a table of function pointers in the layout CrossOver uses, and reads `hwnd`, `cocoa_window`, `cocoa_view`, `client_cocoa_view` from the start of the struct that `get_win_data` returns. Upstream Wine builds `winemac.so` with hidden visibility, so DXMT finds nothing: its device works but its swap chain logs `Failed to create metal view, it seems like your Wine has no exported symbols needed by DXMT` and nothing reaches the screen.

Exporting Wine's own functions would not be enough, because Wine 11's `struct macdrv_win_data` starts with `hwnd`, `cocoa_window`, `client_view`, `rects`: DXMT would take part of `rects` for the client view. Wine 11 also no longer keeps a client-area view per window; it makes one only for an OpenGL or Vulkan surface.

The patch:

- adds `dlls/winemac.drv/dxmt.c`, which exports `macdrv_functions` with DXMT's layout. Its `get_win_data` locks the window data the way Wine's does and returns a `struct macdrv_dxmt_win_data` (DXMT's layout) kept inside Wine's `struct macdrv_win_data`. Its `client_cocoa_view` is a view over the client area, made on first use, moved and resized with the window, and disposed with the Cocoa window. Child windows, which have no Cocoa window of their own, get an empty struct, so DXMT gets no view instead of crashing. Two entries DXMT does not use are NULL (`macdrv_init_display_devices`, `on_main_thread`).
- makes `macdrv_view_release_metal_view` clear the view's cached Metal view, so a second swap chain on the same window gets a new Metal view instead of the one DXMT released.

Only `winemac.so` changes. The Windows side of the driver, its unix call table and every other module are untouched, which is what lets Macron ship the patched `winemac.so` as a drop-in for Gcenx's wine-devel 11.21 build (upstream 11.21 plus one change to `dlls/win32u/vulkan.c`). `../check-abi.sh` checks that on every build.

## Building

`.github/workflows/wine-build.yml` runs `../build-winemac.sh` on a macos-26 runner: it builds `winemac.so` for x86_64 from the source above, unpatched and patched, compares both with Gcenx's, and packs the patched one as an overlay (`lib/wine/x86_64-unix/winemac.so`, plus this folder and Wine's licence under `share/doc/macron-wine/`). Run from `main`, it publishes the overlay as a release; `internal/wineruntime` pins it by URL and SHA-256 and copies it into the DXMT Wine tree only.
