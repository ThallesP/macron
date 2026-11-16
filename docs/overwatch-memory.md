# Overwatch's 15 GB under Macron

On Thalles's MacBook Air M3 (16 GB) the game's process had a footprint of 15 GB, about 12 GB of it macOS `malloc` on Wine's Unix side, and the Mac swapped while he played (docs/proof/overwatch-m3/). This is where that memory comes from, measured on `sbx` sandboxes, and what Macron changes about it.

**In one sentence:** DXMT (Macron's Direct3D 11 layer) parses every shader the game creates into a large in-memory form and keeps it for as long as the game runs, and Overwatch creates about 83,000 shaders before its sign-in screen, so the parsed forms alone hold about 8 GB.

## Reproduced on a sandbox

Sandbox sbx_agft7kmb (GitHub macOS 26 runner: 7.5 GB of memory, 3 cores, the Apple Paravirtual GPU), Steam signed in to the test account, Macron 0.4.0 installed from the release DMG (SHA-256 checked; runtime `wine-11.21-macron.6`, DXMT 0.80, the Steam bridge), Overwatch (2357570) installed with Steam's Install button. `tools/overwatch-memory/ow-mon.sh` read the game process's `footprint` every 3 s.

**Run A, Steam's Play button, frame counter off.** The process appeared 82 s after Play. Its footprint went from 0.15 GB to 7.9 GB in 133 s and then stayed between 7.91 and 7.95 GB until it was stopped at 228 s:

| s after the process appeared | footprint | MALLOC_SMALL | MALLOC_MEDIUM | MALLOC_NANO | MALLOC_TINY | WINE_RESERVE |
|---|---|---|---|---|---|---|
| 31 | 1.2 GB | 337 MB | 22 MB | 50 MB | 23 MB | 567 MB |
| 65 | 3.5 GB | 2,056 MB | 177 MB | 214 MB | 97 MB | 621 MB |
| 99 | 5.8 GB | 3,970 MB | 424 MB | 372 MB | 159 MB | 566 MB |
| 133 | 8.0 GB | 5,653 MB | 646 MB | 511 MB | 214 MB | 512 MB |
| 228 | 7.9 GB | 5,653 MB | 646 MB | 512 MB | 215 MB | 511 MB |

Same shape as the M3: almost all of it is `malloc` (SMALL, then MEDIUM, NANO and TINY), the game's Windows memory (`WINE_RESERVE`) stays near 0.5 GB, graphics about 0.15 GB. DXMT's shader cache on disk stayed under 1 MB, so very few shaders were compiled for Metal.

**Run B, the same start from a shell with `MallocStackLogging=lite`** (`ow-run.sh B-msl shell 900 MallocStackLogging=lite`, the way the M3 session was started). The footprint reached 9.07 GB at 118 s (stack logging costs memory of its own) and stayed within 5 MB of that until 900 s, at the same screen. It grows once, at start, and then does not grow.

## Who allocates it

At 180 s into run B, `heap` and `malloc_history -callTree` on the game's process (`ow-heap.sh`; `calltree.py` summarises the tree):

- **8.04 GB of live `malloc` in 6.4 million blocks.**
- **7.93 GB of it was allocated under `SM50Initialize` in DXMT's `winemetal.so`**, called from Wine's `ntdll.so` (the call from the Windows side into a Unix library). `SM50Initialize` is DXMT's shader parser: it reads a shader's DXBC bytecode into DXMT's own representation, which the shader compiler (`SM50Compile`, LLVM) later turns into Metal.
- 7.29 GB of that is inside `dxmt::dxbc::read_control_flow`, in the `std::variant` visitors over DXMT's instruction types (`InstMovConditional`, `InstSwapConditional`, `InstDotProduct`, ...): the decoded instructions of every shader's basic blocks. The rest is the shaders' argument buffer descriptions (0.47 GB, `ArgumentBufferBuilder::DefineInteger64` and `DefineTexture`) and smaller pieces.
- **82,706 shaders, all alive.** Two allocation sites in `SM50Initialize` run exactly once per call: the error object it creates first (32 bytes, `+ 76`) and the `SM50ShaderInternal` itself (784 bytes, `+ 922`). Both have exactly 82,706 live blocks, so 82,706 shaders were parsed and none was freed. 7.93 GB over 82,706 is about 96 KB a shader.
- Everything else in the heap is small: the Steam client library, Metal, Wine's own Unix side, each under a few MB.

Why none is freed, from DXMT 0.80's source (`src/d3d11/d3d11_pipeline_cache.cpp`): `PipelineCache::CreateShader` parses the bytecode with `SM50Initialize` and stores the result in a `CachedSM50Shader` in `shaders_`, a map keyed by the bytecode's SHA-1 that is never pruned, for the life of the device; the D3D11 shader objects the game holds only point into it. The parsed form is read in exactly one place: the compile jobs (`d3d11_shader.cpp`, every `shader->handle()`), which run when a shader variant is first needed for a draw and DXMT's on-disk shader cache does not already have it. The reflection and argument information the device needs at creation are copied out of the parse into plain data. DXMT's main branch (408 commits after v0.80, checked on 2026-11-16) still keeps every parsed shader the same way.

So: **not a leak and not a cache of compiled shaders; per-shader data kept.** Overwatch creates its whole shader set up front (82,706 at the sign-in screen) and draws with a small part of it; DXMT keeps all of them in a form that is tens of times larger than the bytecode a Windows driver is given.

## What Macron changes

Wine is not involved and no DXMT setting changes it (DXMT 0.80's settings: `DXMT_SHADER_CACHE`, `DXMT_SHADER_CACHE_PATH`, `DXMT_USE_DEFAULT_METAL_CACHE`, logging and capture; none touches the parsed shaders). So the fix is a patch to DXMT, `runtime/dxmt/patches/0001-d3d11-parse-shaders-on-first-compile.patch`, in `d3d11.dll` only (the Windows side; DXMT's `winemetal.so` and its LLVM are unchanged):

- At creation, a shader is parsed as before, its reflection and argument information are taken, and the parsed form is freed; the shader keeps a copy of its bytecode.
- The first time a compile job asks for the parsed form, the bytecode is parsed again (once, under `std::call_once`), the result is kept as before, and the bytecode copy is dropped.

A shader that is drawn therefore ends up exactly as it is today, parsed and kept; one that is never drawn costs its bytecode. A variant that DXMT's on-disk shader cache already has needs no parse at all.

`runtime/dxmt/build.sh` builds DXMT 0.80's `d3d11.dll` (x86_64 and i386) from its tag with the patch, with the llvm-mingw release DXMT's CI uses (pinned by checksum), against the Wine tree DXMT's CI uses for its import libraries, then marks it a Wine builtin (`winebuild --builtin`, as DXMT's own build does; without it Wine does not load the file as its builtin) and strips it. It packs `lib/wine/<arch>-windows/d3d11.dll` with the patch, the patched source file and DXMT's LGPL licence as `dxmt-0.80-macron.1-overlay.tar.xz`. The `DXMT build` workflow builds and publishes it; the runtime's manifest pins it (`DXMTOverlay`), and `macron runtime install` copies it into the DXMT tree only, after DXMT's own files, so the plain tree keeps Wine's `d3d11.dll`.

DXMT's own 0.80 release was built with Homebrew's mingw-w64 GCC 15.2; Homebrew now has GCC 16, which rejects DXMT 0.80's sources (`com_guid.cpp` uses `std::setw` without `<iomanip>`), so the patched build uses clang. To tell the compiler's effect from the patch's, the same script also builds an unpatched control (`DXMT_UNPATCHED=1`) and both were measured against DXMT's released file.

## Before and after, at the same sign-in screen

Sandbox sbx_5wajz2zm (the same kind of runner, set up the same way: Steam from Valve's DMG, the test account, Macron 0.4.0 from the release, Overwatch with Steam's Install button). `ow-compare.sh` copies one x86_64 `d3d11.dll` into the installed runtime's DXMT tree, records its SHA-256, and starts the game; `ow-mon.sh` follows the footprint and takes a picture of the screen every 30 s. In the shell-started runs Wine's `loaddll` channel shows `d3d11.dll` loaded as `builtin`, which is the file in the runtime's tree (a copy in the prefix would load as `native`).

| run | `d3d11.dll` | started with | screen at the end | footprint, level from | at the end |
|---|---|---|---|---|---|
| P4 | DXMT 0.80 as released (`7ca382af…`) | Steam's Play | Battle.net sign-in | about 208 s | **7.95 GB** |
| R2 | DXMT 0.80 built by `build.sh`, unpatched (`DXMT_UNPATCHED=1`, `c1d7dc73…`) | shell | Battle.net sign-in | about 204 s | **7.92 GB** |
| P1 | patched (`7f806a4f…`) | Steam's Play | Battle.net sign-in | about 199 s | **1.82 GB** |
| P3 | patched, again | Steam's Play | Battle.net sign-in | about 204 s | **1.83 GB** |

(Run A on the first sandbox, DXMT as released and Steam's Play, levelled at 7.9 GB as well.) Masked pictures of P3's and P4's last screens: `docs/proof/overwatch-memory/sign-in-*-masked.png` (the Steam profile name, the QR code and the link code are blacked out).

- **The patch takes the process from 7.9 GB to 1.8 GB at the sign-in screen**, a 6.1 GB drop, and the unpatched build of the same source with the same compiler (7.92 GB) shows that the compiler change is not what does it.
- Where the 1.8 GB is (P3, 250 s, `footprint` and `heap`): live macOS `malloc` is **31 MB in 198,724 blocks** (8.04 GB in 6.4 million before); `MALLOC_SMALL` is gone from the footprint's top categories, `MALLOC_MEDIUM` 19 MB, `MALLOC_NANO` 10 MB. The game's Windows memory (`WINE_RESERVE`) is 1,390 MB against 513 MB before: 877 MB more, which is about what keeping 82,706 shaders' bytecode on the Windows side would take (10.6 KB a shader). That last attribution is an inference: macOS's tools do not see inside Wine's Windows heap, and the bytecode size was not measured on its own.
- The game reached the sign-in screen in about the same time with and without the patch (199 to 208 s on this 3-core sandbox; the times vary by tens of seconds from run to run).

### Runs that did not reach the screen, and what they show

Overwatch on these sandboxes does not reach the sign-in screen every time, with either `d3d11.dll`, and the footprint depends on how far it got, so only runs that reached the same screen are compared above. The others are in `docs/proof/overwatch-memory/sbx_5wajz2zm/` with their timelines:

- Started from a shell, the game often opened behind Steam's window and stopped making progress (R1 released 2.65 GB, R4 and R8 patched 1.80 GB); a `sample` of R8 shows its threads waiting, about 0.7 of a core of translated code running. Started with Steam's Play button it came to the front every time.
- Two runs sat on a black screen (R3 patched at 0.66 GB, R7 released at 3.93 GB). R7's script also clicked the game's Dock icon 49 times (`front.log`), which may have disturbed it.
- R6 (patched, shell) crashed 12 s in, after `d3d11.dll` was loaded: `Unhandled page fault on execute access to 00000001400AFD90` inside Overwatch.exe's own image. It happened in one of six patched starts and in none of six starts with the released file (two on the first sandbox); one in six is too few to tell whether the patch has anything to do with it.
- P2 (released, Play) was at 6.7 GB when `ow-mon.sh` went 50 s without a reading, and Overwatch exited with code 5 at 186 s. The sandbox has 7.5 GB of memory and 1 GB of swap; whether memory made the game exit is not established.

## On Thalles's M3 (request M1)

Run for this thread on his MacBook Air M3 on 2026-11-16 (`docs/proof/overwatch-memory/m3-m1/README.md`): Overwatch started the way Steam starts it, on APFS clones of his Overwatch Windows environment, signed in to his account, left alone about four minutes on the screen it shows by itself after signing in, with `ow-mon.sh` following the process and the Metal HUD logging. Both builds use the same Wine (`macron.11`); only DXMT's `d3d11.dll` differs.

| start | `d3d11.dll` | footprint, level | memory pressure | swap in use |
|---|---|---|---|---|
| `dxmt-m1` (the patch) | `68b71e3c...` (the published overlay's) | **3.40 to 3.47 GB** from 55 s on | 1 (normal) throughout | 5,990 to 5,974 MB |
| control, main at `acf610a` | `7ca382af...` (DXMT 0.80 as released) | **14.24 to 14.25 GB** from 111 s on | 2 (warning) from the first reading | 5,966 to 7,699 MB |

The HUD's memory figure agrees: 3.4 GB patched, 14.2 to 14.3 GB in the control (14.3 GB with 0.4.0 in request 6). In the patched start's `footprint` no `MALLOC` category is among the first rows; the game's Windows memory is 2.4 GB and graphics 0.66 GB. **So the M3's 12 GB of `malloc` was the parsed shaders, as on the sandboxes, and the patch removes it there: 14.25 GB to 3.4 GB, with no swapping and no memory pressure.** Neither start crashed. The frame rates (50.0 patched, 43.3 control, 53.5 for 0.4.0 earlier in the night) come from starts minutes apart on a Mac that had run benchmarks for hours, so they say nothing about the patch either way; frames over 50 ms after the first 20 s were 38 patched and 39 control.

## What is established and what is not

Established, with the measurements above:

- On Thalles's M3 the same: 14.25 GB with DXMT as released, 3.4 GB with the patch, on the same Wine, the same screen and his account (M1).

- On a sandbox, Overwatch's process grows to 7.9 GB before its sign-in screen and then stays level; about 7 GB of it is macOS `malloc` (run A: `MALLOC_SMALL` 5.65 GB, `MEDIUM` 0.65, `NANO` 0.51, `TINY` 0.22), the same categories as on the M3.
- 7.93 GB of the 8.04 GB of live `malloc` was allocated by DXMT's shader parser (`SM50Initialize`, in `winemetal.so`): 82,706 parsed shaders, about 96 KB each, all kept alive by DXMT's pipeline cache, which never frees them.
- It is DXMT's (not Wine's, not Macron's setup); no setting of DXMT 0.80 changes it, and DXMT's main branch still does it.
- With DXMT's `d3d11.dll` parsing a shader again when it is first compiled instead of keeping every parse, the process is 1.8 GB at the same screen, with 31 MB of live `malloc`.

Not established:

- **A match or the practice range.** Where 0.4.0 held 15.2 to 15.6 GB on the M3; M1 measured the screen after signing in, and a match needs Thalles at the keyboard. Whether the hitches he felt during swapping are gone is his to say after playing the release candidate.
- **Frame time.** A shader that is drawn for the first time is now parsed again before it is compiled. That is once per shader, next to an LLVM and Metal compile that takes much longer, and a variant DXMT's on-disk cache already has needs no parse; but it was not measured, and the sandbox's GPU cannot show the frame rate the M3 has. The 16 hitches where DXMT's shader cache grew are a separate question this change does not address.
- **Games other than Overwatch.** The change is in DXMT's `d3d11.dll` for every Direct3D 10 and 11 game on the default backend. The scenario checks (the Direct3D 11 probe) cover that it still draws; a game that creates few shaders gains nothing and keeps a bytecode copy per shader.
- **The Windows side.** The 877 MB growth of `WINE_RESERVE` is attributed to the bytecode copies by size, not measured.

## Shipping

- The overlay is published as [runtime-dxmt-0.80-macron.1](https://github.com/ThallesP/macron/releases/tag/runtime-dxmt-0.80-macron.1) (`dxmt-0.80-macron.1-overlay.tar.xz`, SHA-256 `d3982b6ad010d45d9330ba73ec2b1af7a95b5423956ffabe2ab86ceea79264e2`, 1,353,168 bytes), built by the `DXMT build` workflow on main. It holds `lib/wine/x86_64-windows/d3d11.dll` (SHA-256 `68b71e3c...`) and `lib/wine/i386-windows/d3d11.dll` (`3a601ced...`), both marked as Wine builtins, and under `share/macron/dxmt/` the patch, the patched source file, DXMT's licence and the build's compiler.
- The runtime pins it in `Manifest.DXMTOverlay`; the runtime's version names it (`...-dxmt-0.80-macron.1-...`), so an installed Macron that updates gets a fresh runtime with it.
- It goes to Thalles in v0.5.0-rc3, the release candidate after rc2 (rc2 was already built and under test when the pin was ready).
- **Checked with rc3's own DMG** (Release run 43847983983 from 3ea51fd, `Macron-0.5.0-rc3.dmg` SHA-256 `4d512ec0...`, the file that is published) on a fresh sandbox, sbx_b6zgiskh: the installer put the overlay's `d3d11.dll` (`68b71e3c...`) into the runtime; Overwatch installed with Steam's Install button and started with Play reached the Battle.net sign-in screen and levelled at **1.84 GB** (`MALLOC_MEDIUM` 25 MB, `MALLOC_NANO` 17 MB, `WINE_RESERVE` 1,387 MB), as in the patched runs before. `docs/proof/overwatch-memory/rc3-sbx_b6zgiskh/` (the picture masked).
- On his M3: request M1 in `docs/perf/real-mac-requests.md` (branch `dxmt-m1`), answered above: 14.25 GB to 3.4 GB.

## Upstream

The change belongs in DXMT. The patch applies to v0.80; DXMT's main branch has the same `CachedSM50Shader` keeping the parse, so it should carry over with small changes. Offering it is Thalles's decision (it would go to 3Shain/dxmt from his account): see "Needs Thalles" in PROGRESS.md.

## Files

- `tools/overwatch-memory/`: `ow-run.sh` (start, follow, stop), `ow-mon.sh` (footprint every 3 s), `ow-heap.sh` (`heap`, `vmmap`, `malloc_history`), `calltree.py` (summarise a call tree), `ow-compare.sh` (swap `d3d11.dll` and run), `account-lock.sh` (the test account's one-machine lock).
- `runtime/dxmt/`: the patch, `build.sh`, `release.env`; `.github/workflows/dxmt-build.yml` builds and publishes it.
- `docs/proof/overwatch-memory/`: every run's timeline (`mon.tsv`), last `footprint`, the DLL's SHA-256, Wine's `loaddll` lines; run B's heap census and call tree summary (`calltree-t180-over-50MB.txt`); P3's heap census.
