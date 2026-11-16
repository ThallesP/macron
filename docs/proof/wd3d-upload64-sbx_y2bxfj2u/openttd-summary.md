## openttd: PASSED

Where: macOS 26.7.2 on Apple M1 (Virtual). Last stage: done.

| Measure | Value | Needs |
|---|---|---|
| SSIM | 0.944 | >= 0.70 |
| Colour distance | 0.001 | <= 0.10 |
| Menu text | 5 of 5 found | all |

| Negative control | SSIM | Colour distance | Rejected |
|---|---|---|---|
| black frame | 0.004 | 0.066 | true |
| colours inverted | -0.808 | 0.048 | true |
| upside down | 0.069 | 0.005 | true |
| shifted by a fifth | 0.063 | 0.026 | true |
| the desktop without the game | 0.004 | 0.066 | true |
| another game's menu | 0.047 | 0.045 | true |


Timings (s): prepareGame 1, runtimeInstall 0, firstWindow 57, reached 57, total 83,
