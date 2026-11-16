# Wine patch 0009 on the real GPU (Thalles's MacBook Air M3), 2026-11-16

Runtime `wine-11.21-macron.9` (patches 0001 to 0009), `macron` built from main at 06af9f4, installed into a separate `MACRON_HOME` so the Macron Thalles has installed (0.4.0) was not touched. Ultimate Custom Night started as Steam starts it, with no input: it goes to its character-select menu by itself (about 2,240 draws a frame, two uploads each). Thalles was asleep; each start ran on an empty AeroSpace workspace with Wine's audio driver off for that start, 25 to 45 s.

## Result

| runtime | menu, frames a second |
|---|---|
| 0.4.0 (`macron.6`) | 4.9 to 5.2 |
| `macron.9`, default | **31.0** median over 6 minutes (p10 30.0, p90 31.7, 256 readings) |
| `macron.9`, `WINE_D3D_CONFIG=csmt=0x1` | 30.0 (29.96 to 30.05, locked) |

So 0009 removed the wait: six times faster. It is not the game's own 60 yet.

## Where the frame goes now (default, command stream on the game's thread)

`sample` for 8 s at 2 ms, the game's thread, 2,958 samples; x86 frames attributed to modules with `WINEDEBUG=-all,+loaddll`:

| where | share |
|---|---|
| waiting in `NtUserMsgWaitForMultipleObjectsEx` (the game's own frame timer) | 41.0% |
| `opengl32.so` `wow64_flush_batch` (the queued OpenGL calls, in Apple's OpenGL) | 12.4% |
| `wined3d.dll` | 10.2% |
| x86 code with no named frame on the stack | 10.2% |
| `mmf2d3d9.dll` (Clickteam's Direct3D 9 runtime) | 9.7% |
| `opengl32.dll` | 3.1% |
| `d3d9.dll` | 2.6% |
| `Ultimate Custom Night.exe` | 2.4% |
| `ntdll.dll` | 1.8% |
| `wglSwapBuffers` | 1.4% |

The thread works about 59% of a 32 ms frame, about 19 ms, and waits the other 13 ms. That is what a frame timer on a 16.7 ms tick does when the work takes a little more than one tick: it waits for the next one, and the game runs at half its rate. Nothing here proves the timer is tick-aligned; the steady 31 and the idle 41% are what suggest it. If so, the menu needs its 19 ms brought under 16.7 ms to run at 60, and a night (about 3,900 draws) needs the cost per draw roughly halved.

Per frame, roughly: Clickteam's own code about 4 ms (`mmf2d3d9.dll` and the exe), Wine's about 13 ms (`wined3d.dll`, `opengl32.dll`, `d3d9.dll`, the OpenGL calls themselves, and the unnamed x86 frames).

## With the command stream on its own thread

`WINE_D3D_CONFIG=csmt=0x1`: 30.0 fps, locked. The game's thread: 62% x86 code, 11.5% `cthread_yield`, 11.4% `os_sync_wake_by_address_any`, 11.2% waiting in its timer. The `wined3d_cs` thread: 52.6% waiting for work, 31% x86 code, 15.5% `wow64_flush_batch`. So the second thread sits idle half the time while the game's thread spends a quarter of its time waking it and yielding to it: no gain as it stands.

## Not measured

A night (it needs input to get there), and the picture: no screenshot was compared against a reference in these starts; the one taken with `macron.9` shows the same menu as before.
