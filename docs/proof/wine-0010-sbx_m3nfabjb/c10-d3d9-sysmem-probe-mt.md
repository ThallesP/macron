## d3d9-sysmem-probe: PASSED

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


| Uploads run | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps | Needs |
|---|---|---|---|---|---|---|
| probe32.exe | 2829 | 0 | 0 | 5656 | 7 | no waiting upload |
| probe32.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 3636 | 7272 | 0 | 0 | 9 | waiting uploads (rejected) |
| probe64.exe | 3232 | 0 | 0 | 6464 | 8 | no waiting upload |
| probe64.exe (control, WINE_OPENGL_UNSYNCHRONIZED=0) | 3232 | 6464 | 0 | 0 | 8 | waiting uploads (rejected) |

Why: Ultimate Custom Night (Clickteam Fusion) creates its device with D3DCREATE_SOFTWARE_VERTEXPROCESSING and draws from a dynamic D3DPOOL_SYSTEMMEM vertex buffer locked with NOOVERWRITE, with a system memory index buffer (+d3d9 trace, docs/proof/gate1-m3/). On Thalles's M3 it spent 77% of its thread in glBufferSubData, which Apple's OpenGL over Metal makes wait for the GPU whenever issued draws use the buffer. The sandboxes' software renderer does not wait, so their frame rate cannot show it; the trace can. The control runs with WINE_OPENGL_UNSYNCHRONIZED=0.


Timings (s): prepareGame 0, runtimeInstall 0, firstWindow 59, reached 73, total 246,
