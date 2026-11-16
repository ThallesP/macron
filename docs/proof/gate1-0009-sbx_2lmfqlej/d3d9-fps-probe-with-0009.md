## d3d9-fps-probe: PASSED

Where: macOS 26.7.2 on Apple M1 (Virtual). Last stage: done.

| Measure | Value | Needs |
|---|---|---|
| SSIM | 0.995 | >= 0.70 |
| Colour distance | 0.001 | <= 0.10 |
| Menu text | 0 of 0 found | all |

| Negative control | SSIM | Colour distance | Rejected |
|---|---|---|---|
| black frame | 0.001 | 0.249 | true |
| colours inverted | -0.521 | 0.505 | true |
| upside down | 0.136 | 0.056 | true |
| shifted by a fifth | 0.099 | 0.128 | true |
| the desktop without the game | 0.085 | 0.165 | true |
| another game's menu | 0.114 | 0.195 | true |


| Frame rate run | fps | p50 ms | p95 ms | p99 ms | frames | Needs |
|---|---|---|---|---|---|---|
| probe | 13.2 | 73.46 | 99.06 | 123.06 | 132 | >= 10.2 |
| throttled by 196 ms (control) | 2.3 | 433.84 | 467.96 | 483.92 | 24 | rejected |
| baseline | 12.8 | 74.76 | 108.64 | 123.04 | 128 | probe >= 0.80 of it |

Adapter: NVIDIA GeForce 6800 (vendor 0x10de, device 0x0041). Threshold source: Windows runner (WARP): the 32-bit probe in mix mode ran at 0.885 of the 64-bit one on main (17.43 against 19.69 fps, https://github.com/ThallesP/macron/actions/runs/43634976736) and at 0.911 and 0.884 in branch runs (https://github.com/ThallesP/macron/actions/runs/43634196191); 0.8 leaves room for noise. Macron's own numbers are not part of it..


| Uploads run | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps | Needs |
|---|---|---|---|---|---|---|
| probe32.exe | 4444 | 0 | 0 | 4444 | 11 | no waiting upload |
| probe32.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 4444 | 4444 | 0 | 0 | 11 | waiting uploads (rejected) |
| probe64.exe | 4848 | 0 | 0 | 4848 | 12 | no waiting upload |
| probe64.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 3223 | 3223 | 0 | 0 | 8 | waiting uploads (rejected) |

Why: On a real Apple GPU (Thalles's M3, docs/proof/gate1-m3/), Apple's OpenGL is a layer over Metal and every glBufferSubData or synchronized map into a buffer that issued draws still use submits the command buffer and waits for the GPU: a native program with 3,900 draws a frame ran at 3 fps that way and at 120 with unsynchronized mapped writes. The sandboxes' software renderer does not wait, so their frame rate cannot show it; the trace can. The control runs with WINE_OPENGL_UNSYNCHRONIZED=0, which brings back wined3d's glBufferSubData uploads.


Timings (s): prepareGame 0, runtimeInstall 0, firstWindow 67, reached 81, throttledRun 44, total 274,
