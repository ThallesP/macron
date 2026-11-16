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
