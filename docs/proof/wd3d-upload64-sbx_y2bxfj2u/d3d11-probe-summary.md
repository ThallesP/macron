## d3d11-probe: PASSED

Where: macOS 26.7.2 on Apple M1 (Virtual). Last stage: done.

| Measure | Value | Needs |
|---|---|---|
| SSIM | 0.996 | >= 0.70 |
| Colour distance | 0.000 | <= 0.10 |
| Menu text | 0 of 0 found | all |

| Negative control | SSIM | Colour distance | Rejected |
|---|---|---|---|
| black frame | 0.000 | 0.320 | true |
| colours inverted | -0.798 | 0.508 | true |
| upside down | 0.019 | 0.226 | true |
| shifted by a fifth | 0.035 | 0.196 | true |
| the desktop without the game | 0.001 | 0.318 | true |
| another game's menu | 0.067 | 0.233 | true |


Timings (s): prepareGame 0, runtimeInstall 0, firstWindow 73, reached 75, total 84,
