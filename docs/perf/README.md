# What would make games faster on a real Apple-silicon Mac

The reference is Overwatch (Direct3D 11, 64-bit) through DXMT on Thalles's MacBook Air (M3, 10 GPU cores, 16 GB, 2880x1864 panel, no fan), where Macron 0.4.0 gave 57.5 fps in the practice range and "60 to 75 fps" with the frame rate unlocked (docs/proof/overwatch-m3/). There the Metal HUD put the GPU's time a frame at a median 15.7 ms (p25 14.7, p75 16.8): **the GPU is the limit at about 64 fps**, so work taken off the CPU alone does not raise Overwatch's frame rate. What raises it is less GPU work a frame, or the GPU getting more of the machine.

Every line below says where its evidence comes from: **source** (read in DXMT 0.80 or Wine 11.21), **sandbox** (a GitHub macOS runner: virtual Apple GPU, which shows that something works and what it does, never how fast), **M3** (Thalles's Mac), or **pending** (asked for in [real-mac-requests.md](real-mac-requests.md), not known yet). Nothing marked pending is a finding.

State: 2026-11-16, first batch of real-Mac requests written; no M3 number from them yet.

## Candidates, ranked

Ranked by what they could take off Overwatch's 15.7 ms of GPU time a frame, times how sure the evidence is today.

### 1. DLSS through MetalFX: DXMT's NVIDIA extension (`DXMT_ENABLE_NVEXT=1`)

DXMT answers a game's DLSS calls with Apple's MetalFX temporal upscaler, so a game that offers DLSS renders at a lower resolution and MetalFX rebuilds the full-size picture.

- **source:** DXMT 0.80 ships `nvapi64.dll` and `nvngx.dll`; with `DXMT_ENABLE_NVEXT=1` the adapter reports NVIDIA's vendor id (0x10de, `dxgi_adapter.cpp`), NVAPI reports an RTX 4090 (`nvapi.cpp`, `NvAPI_GPU_GetArchInfo`), NGX reports super resolution available unconditionally, and `NVSDK_NGX_D3D11_CreateFeature` creates a MetalFX temporal scaler (`nvngx.cpp`). Without the variable nothing changes. DXMT's wiki says not to turn it on for every game, because a game that believes it is on NVIDIA may take paths that are only fast there.
- **source:** Macron already installs both DLLs: a DXMT prefix made by Macron has `nvngx.dll` and `nvapi64.dll` in `system32` (checked on the sandbox). Macron never sets the variable.
- **sandbox** (`tools/perf/nvprobe.cpp`, through `macron waitforexitandrun` and the Steam bridge's `steam.exe`, so the way Steam starts a game): without the variable the adapter is `0x106b` and `nvngx.dll` does not load; with `DXMT_ENABLE_NVEXT=1` the adapter is `0x10de`, `NvAPI_Initialize` succeeds, NGX init succeeds and reports `SuperSampling.Available=1`, and creating the DLSS feature fails with `0xbad0001` (feature not supported): the virtual GPU has no MetalFX temporal scaler.
- **Overwatch:** offers DLSS, FSR 2.2 and FSR 1 ("High-Quality Upsampling"); DLSS or FSR 2.2 cap the render scale at 67% ([PCGamingWiki](https://www.pcgamingwiki.com/wiki/Overwatch_2), [Blizzard forum](https://us.forums.blizzard.com/en/overwatch/t/custom-render-scale-option-maxes-at-67/874131)). At 67% the game shades 45% of the pixels.
- **pending:** whether the M3 creates the feature (request 1); whether Overwatch loads `nvngx.dll` and offers DLSS under DXMT (request 6); what share of Overwatch's GPU time scales with pixels (request 3's sweep is the stand-in); how MetalFX temporal looks and costs in the game (Thalles, by hand: it needs a settings change in his account).
- **If it holds:** a per-game profile switch (`env` in `fixes/games/<id>.json`) for games known to work, not a default.

### 2. Rendering fewer pixels: what DXMT really renders at

DXMT renders at exactly the size the game asks for, and on a Retina Mac that size is in points, not pixels.

- **source:** Wine's Mac driver has Retina mode off unless the `RetinaMode` registry value is set (`retina_enabled = false`, `macdrv_main.c`), so Windows sees display modes in points and window layers have `contentsScale` 1.0 (`cocoa_window.m`). DXMT's back buffer is the swap chain's size and its layer's drawable is that size times the MetalFX factor, 1 without MetalFX (`d3d11_swapchain.cpp`, `ResizeBuffers` and `ApplyLayerProps`).
- So Overwatch's 1920x1243 display mode is 1920x1243 pixels (2.4 million), and macOS's compositor scales it up to the 2880x1864 panel (5.4 million). Nothing is spent rendering at Retina resolution; there is no Retina waste to remove. The lever is the other way: fewer pixels and a good upscale (candidates 1 and 4, or the game's own render scale and FSR 1).
- **pending:** how GPU time a frame falls with resolution on the M3 (request 3). If it falls in proportion to pixels, upscaling pays; if a large part stays fixed, that part is per-frame work (DXMT's or the game's) and the next step is a Metal System Trace.

### 3. macOS Game Mode for Wine's processes

macOS gives Game Mode (CPU and GPU priority for the game, background work held back) only to processes whose app bundle says they are a game, and a Wine process is no app bundle.

- **sandbox:** with the stock runtime, RunningBoard sees every Wine process as `anon<wine>` and `gamepolicyd` labels none of them, focused or not. Writing `LSApplicationCategoryType = public.app-category.games` into the Info.plist embedded in Wine's loader (re-signed) changes nothing: still `anon<wine>`.
- **source:** Wine starts every Windows process, the game included, by running `lib/wine/x86_64-unix/wine` (`wineloader` in `dlls/ntdll/unix/loader.c` is the realpath of `ntdll.so`'s folder plus `wine`), and that loader opens the `ntdll.so` beside the realpath of itself (`loader/main.c`).
- **sandbox:** so `tools/perf/gamemode-bundle.sh` moves that loader into `Macron Game.app/Contents/MacOS/wine` (with `ntdll.so` linked beside it and a games-category Info.plist) and leaves a symlink at the old path. Then `gamepolicyd` logs `Found game GameProcess(Optional("wine"), pid=…, labelReason=infoPlist)` for every Wine process, the Game Mode rocket appears in the menu bar, Game Mode goes to `paused` while the focused game is windowed, and the Direct3D 11 probe draws as before. The first start of the bundle asked once "Allow "Macron Game" to find devices on local networks?" and Wine waited until it was answered.
- **pending:** whether Game Mode turns `on` for a full-screen game on the M3, and whether it changes frame times at all (request 5). Nobody has published frame times for Wine under Game Mode; the projects that do the same ([highball #338](https://github.com/gauthierpiarrette/highball/issues/338), [Yaagl #773](https://github.com/yaagl/yet-another-anime-game-launcher/issues/773)) checked only that it turns on, and Yaagl reports audio problems for some users.

### 4. MetalFX spatial upscaling in the swap chain (`DXMT_METALFX_SPATIAL_SWAPCHAIN=1`)

DXMT can upscale the game's finished frame with MetalFX before presenting it, so the picture is sharp on a Retina screen while the game renders the smaller size.

- **source:** with the variable, the swap chain keeps the game's back buffer at the size the game asked for and presents a MetalFX spatial upscale of it, `d3d11.metalSpatialUpscaleFactor` times larger (default 2, set through `DXMT_CONFIG="d3d11.metalSpatialUpscaleFactor=1.5"`).
- It adds GPU work (the upscale and a larger drawable); it only saves time together with a lower game resolution. For Overwatch, the game's own render scale with FSR 1 is the same idea, inside the game.
- **pending:** what the upscale costs on the M3 (request 4).

### 5. GPU work DXMT does that a native renderer would not

- **source:** presenting is one extra full-screen pass: DXMT draws the back buffer into the drawable with a full-screen triangle (load action DontCare, store Store; `dxmt_presenter.cpp`). At Overwatch's 2.4 million pixels that is reading and writing about 20 MB a frame. DXMT's shader compiler emits fast math unless HLSL marks an instruction `precise` (`airconv`, `UseFastMath`). It lets the GPU fall up to 3 frames behind (`max_latency_ = 3`), which is the usual depth. None of this stands out.
- **source and sandbox:** Overwatch's `CreateGeometryShaderWithStreamOutput: not supported, expect problem` is DXMT emulating stream output; Unigine Superposition hits the same line on the sandbox (`warn: Emulate stream output`).
- **pending:** the GPU busy share (GPU time over frame interval) in the benchmark (request 2) and in Overwatch's menu (request 6). A frame interval much longer than the GPU time means the GPU waits on something.

### 6. msync (Mach-semaphore synchronisation) in the Wine overlay

msync replaces wineserver round trips for Windows waits and events with Mach semaphores in the game's own process.

- **M3:** a wineserver round trip costs 8.1 µs and this Wine has no msync or esync (`macron diagnose`).
- For Overwatch it cannot raise the frame rate while the GPU is the limit. It may help CPU-bound games. It exists as [marzent/wine-msync](https://github.com/marzent/wine-msync) (LGPL-3.0, written against CrossOver's Wine) and as a per-bottle switch in CrossOver 25, where CodeWeavers say some applications run faster, some show no difference and some break ([CodeWeavers](https://support.codeweavers.com/miscellanous/advanced-settings-in-crossover-mac)).
- **pending:** how busy the wineserver is while Overwatch runs (request 6's `top`).

### 7. AVX under Rosetta (`ROSETTA_ADVERTISE_AVX=1`)

Rosetta can run AVX and AVX2 code but hides them from CPUID unless the variable is set.

- **sandbox** (`tools/perf/avxprobe.c`, through `macron waitforexitandrun`): without the variable a Windows program under Macron sees no AVX, AVX2, FMA, F16C or BMI, and `IsProcessorFeaturePresent(PF_AVX_INSTRUCTIONS_AVAILABLE)` is false; with it, all of them are present, XCR0 is 0x7 and `GetEnabledXStateFeatures` is 0x7. The same float kernel (4096 floats in L1, 200,000 passes) took 0.079 to 0.080 s as SSE2 (20.4 to 20.8 GFLOP/s) and 0.125 s as AVX2+FMA (13.1 GFLOP/s): **AVX2 code is 1.56 times slower** than SSE2 code under Rosetta here. The sandbox's CPU cores are real Apple cores, and this kernel makes no system calls, so the ratio should hold on the M3 (request 1 checks).
- So it is a compatibility switch for games that refuse to start without AVX (CrossOver turns it on per game), not a speed-up; turned on for every game it can make games that pick an AVX path slower.

### 8. D3DMetal (Apple's Game Porting Toolkit) installed by the user

- Apple's licence does not let Macron ship D3DMetal. CrossOver bundles it and picks DXMT, D3DMetal or wined3d per game from its own database ([CrossOver 25](https://www.codeweavers.com/crossover/changelog)). Whisky imported a user's Game Porting Toolkit by copying the toolkit's `redist/lib/external` (`D3DMetal.framework`, `libd3dshared.dylib`) into its Wine's `lib/external` ([AppleGamingWiki](https://www.applegamingwiki.com/wiki/Game_Porting_Toolkit)).
- **not established:** whether D3DMetal's Windows DLLs work with the Wine Macron uses (Gcenx's 11.21, not CrossOver's), and how D3DMetal and DXMT compare on Overwatch. A user who has downloaded the toolkit from Apple (free developer account) could point Macron at it; that is a backend Macron does not have yet.

## The benchmark

`tools/perf/bench.sh` runs a free Unigine benchmark through `macron waitforexitandrun` with the default DXMT profile, so the installed Macron 0.4.0 needs nothing new: Superposition 1.1 (64-bit, Direct3D 11, 2017) or, as a fallback, Heaven 4.0 (32-bit, 2013). Each plays its own fixed camera path in a window and quits by itself (Superposition's benchmark mode `-mode 2`, Heaven's Phoronix mode), with the Metal HUD logging every frame's interval and GPU time into macron's log. The script waits for the Mac to be thermally nominal before each run (a MacBook Air has no fan), logs the thermal pressure level each second, and summarises frame rate, GPU busy share and interval and GPU time percentiles.

What the sandbox showed of it:

- Both install silently under Macron's Wine and start through DXMT. Both need their own Microsoft shader compiler (`d3dcompiler_47`, or `d3dcompiler_42` and `d3dx11_42`) set to native: Wine's builtin compiler (vkd3d-shader) fails on their shaders (`E5017: Aborting due to not yet implemented feature: Double-to-integer conversion`, `Function "asdouble" is not defined`), after which Superposition stops with `can't load "superposition/system_script.cpp"`. `bench.sh setup` sets the overrides in the benchmark's prefix.
- Neither can finish on the sandbox's virtual GPU: Superposition hangs it at frame 9 (`kIOGPUCommandBufferCallbackErrorHang`, then every submission is ignored), Heaven around frame 100 after `Failed to create tessellation raster PSO`. DXMT warns `Experimental non-Apple GPU support` there. Whether they run on the M3 is request 2's first answer.
- With `MTL_HUD_ENABLED=1` DXMT stops after choosing its feature level on the sandbox (the Direct3D 11 probe presents normally without it), so the HUD's log cannot be tried there. Its format comes from Apple's tech talk and [MetalHUDLogger](https://github.com/InvurseStudio/MetalHUDLogger): `metal-HUD: <frame>,<memory>,<memory>[,<misses>],<interval>,<GPU>,…`.

## Other findings on the way

- A program started through Macron's `steam.exe` (the Steam bridge, so every Steam game) has no stdout in macron's log; the probes write their results to a file.
