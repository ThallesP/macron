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
| probe | 13.7 | 70.49 | 89.75 | 107.76 | 138 | >= 10.6 |
| throttled by 190 ms (control) | 3.9 | 247.19 | 345.30 | 347.74 | 39 | rejected |
| baseline | 13.2 | 66.79 | 157.39 | 186.57 | 133 | probe >= 0.80 of it |

Adapter: NVIDIA GeForce 6800 (vendor 0x10de, device 0x0041). Threshold source: Windows runner (WARP): the 32-bit probe in mix mode ran at 0.885 of the 64-bit one on main (17.43 against 19.69 fps, https://github.com/ThallesP/macron/actions/runs/43634976736) and at 0.911 and 0.884 in branch runs (https://github.com/ThallesP/macron/actions/runs/43634196191); 0.8 leaves room for noise. Macron's own numbers are not part of it..


Timings (s): prepareGame 0, runtimeInstall 0, firstWindow 6, reached 22, throttledRun 34, total 84,
