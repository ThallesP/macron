# Macron's DXMT

Macron's runtime takes DXMT 0.80 as released and replaces the modules these patches change with its own build of them (`build.sh`, the `DXMT build` workflow, published as `runtime-dxmt-0.80-macron.N`): `d3d11.dll`, `winemetal.dll` and `dxgi.dll` for x86_64 and i386, `nvngx.dll` for x86_64, and `winemetal.so`. They only work together: `d3d11.dll` calls `winemetal.dll`, which calls `winemetal.so` by number, and `dxgi.dll` trusts the certificate of the `nvngx.dll` it was built with. DXMT's `d3d10core.dll` and `nvapi64.dll` stay as released.

## Patches

Applied in order to DXMT at the commit in `release.env`.

| Patch | What it is for |
|---|---|
| `0001-d3d11-parse-shaders-on-first-compile` | DXMT kept every shader a game creates in its parsed form for the life of the device. Overwatch creates 82,706 before its sign-in screen: 7.9 GB. The patch keeps the bytecode and parses again on a shader's first compile (docs/overwatch-memory.md). |
| `0002-dxgi-maximum-frame-latency-follows-dxgi` | From mavrik-engine. `SetMaximumFrameLatency(0)` means DXGI's default of 3 instead of "wait for every frame", values above 16 are refused, and `dxgi.maxFrameLatency` (DXVK's option) caps the limit. |
| `0003-dxmt-frame-latency-counts-to-presentation` | From mavrik-engine, ported to 0.80. With vsync or a frame cap, a frame counts against the frame latency limit until it is on screen, not until the GPU has finished it, so finished frames no longer queue in Core Animation's drawables. Adds the Unix call `MetalDrawable_signalEventWhenPresented`. |
| `0004-dxmt-latency-sleep` | Macron's. A game that DXMT's encode thread holds back (vsync, a frame cap, or more frames than a window gets drawables for) queued up to three frames between its input and the screen. Present now sleeps until the next frame, started then, reaches the encoder as it becomes free (`dxmt.latencySleep`). `dxmt.logLatency` logs what it measures. |
| `0005-dxgi-nvext-ngx-certificate-and-rtx-adapter` | Macron's, after Mullion. With the NVIDIA extension on (Macron's `dlss` switch), a game's NGX loads DXMT's `nvngx.dll`, which it refused as unsigned, and the adapter is the RTX card DXMT's NVAPI already names instead of "Apple M3" with NVIDIA's vendor id. |

mavrik-engine is https://github.com/camsteinberg/mavrik-engine (LGPL 2.1 or later) and Mullion is https://github.com/dttdrv/mullion (LGPL 2.1); each patch names the commit it follows.

## Input latency: what the patches change, and what they do not

A frame passes three stages, each on its own thread: the game (until it calls Present), DXMT's encode thread, and the GPU. DXGI lets a game run three frames ahead unless it asks for fewer. The encode thread takes one frame at a time, and a present holds it until a drawable is free.

- **A game held back by the encode thread** (0004): with vsync or a frame cap a drawable is free once a display interval, and without them a window still gets only about two a refresh. Such a game had three frames queued in front of the encoder. Now, when frames are seen to wait for the encoder, Present sleeps until a frame started then arrives just as the encoder is done with the ones in flight. The encoder always has work, so the frame rate stays.
- **With vsync or a frame cap** (0003): the limit counts frames until they are on screen.
- **A game held back by the GPU is not paced.** It keeps the queue its frame latency limit allows (DXGI's 3 unless it asks; Deadlock asks for 1). Pacing it needs to know when the GPU can take the next frame, and an Apple GPU works on two frames at once: taking it to work on one at a time cost up to half the frame rate on an M3. `dxmt.latencyGpuModel` has the two readings that were tried, for whoever measures further.
- `dxgi.maxFrameLatency=N` (0002) caps the limit for one game. A limit of 1 took the GPU-bound test program from 46 to 51 frames a second to 30 to 34 on the M3, so it is nobody's default.

Measured with `tools/perf/latency.sh` (its numbers are in docs/perf/latency.md).

The options go in `DXMT_CONFIG`, separated by `;`, for example as a game's launch option in Steam: `DXMT_CONFIG="dxmt.latencySleep=False" %command%`. A game's profile and the player can both set it; the player's options win.

| Option | Default | |
|---|---|---|
| `dxmt.latencySleep` | `True` | The sleep of 0004. |
| `dxmt.latencySleepWithVsync` | `True` | Whether it also applies to a present that waits for the display. |
| `dxmt.latencyTolerance` | `1000` | Microseconds a frame starts earlier than computed. More costs latency, less risks the encoder waiting. |
| `dxmt.latencyGpuModel` | `none` | `finish` or `start`: also pace a game the GPU holds back, by one of the two readings 0004 describes. |
| `dxmt.logLatency` | `False` | Every 300 frames, a line `Latency: frame N, F fps, start to GPU done X ms (max Y), ...`. Macron turns it on with `show-fps`, and `macron diagnose` reads it. |
| `dxmt.logLatencyFrames` | `False` | A line a frame with all of its times. |
| `dxgi.maxFrameLatency` | `0` | Caps the frame latency limit (1 to 16); 0 leaves it to the game. |

## DLSS: how a game gets to DXMT's `nvngx.dll`

With `DXMT_ENABLE_NVEXT=1` (Macron's `dlss` switch), DXMT answers as an NVIDIA card and its `nvngx.dll` turns a game's DLSS calls into MetalFX's temporal upscaler. The game does not call that DLL itself: it links NVIDIA's NGX, which looks the DLL up in the registry and loads it only when its signature chains to a root the machine trusts. DXMT's is unsigned, so NGX refused it (Deadlock: `failed to load NGXCore: -2146885623`, with NGX's log turned on) and the game reported DLSS as not available.

`build.sh` makes a key for each build, signs that build's `nvngx.dll` with it, compiles the certificate into `dxgi.dll`, and deletes the key. `dxgi.dll` adds the certificate to the prefix's Root store when the extension is on. So nobody holds a key that a prefix trusts, and a prefix trusts nothing but the one file its runtime came with. The certificate names Macron and is not a certificate authority. Macron's launcher copies the runtime's `nvngx.dll` into the prefix's System32 at every start, because NGX checks the file there and an older prefix has an older runtime's.

`tools/perf/nvprobe.cpp` shows what a game sees: the adapter, where the registry sends NGX, and whether the signature of the file there is trusted.

## Building

`build.sh <work> <out>` on a Mac with Homebrew's `cmake` and `ninja` (it installs `osslsigncode`). The Windows side is built with the llvm-mingw release DXMT's CI uses; `winemetal.so` with Xcode's clang for x86_64 and macOS 15.0, against LLVM 15.0.7, which DXMT's shader compiler links statically. LLVM is built first (`<work>/llvm-darwin`, about three quarters of an hour on a three-core runner, kept in the Actions cache by the workflow); `DXMT_LLVM=<dir>` uses a build that is already there. The script checks that `winemetal.so` is x86_64 only, targets macOS 15.0, exports the call table and needs nothing outside macOS and Wine, that every DLL carries Wine's builtin mark, and that `nvngx.dll`'s signature verifies against the build's certificate.

Bump `RELEASE_TAG` and `ASSET_NAME` in `release.env` for every published build, then pin the new asset in `internal/wineruntime/manifest.go`.
