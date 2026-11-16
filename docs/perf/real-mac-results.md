# Results from Thalles's Mac

MacBook Air M3 (Mac15,13, 8 CPU cores, 10 GPU cores, 16 GB), macOS 26.5.2, on its charger, Macron 0.4.0 as installed (runtime `wine-11.21-macron.6`, DXMT 0.80). Run by the session on that Mac while Thalles slept, 2026-11-16 04:05 to 05:17 local time (07:05 to 08:17 UTC). Files per run are in `docs/perf/m3-batch1/<tag>/`.

## How the runs differ from the requests

- **Unattended rules.** Each run had an empty AeroSpace workspace in front and Wine's audio driver off in the benchmark's Windows environment (`HKCU\Software\Wine\Drivers`, `Audio` empty), so Superposition logged "Can't initialize OpenAL ... \"null\" is used instead". Thalles's own windows were on other workspaces; a browser tab with a video, Spotify, Discord and this session's tools were running the whole time, so the Mac was not idle.
- **The benchmark's window has to be the active app.** Started from a script, Superposition logs "Benchmark running" and "Benchmark stopped" one second apart and reports FPS -1. The runs here keep it active with `ci/steam-play/drive activate wine` every 1.5 s. One run (`mfx-on`) still stopped that way after 24 s and was repeated as `mfx-on-2`.
- **AeroSpace tiles new windows.** Without help "1600x900 windowed" rendered at 960x1172. Each benchmark window was set to floating as it appeared and then has the requested size; every `bench.txt` has the render resolution and all of them match the request.
- A first pass without those two fixes is not in the repository (every Superposition run stopped after a second, and the `pair` windows were tiled).
- **Not run:** request 5 (Game Mode: it asks a macOS permission question that is Thalles's to answer, and changes his installed runtime) and request 6's start 2 (the NVIDIA switch for Overwatch on his account). Request 6's start 1 follows separately.

## Setup

```
e3c0d7e5aa75da3b306091349358e8804de16a647cc1d4a9e6ef34ed7a0c8495  avxprobe.exe
4f23bf5d8a275112ee6addf69f7dfaa768814e41199a74a261373feb2ae372ca  nvprobe.exe
047d28f4b4b380d286d9a6aa58243397c00355c6f5e849f4bdb53937dd69f80d  gpuload.exe
4fff6849a06d122113255e48d06bb1e573dbaae05eea63aaddeacb18db99f2d8  gpuload
          Display Type: Built-in Liquid Retina Display
          Resolution: 2880 x 1864 Retina
wine-11.21-macron.5-dxmt-0.80-dxvk-1.10.3-20230507-bridge-proton-11.0-2e-macron.1
wine-11.21-macron.6-dxmt-0.80-dxvk-1.10.3-20230507-bridge-proton-11.0-2e-macron.1
```

## Request 7

```
xctrace version 16.0 (17F113)
```

## Request 1

`avx-0.txt`:
```
RESULT cpuid sse4.2=1 avx=0 avx2=0 fma=0 f16c=0 bmi1=0 bmi2=0 avx512f=0 osxsave=0 xcr0=0x0
RESULT windows PF_AVX=0 PF_AVX2=0 PF_AVX512F=0 xstate=0x0
RESULT sse2 0.057 s 28.93 GFLOP/s
RESULT avx2 skipped: not advertised
```
`avx-1.txt`:
```
RESULT cpuid sse4.2=1 avx=1 avx2=1 fma=1 f16c=1 bmi1=1 bmi2=1 avx512f=0 osxsave=1 xcr0=0x7
RESULT windows PF_AVX=1 PF_AVX2=1 PF_AVX512F=0 xstate=0x7
RESULT sse2 0.057 s 28.90 GFLOP/s
RESULT avx2 0.084 s 19.47 GFLOP/s, 1.48x the SSE2 time (results equal)
```
`avx-1-1.txt`:
```
RESULT cpuid sse4.2=1 avx=1 avx2=1 fma=1 f16c=1 bmi1=1 bmi2=1 avx512f=0 osxsave=1 xcr0=0x7
RESULT windows PF_AVX=1 PF_AVX2=1 PF_AVX512F=0 xstate=0x7
RESULT sse2 0.057 s 28.81 GFLOP/s
RESULT avx2 0.084 s 19.40 GFLOP/s, 1.48x the SSE2 time (results equal)
```
`avx-1-2.txt`:
```
RESULT cpuid sse4.2=1 avx=1 avx2=1 fma=1 f16c=1 bmi1=1 bmi2=1 avx512f=0 osxsave=1 xcr0=0x7
RESULT windows PF_AVX=1 PF_AVX2=1 PF_AVX512F=0 xstate=0x7
RESULT sse2 0.057 s 28.74 GFLOP/s
RESULT avx2 0.084 s 19.43 GFLOP/s, 1.48x the SSE2 time (results equal)
```
`avx-1-3.txt`:
```
RESULT cpuid sse4.2=1 avx=1 avx2=1 fma=1 f16c=1 bmi1=1 bmi2=1 avx512f=0 osxsave=1 xcr0=0x7
RESULT windows PF_AVX=1 PF_AVX2=1 PF_AVX512F=0 xstate=0x7
RESULT sse2 0.057 s 28.77 GFLOP/s
RESULT avx2 0.084 s 19.43 GFLOP/s, 1.48x the SSE2 time (results equal)
```
`nv-0.txt`:
```
RESULT DXMT_ENABLE_NVEXT=(unset)
RESULT adapter vendor 0x106b device 0x0000 "Apple M3" dedicated 6062 MB
RESULT NvAPI_Initialize: failed
RESULT nvngx.dll: not loaded (error 1114)
```
`nv-1.txt`:
```
RESULT DXMT_ENABLE_NVEXT=1
RESULT adapter vendor 0x10de device 0x0000 "Apple M3" dedicated 6062 MB
RESULT NvAPI_Initialize: OK
RESULT NGX init: OK
RESULT NGX SuperSampling.Available=1
RESULT DLSS feature 1280x720->1920x1080: created (MetalFX temporal scaler) (0x1)
```

## Requests 2 to 4 (Superposition, medium, windowed)

| tag | render size | benchmark's FPS (min to max) | HUD frames a second | GPU busy | GPU time ms p25 / median / p75 / p95 | ran | thermal levels seen |
|---|---|---|---|---|---|---|---|
| base-a | 1600x900 | 50.9 (32.5 to 76.3) | 48.3 | 118% | 20.46 / 23.98 / 28.60 / 31.33 | 199 s | 48 0   47 1  101 2  |
| base-b | 1600x900 | 48.7 (20.0 to 76.1) | 46.0 | 116% | 19.39 / 21.16 / 29.66 / 42.25 | 199 s | 49 0   18 1  129 2  |
| res-1280 | 1280x720 | 76.0 (46.2 to 107.2) | 59.1 | 89% | 13.64 / 14.50 / 16.07 / 20.94 | 559 s | 457 0   27 1   63 2  |
| res-960 | 960x540 | 79.4 (21.7 to 125.3) | 53.8 | 63% | 10.14 / 10.71 / 11.60 / 18.22 | 200 s | 51 0   19 1  126 2  |
| res-640 | 640x360 | 96.3 (13.3 to 133.7) | 55.9 | 53% | 7.96 / 8.37 / 9.02 / 14.55 | 201 s | 57 0   21 1  119 2  |
| mfx-off | 800x450 | 78.7 (14.4 to 131.9) | 51.3 | 61% | 9.11 / 9.73 / 12.20 / 20.59 | 201 s | 48 0   38 1  110 2  |
| mfx-on | 800x450 | -1.$ (1000000000.0 to 0.0) | 18.5 | 34% | 11.57 / 18.56 / 22.69 / 30.79 | 24 s | 24 0  |
| mfx-on-2 | 800x450 | 104.0 (49.8 to 133.7) | 59.8 | 60% | 9.36 / 9.99 / 10.50 / 12.09 | 201 s | 129 0   32 1   38 2  |

`mfx-on` is the run that stopped after a second; `mfx-on-2` is its repeat. `mfx-on-2` and `mfx-off` render 800x450; whether the window showed the picture upscaled was not looked at (no screenshots were taken).

## Request 8 (gpuload, 1600x900)

```
m3-draws-d3d11: frames a second: 60.0; GPU time ms: p25 0.82, median 0.83, p75 0.86, p95 1.03, p99 1.18
m3-draws-metal-arm64: frames a second: 248.4; GPU time ms: p25 0.63, median 0.70, p75 1.33, p95 1.97, p99 1.98
m3-draws-metal-x86_64: frames a second: 247.6; GPU time ms: p25 0.63, median 0.69, p75 0.70, p95 1.51, p99 1.97
m3-fill-d3d11: frames a second: 60.0; GPU time ms: p25 7.17, median 7.23, p75 7.42, p95 7.62, p99 8.13
m3-fill-metal-arm64: frames a second: 143.6; GPU time ms: p25 13.15, median 13.67, p75 14.21, p95 15.38, p99 16.57
m3-fill-metal-x86_64: frames a second: 136.6; GPU time ms: p25 12.22, median 12.90, p75 13.34, p95 13.82, p99 14.12
m3-mix-d3d11: frames a second: 60.0; GPU time ms: p25 7.22, median 7.47, p75 7.61, p95 7.75, p99 7.81
m3-mix-metal-arm64: frames a second: 143.5; GPU time ms: p25 12.19, median 12.60, p75 13.30, p95 14.29, p99 15.81
m3-mix-metal-x86_64: frames a second: 134.6; GPU time ms: p25 11.17, median 11.80, p75 12.19, p95 12.93, p99 13.50
```

Each run's own figures (`bench.txt`):

```
m3-draws-d3d11: config api d3d11 mode draws size 1600x900 passes 0 draws 4000;fps 120.00 frames 2400 interval_ms p50 7.909 p95 15.100 p99 15.494;
m3-draws-metal-arm64: config api metal-arm64 mode draws size 1600x900 passes 0 draws 4000;fps 434.59 frames 8693 interval_ms p50 2.550 p95 4.688 p99 5.522;gpu_ms p50 0.697 p95 1.548 frames 8693;
m3-draws-metal-x86_64: config api metal-x86_64 mode draws size 1600x900 passes 0 draws 4000;fps 403.93 frames 8079 interval_ms p50 2.530 p95 5.350 p99 6.523;gpu_ms p50 0.695 p95 1.527 frames 8079;
m3-fill-d3d11: config api d3d11 mode fill size 1600x900 passes 8 draws 0;fps 120.00 frames 2400 interval_ms p50 7.995 p95 10.451 p99 10.833;
m3-fill-metal-arm64: config api metal-arm64 mode fill size 1600x900 passes 8 draws 0;fps 145.28 frames 2906 interval_ms p50 6.855 p95 8.003 p99 11.651;gpu_ms p50 13.677 p95 15.581 frames 2906;
m3-fill-metal-x86_64: config api metal-x86_64 mode fill size 1600x900 passes 8 draws 0;fps 137.89 frames 2758 interval_ms p50 7.255 p95 8.083 p99 13.061;gpu_ms p50 12.895 p95 13.816 frames 2758;
m3-mix-d3d11: config api d3d11 mode mix size 1600x900 passes 8 draws 4000;fps 120.08 frames 2402 interval_ms p50 7.406 p95 10.336 p99 10.627;
m3-mix-metal-arm64: config api metal-arm64 mode mix size 1600x900 passes 8 draws 4000;fps 144.46 frames 2890 interval_ms p50 6.925 p95 8.205 p99 8.972;gpu_ms p50 12.592 p95 14.309 frames 2889;
m3-mix-metal-x86_64: config api metal-x86_64 mode mix size 1600x900 passes 8 draws 4000;fps 135.77 frames 2716 interval_ms p50 7.354 p95 8.503 p99 11.652;gpu_ms p50 11.798 p95 12.939 frames 2716;
```

The Direct3D 11 runs present 60.0 frames a second by the HUD while the program itself counts 120 (fill): the two counts differ, and the native runs are not held to the display. Read the GPU times with that in mind.

## Request 6, part 3 (Overwatch's settings file)

```
GPUScaler = "10.000000"
FullScreenRefresh = "60"
FullscreenWindow = "0"
FullscreenWindowEnabled = "1"
WindowedRefresh = "60"
```

Nothing else in the file matches the pattern.

## Request 6, start 1 (Overwatch left alone after its start, 2026-11-16 05:17 to 05:21 local)

Started the way Steam starts it with `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1`, the frame counter file absent, Macron 0.4.0, sound off, an empty workspace in front, and left for 185 s with nothing clicked. No screenshot was taken (it would show his account), so "the main menu" is what the game shows by itself after signing in; the HUD's figures are level from 40 s after the start to the end.

```
HUD frames logged: 20030
frames measured: 18027 (the first 2003 left out)
frames a second: 53.5
GPU busy: 40% (sum of GPU time over sum of intervals)
interval ms: p25 16.84, median 18.27, p75 20.49, p95 22.85, p99 24.64
GPU time ms: p25 7.46, median 7.77, p75 8.15, p95 8.63, p99 8.98
```

Every 20 s (memory by the HUD, frames a second, median GPU time): 13.2 GB / 25.1 / 1.0 ms at the start, then 13.7 to 14.3 GB, 54.2 to 54.6 frames a second, 7.7 to 8.3 ms for the rest.

`top`, four samples 5 s apart from 135 s after the start (`ow-top.txt`; command, CPU %, threads):

```
wine 100.2 70/4
wineserver 51.2 2
WindowServer 30.3 21
wine 103.9 70/1
wineserver 54.6 2
WindowServer 30.1 21/1
wine 104.4 70/4
wineserver 54.6 2/1
WindowServer 30.3 22
```

So on this screen the GPU works 40% of the time (7.8 ms of an 18.3 ms frame) and the frame rate is 54, while `wineserver` uses about half a core next to the game's one core. That is the case the request describes as "server round trips are a real cost".

Start 2 (the NVIDIA switch) was not run.

# Batch 2 (2026-11-16 05:45 to 06:19 local, 08:45 to 09:19 UTC)

Same Mac and rules as batch 1 (empty workspace in front, benchmark windows floating and active, Wine's sound off). Files in `docs/perf/m3-batch2/`. Request 15 follows separately.

## Request 9

```
cap-d3d-hud0.txt:RESULT fps 117.73 frames 1766 interval_ms p50 3.793 p95 15.207 p99 15.546
cap-d3d-hud1.txt:RESULT fps 119.99 frames 1800 interval_ms p50 9.770 p95 14.970 p99 15.367
cap-metal-hud0.txt:RESULT fps 478.31 frames 7175 interval_ms p50 1.943 p95 4.186 p99 5.151
cap-metal-hud1.txt:RESULT fps 481.59 frames 7225 interval_ms p50 2.337 p95 4.057 p99 4.773
```

## Request 10 (`pair m3sync 1600x900`, both sides sync interval 1)

```
m3sync-draws-d3d11: frames a second: 59.9; GPU time ms: p25 0.95, median 0.98, p75 1.01, p95 1.05, p99 1.10
m3sync-draws-metal-arm64: frames a second: 60.0; GPU time ms: p25 0.76, median 0.77, p75 0.79, p95 0.82, p99 0.84
m3sync-draws-metal-x86_64: frames a second: 60.0; GPU time ms: p25 0.76, median 0.77, p75 0.79, p95 0.82, p99 0.85
m3sync-fill-d3d11: frames a second: 59.9; GPU time ms: p25 14.15, median 14.22, p75 14.40, p95 14.73, p99 15.60
m3sync-fill-metal-arm64: frames a second: 60.0; GPU time ms: p25 13.93, median 14.36, p75 14.57, p95 14.81, p99 15.08
m3sync-fill-metal-x86_64: frames a second: 60.0; GPU time ms: p25 13.91, median 14.28, p75 14.51, p95 14.71, p99 14.83
m3sync-mix-d3d11: frames a second: 60.0; GPU time ms: p25 14.08, median 14.21, p75 14.42, p95 14.68, p99 14.86
m3sync-mix-metal-arm64: frames a second: 60.0; GPU time ms: p25 12.97, median 13.59, p75 13.89, p95 15.09, p99 15.49
m3sync-mix-metal-x86_64: frames a second: 60.0; GPU time ms: p25 12.94, median 13.57, p75 14.06, p95 14.49, p99 14.86
```

```
m3sync-draws-d3d11: config api d3d11 mode draws size 1600x900 passes 0 draws 4000 sync 1;fps 60.00 frames 1200 interval_ms p50 16.664 p95 17.461 p99 18.435;
m3sync-draws-metal-arm64: config api metal-arm64 mode draws size 1600x900 passes 0 draws 4000 sync 1;fps 60.00 frames 1201 interval_ms p50 16.677 p95 17.008 p99 17.212;gpu_ms p50 0.770 p95 0.818 frames 1201;
m3sync-draws-metal-x86_64: config api metal-x86_64 mode draws size 1600x900 passes 0 draws 4000 sync 1;fps 60.00 frames 1201 interval_ms p50 16.682 p95 17.093 p99 17.598;gpu_ms p50 0.771 p95 0.824 frames 1201;
m3sync-fill-d3d11: config api d3d11 mode fill size 1600x900 passes 8 draws 0 sync 1;fps 59.15 frames 1184 interval_ms p50 16.687 p95 17.530 p99 20.249;
m3sync-fill-metal-arm64: config api metal-arm64 mode fill size 1600x900 passes 8 draws 0 sync 1;fps 60.00 frames 1200 interval_ms p50 16.658 p95 17.063 p99 17.230;gpu_ms p50 14.359 p95 14.814 frames 1200;
m3sync-fill-metal-x86_64: config api metal-x86_64 mode fill size 1600x900 passes 8 draws 0 sync 1;fps 60.00 frames 1201 interval_ms p50 16.659 p95 17.112 p99 17.267;gpu_ms p50 14.285 p95 14.708 frames 1201;
m3sync-mix-d3d11: config api d3d11 mode mix size 1600x900 passes 8 draws 4000 sync 1;fps 59.99 frames 1200 interval_ms p50 16.659 p95 17.515 p99 17.832;
m3sync-mix-metal-arm64: config api metal-arm64 mode mix size 1600x900 passes 8 draws 4000 sync 1;fps 60.00 frames 1201 interval_ms p50 16.665 p95 17.175 p99 17.437;gpu_ms p50 13.596 p95 15.055 frames 1201;
m3sync-mix-metal-x86_64: config api metal-x86_64 mode mix size 1600x900 passes 8 draws 4000 sync 1;fps 60.00 frames 1200 interval_ms p50 16.698 p95 17.169 p99 17.527;gpu_ms p50 13.569 p95 14.494 frames 1200;
```

## Request 11 (Metal System Traces)

`xctrace record` attached without asking for anything. The Direct3D 11 and native traces recorded and exported (19 and 19 tables). The Superposition trace did not: `record` was still running after 90 s and was ended by my time limit (exit 142), and `export` then said "Trace is malformed; instrument run data is missing". The run it was attached to (`tr-sp`) is not in the repository.

```
trace-d3d-device-gpu-info.xml.gz
trace-d3d-gpu-aps-stream.xml.gz
trace-d3d-gpu-counter-info.xml.gz
trace-d3d-gpu-counter-value.xml.gz
trace-d3d-gpu-performance-device-state-intervals.xml.gz
trace-d3d-gpu-performance-state-info.xml.gz
trace-d3d-gpu-performance-state-intervals.xml.gz
trace-d3d-gpu-shader-profiler-interval.xml.gz
trace-d3d-gpu-shader-profiler-sample.xml.gz
trace-d3d-metal-application-encoders-list.xml.gz
trace-d3d-metal-gpu-counter-intervals.xml.gz
trace-d3d-metal-gpu-counter-profile.xml.gz
trace-d3d-metal-gpu-execution-points.xml.gz
trace-d3d-metal-gpu-info.xml.gz
trace-d3d-metal-gpu-intervals.xml.gz
trace-d3d-metal-gpu-state-intervals.xml.gz
trace-d3d-metal-gpu-submission-to-command-buffer-id.xml.gz
trace-d3d-metal-object-dependency-chain-gpu.xml.gz
trace-d3d-metal-visual-highlight-chain-gpu.xml.gz
trace-metal-device-gpu-info.xml.gz
trace-metal-gpu-aps-stream.xml.gz
trace-metal-gpu-counter-info.xml.gz
trace-metal-gpu-counter-value.xml.gz
trace-metal-gpu-performance-device-state-intervals.xml.gz
trace-metal-gpu-performance-state-info.xml.gz
trace-metal-gpu-performance-state-intervals.xml.gz
trace-metal-gpu-shader-profiler-interval.xml.gz
trace-metal-gpu-shader-profiler-sample.xml.gz
trace-metal-metal-application-encoders-list.xml.gz
trace-metal-metal-gpu-counter-intervals.xml.gz
trace-metal-metal-gpu-counter-profile.xml.gz
trace-metal-metal-gpu-execution-points.xml.gz
trace-metal-metal-gpu-info.xml.gz
trace-metal-metal-gpu-intervals.xml.gz
trace-metal-metal-gpu-state-intervals.xml.gz
trace-metal-metal-gpu-submission-to-command-buffer-id.xml.gz
trace-metal-metal-object-dependency-chain-gpu.xml.gz
trace-metal-metal-visual-highlight-chain-gpu.xml.gz
```

## Request 12 (soak)

```
RESULT config api d3d11 mode fill size 1600x900 passes 12 draws 0 sync 1
RESULT fps 59.99 frames 17997 interval_ms p50 16.669 p95 17.390 p99 18.265
```

## Request 13

```
2026-11-16 05:45:47.854 wine[19914:5533229] metal-HUD: 39,52.31,208.43,16.67,0.00,16.67,0.00,16.67,0.00,16.67,0.00,16.66,0.00,16.66,0.00,16.67,0.00,16.67,0.00,16.67,0.00,16.67,0.00,16.67,10.89,16.67,10.89,16.67,10.65,16.67,10.65,16.67,10.57,16.67,10.57,16.67,10.26,16.67,10.26,16.67,10.28,16.67,10.28,16.67,10.10,16.67,10.10,16.67,10.01,16.67,10.01,16.66,9.94,16.66,9.94,16.66,10.03,16.66,10.03,16.67,10.06,16.67,10.06,16.67,9.97,16.67,9.97,16.67,9.97,16.67,9.97,16.67,10.07,16.67,10.07,16.67,10.09,16.67,10.09,16.66,9.98,16.66,9.98,16.67,10.12,16.67,10.12,16.67,10.15,16.67,10.15,16.66,10.19,16.66,1
2026-11-16 05:45:48.355 wine[19914:5533225] metal-HUD: 69,52.31,208.43,16.67,11.43,16.67,11.78,16.67,11.78,16.67,11.93,16.67,11.93,16.67,12.06,16.67,12.06,16.67,12.04,16.67,12.04,16.67,12.23,16.67,12.23,16.66,12.49,16.66,12.49,16.67,12.89,16.67,12.89,16.67,13.09,16.67,13.09,16.67,13.59,16.67,13.59,16.67,13.52,16.67,13.52,16.67,13.70,16.67,13.70,16.67,14.10,16.67,14.10,16.67,14.50,16.67,14.50,16.67,14.52,16.67,14.52,16.67,14.76,16.67,14.76,16.67,14.90,16.67,14.90,16.67,15.10,16.67,15.10,16.67,14.51,16.67,14.51,16.67,14.32,16.67,14.32,16.67,14.52,16.67,14.52,16.67,14.76,16.67,14.76,16.67,14.79,1
2026-11-16 05:45:49.070 wine[19914:5533229] metal-HUD: 92,52.31,208.43,16.67,14.59,16.67,14.59,16.67,13.69,16.67,13.69,16.67,12.45,16.67,12.45,16.67,12.41,16.67,12.41,16.67,12.13,16.67,12.13,16.66,12.05,16.66,12.05,16.67,12.05,16.67,12.05,16.67,12.10,16.67,12.10,16.67,11.95,16.67,11.95,16.66,12.12,16.66,12.12,16.67,11.93,16.67,11.93,16.67,12.03,16.67,12.03,16.67,11.95,16.67,11.95,16.66,12.03,16.66,12.03,16.67,11.95,16.67,11.95,16.67,12.93,16.67,12.93,16.67,12.16,16.67,12.16,16.67,12.14,16.67,12.14,16.67,12.88,16.67,12.88,16.67,13.05,16.67,13.05,350.00,24.96,350.00,24.96,16.67,36.07,16.67,36.07
```

## Request 14 (800x450, ABBA)

| tag | benchmark's FPS (min to max) | HUD frames a second | GPU busy | GPU time ms p25 / median / p75 / p95 | thermal levels seen |
|---|---|---|---|---|---|
| abba-off-1 | 91.1 (32.2 to 132.6) | 58.4 | 58% | 9.06 / 9.61 / 10.44 / 13.69 | 68 0   28 1  104 2  |
| abba-on-1 | 85.6 (12.7 to 132.7) | 54.5 | 62% | 9.45 / 10.06 / 10.69 / 16.13 | 49 0   19 1  124 2  |
| abba-on-2 | 81.0 (14.8 to 132.1) | 53.7 | 62% | 9.67 / 10.31 / 11.79 / 17.88 | 45 0   32 1  116 2  |
| abba-off-2 | 84.6 (22.6 to 135.6) | 54.5 | 59% | 9.07 / 9.64 / 11.33 / 16.91 | 49 0   18 1  126 2  |

`shot-abba-off-1.png` and `shot-abba-on-1.png`: the benchmark's window 90 s after the script started each run (`screencapture -l`, no permission question).

## Request 15 (Overwatch's Wine server requests, 2026-11-16 06:32 to 06:35 local)

Start 1 again with `WINEDEBUG=-all,+server` added, sound off, an empty workspace in front, nothing clicked, 185 s; `server-count.sh` over 5 s of the launch log from 165 s after the script started. The game kept running with the trace on: the HUD's last line is frame 10,610 with a 17.3 ms interval and 8.4 ms of GPU time, so it was drawing at about its usual rate. The launch log was 3.2 GB and is deleted. No screenshot, so "menu" is what the game shows by itself.

```
seconds 5
bytes 117238740
lines 548119
requests 266302
metal-HUD lines 10
```

The requests in those 5 s (`requests.txt`, all of it):

```
228472 select
12275 event_op
9759 close_handle
8838 create_event
2637 release_semaphore
 581 ioctl
 467 set_timer
 467 create_timer
 429 open_key
 304 get_key_value
 294 set_async_direct_result
 292 set_queue_mask
 291 recv_socket
 287 set_cursor
 287 get_window_rectangles
 287 get_thread_input
 140 enum_key
 100 create_key
  80 set_key_value
   7 get_async_result
   4 get_message
   3 send_socket
   1 get_process_debug_info
```

So 228,472 of 266,302 requests are `select` (86%), and with `event_op`, `create_event`, `close_handle` and `release_semaphore` the synchronisation calls are 98%: about 53,000 server round trips a second, of which window, input and registry requests are a few hundred.

# Batch 3 (2026-11-16 07:05 to 07:13 local, 10:05 to 10:13 UTC)

Two more starts of Overwatch left alone, as request 6's start 1 (Macron 0.4.0, sound off, an empty workspace in front, nothing clicked). Files in `docs/perf/m3-batch3/`.

## Request 16

`sample` attached without asking for anything. The `ps` line (pid, %CPU, command) just before the 5 s sample, 150 s after the script started:

```
85820  99.2 Z:\Users\~\Library\Application Support\Steam\steamapps\common\
```

`ow-sample.txt.gz` is the whole sample (5 s at 2 ms).

## Request 17

`handles.txt`, all of it (the launch log was 3.2 GB and is deleted):

```
thread 01e0, zero-timeout waits in 5 s: 217860
polls handle  made by (the last reply returning it before the slice)
217860 03c8 create_event
217860 03c0 create_event
217860 03bc create_event
217860 03b4 create_event
217860 03b0 create_event
217860 03ac create_event
217860 03a4 create_event
217860 03a0 create_event
```

So one thread makes 217,860 zero-timeout waits in 5 s (about 43,600 a second), on eight handles that were each made by `create_event`.
