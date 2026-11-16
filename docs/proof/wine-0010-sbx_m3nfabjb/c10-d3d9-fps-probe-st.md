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
| the desktop without the game | -0.002 | 0.283 | true |
| another game's menu | 0.114 | 0.195 | true |


| Frame rate run | fps | p50 ms | p95 ms | p99 ms | frames | Needs |
|---|---|---|---|---|---|---|
| probe | 16.2 | 61.01 | 67.07 | 70.67 | 163 | >= 13.1 |
| throttled by 153 ms (control) | 3.3 | 310.01 | 372.95 | 374.42 | 34 | rejected |
| baseline | 16.3 | 59.12 | 72.23 | 81.41 | 164 | probe >= 0.80 of it |

Adapter: NVIDIA GeForce 6800 (vendor 0x10de, device 0x0041). Threshold source: Windows runner (WARP): the 32-bit probe in mix mode ran at 0.885 of the 64-bit one on main (17.43 against 19.69 fps, https://github.com/ThallesP/macron/actions/runs/43634976736) and at 0.911 and 0.884 in branch runs (https://github.com/ThallesP/macron/actions/runs/43634196191); 0.8 leaves room for noise. Macron's own numbers are not part of it..


| Uploads run | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps | Needs |
|---|---|---|---|---|---|---|
| probe32.exe | 5656 | 0 | 0 | 5656 | 14 | no waiting upload |
| probe32.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 5252 | 5252 | 0 | 0 | 13 | waiting uploads (rejected) |
| probe64.exe | 4039 | 0 | 0 | 4040 | 10 | no waiting upload |
| probe64.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 4848 | 4848 | 0 | 0 | 12 | waiting uploads (rejected) |

Why: On a real Apple GPU (Thalles's M3, docs/proof/gate1-m3/), Apple's OpenGL is a layer over Metal and every glBufferSubData or synchronized map into a buffer that issued draws still use submits the command buffer and waits for the GPU: a native program with 3,900 draws a frame ran at 3 fps that way and at 120 with unsynchronized mapped writes. The sandboxes' software renderer does not wait, so their frame rate cannot show it; the trace can. The control runs with WINE_OPENGL_UNSYNCHRONIZED=0, which brings back wined3d's glBufferSubData uploads.


Timings (s): prepareGame 0, runtimeInstall 0, firstWindow 61, reached 75, throttledRun 35, total 241,
