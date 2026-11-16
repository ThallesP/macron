# Requests for Thalles's Mac

Experiments only a real Apple GPU can answer, for whoever runs things on Thalles's MacBook Air (M3) while he sleeps. docs/perf/README.md says why each one matters. Results go to `docs/perf/real-mac-results.md` (and the files named below to `docs/perf/m3-batch1/`), pushed to `main`.

Ground rules for all of them:

- Macron 0.4.0 as installed. Nothing here changes Macron for good: request 5 changes the installed runtime and puts it back.
- The Mac on its charger, Steam open but idle (no download), no other heavy apps. Each benchmark run waits by itself until the Mac's thermal pressure is back to nominal.
- Overwatch (request 6) is only started and left on its main menu; nothing is clicked in it.
- Nothing personal goes into the repository: no account names, and the home folder is written `~`. `macron.log` files stay on the Mac (they hold paths with the user name); commit `summary.txt`, `env.txt`, `hud.csv` and `thermal.txt` only.
- If a step fails, write what it printed into the results and go on to the next request.

## Batch 1 (2026-11-16)

### Setup (5 minutes)

```sh
mkdir -p ~/macron-perf && cd ~/macron-perf
git clone --depth 1 https://github.com/ThallesP/macron src
curl -fsSLO https://github.com/ThallesP/macron/releases/download/perf-probes-1/avxprobe.exe
curl -fsSLO https://github.com/ThallesP/macron/releases/download/perf-probes-1/nvprobe.exe
shasum -a 256 avxprobe.exe nvprobe.exe
# must print:
# e3c0d7e5aa75da3b306091349358e8804de16a647cc1d4a9e6ef34ed7a0c8495  avxprobe.exe
# 4f23bf5d8a275112ee6addf69f7dfaa768814e41199a74a261373feb2ae372ca  nvprobe.exe
system_profiler SPDisplaysDataType | grep -E 'Resolution|UI Looks like|Retina'
ls ~/Library/Application\ Support/Macron/runtimes/
```

Record the display lines and the runtime folder's name. The probes are built from `tools/perf/avxprobe.c` and `tools/perf/nvprobe.cpp` (the commands are at the top of each file). Disk needed: about 5 GB in `~/macron-perf`; `rm -rf ~/macron-perf` at the end removes all of it.

### Request 1: AVX and the NVIDIA extension, two small programs (2 minutes)

```sh
cd ~/macron-perf
for v in 0 1; do
  if [ $v = 1 ]; then export ROSETTA_ADVERTISE_AVX=1 DXMT_ENABLE_NVEXT=1; fi
  SteamAppId=990001 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/probes macron waitforexitandrun "$PWD/avxprobe.exe" "Z:$HOME/macron-perf/avx-$v.txt" >/dev/null 2>&1
  SteamAppId=990001 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/probes macron waitforexitandrun "$PWD/nvprobe.exe" "Z:$HOME/macron-perf/nv-$v.txt" >/dev/null 2>&1
done
unset ROSETTA_ADVERTISE_AVX DXMT_ENABLE_NVEXT
for i in 1 2 3; do ROSETTA_ADVERTISE_AVX=1 SteamAppId=990001 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/probes macron waitforexitandrun "$PWD/avxprobe.exe" "Z:$HOME/macron-perf/avx-1-$i.txt" >/dev/null 2>&1; done
cat avx-*.txt nv-*.txt
```

(The first start creates a Windows environment and takes a minute.) Read:

- `avx-1*.txt`, the line `RESULT avx2 ... x the SSE2 time`. On the sandbox it was 1.56 and 1.81 (AVX2 code slower than SSE2 code under Rosetta). Similar here means `ROSETTA_ADVERTISE_AVX` stays a per-game compatibility switch and is no speed-up. Below 1.0 would mean AVX code is faster on the M3, and the variable becomes a candidate for games with AVX paths.
- `nv-1.txt`, the line `RESULT DLSS feature`. `created (MetalFX temporal scaler)` means a game's DLSS calls can run on this Mac through DXMT (candidate 1 goes on to request 6). `not created` means the M3's MetalFX temporal scaler is unavailable to DXMT, and candidate 1 is dead here.

### Request 2: does the benchmark run, and the baseline (about 20 minutes)

```sh
cd ~/macron-perf/src
tools/perf/bench.sh setup superposition
tools/perf/bench.sh run superposition base-a 1600x900 windowed
tools/perf/bench.sh run superposition base-b 1600x900 windowed
```

Each `run` prints its summary (also in `~/macron-perf/results/<tag>/summary.txt`). A window opens and plays Superposition's benchmark by itself; leave it alone. Read:

- First, whether it ran: `HUD frames logged` in the thousands and no `GPU Hang` or `Device error` lines. **If it did not** (few frames, `GPU Hang`, `timed out`, or the window shows an error), run `tools/perf/bench.sh setup heaven` and do this request and requests 3 to 5 with `heaven` in place of `superposition`, writing which one was used. On the sandbox both hang its virtual GPU, so this is the first real answer.
- `GPU busy`: the share of the time the GPU was working. Near 100% means the GPU is the limit, as in Overwatch's practice range. Well under 100% (say under 85%) means the GPU waits for something each frame (the CPU, DXMT's encoding, presenting), and that wait is worth chasing before anything else.
- `frames a second` of base-a against base-b: how much two identical runs differ. Later differences smaller than that are noise.
- In `engine-log.txt`, the lines `render_shaders_quality` and `render_textures_quality`: whether the benchmark took quality 1 (medium) from the command line.

### Request 3: GPU time against resolution (about 30 minutes)

```sh
tools/perf/bench.sh run superposition res-1280 1280x720 windowed
tools/perf/bench.sh run superposition res-960 960x540 windowed
tools/perf/bench.sh run superposition res-640 640x360 windowed
```

Read the median `GPU time ms` of base-a (1.44 million pixels), res-1280 (0.92), res-960 (0.52) and res-640 (0.23). If GPU time falls about in proportion to pixels, rendering fewer pixels and upscaling (candidates 1 and 4) is the lever. If a large part stays the same at every size (say res-640 still above half of base-a), that part is fixed per-frame work and the next request will be a Metal System Trace to find whose it is.

### Request 4: what MetalFX spatial upscaling costs (about 20 minutes)

```sh
tools/perf/bench.sh run superposition mfx-off 800x450 windowed
tools/perf/bench.sh run superposition mfx-on 800x450 windowed DXMT_METALFX_SPATIAL_SWAPCHAIN=1
```

Both render 800x450; mfx-on has DXMT upscale each frame to 1600x900 with MetalFX before presenting (sharp on the Retina panel instead of stretched by macOS). Read the median `GPU time ms` of both: the difference is what the upscale costs. Compare mfx-on with base-a (1600x900 rendered by the game): if mfx-on is much cheaper, DXMT's upscaling is a cheap way to get a full-size picture from a smaller render. Also check that mfx-on's summary shows no new `err:` lines.

### Request 5: Game Mode (about 40 minutes)

Full-screen runs at the size macOS gives the desktop (the `UI Looks like` numbers from the setup, for example 1920x1243), alternating without and with Game Mode. Set `W` and `H` to those numbers and `RT` to the runtime folder from the setup:

```sh
W=1920 H=1243
RT="$HOME/Library/Application Support/Macron/runtimes/<runtime folder>/wine-dxmt"
cd ~/macron-perf/src
tools/perf/bench.sh run superposition fs-a ${W}x${H} fullscreen

tools/perf/gamemode-bundle.sh apply "$RT"
log stream --style compact --predicate 'process == "gamepolicyd"' > ~/macron-perf/gamemode-a.log & logger=$!
tools/perf/bench.sh run superposition gm-a ${W}x${H} fullscreen
kill $logger
tools/perf/gamemode-bundle.sh revert "$RT"

tools/perf/bench.sh run superposition fs-b ${W}x${H} fullscreen

tools/perf/gamemode-bundle.sh apply "$RT"
log stream --style compact --predicate 'process == "gamepolicyd"' > ~/macron-perf/gamemode-b.log & logger=$!
tools/perf/bench.sh run superposition gm-b ${W}x${H} fullscreen
kill $logger
tools/perf/gamemode-bundle.sh revert "$RT"
tools/perf/gamemode-bundle.sh status "$RT"    # must print "not applied"

grep -h -E 'Found game|Game mode' ~/macron-perf/gamemode-*.log
```

The first start with `apply` may show "Allow "Macron Game" to find devices on local networks?": click **Allow**, and write down that it appeared (Wine waits until it is answered). During gm-a and gm-b the menu bar should show the Game Mode rocket. Read:

- The `grep`: `Found game ... labelReason=infoPlist`, then whether a line says `Game mode status is now on` (the sandbox only reached `paused`, with a window). If it never says `on`, Game Mode does not engage for Wine's full-screen window and the frame rates below do not matter.
- `frames a second`, `GPU time ms` and the interval p95 and p99 of gm-a and gm-b against fs-a and fs-b. A difference larger than base-a against base-b means Game Mode matters; frame-time tails (p99) are where it is expected to show, if anywhere.
- `fs-a` against `base-a` also says what full screen itself does (a larger frame, but presented without the compositor).
- Before finishing, `status` must say `not applied`: the runtime is back as it was.

### Request 6: Overwatch on its main menu (about 15 minutes; his account, nothing clicked in the game)

Start Overwatch from a terminal exactly as on 2026-11-16 (`SteamAppId=2357570`, `STEAM_COMPAT_DATA_PATH` of its compatdata, `macron waitforexitandrun` on `Overwatch.exe`, Steam running), but **without** the `show-fps` file (it overrides `WINEDEBUG`; `rm ~/Library/Application\ Support/Macron/show-fps` if it exists) and with these variables added in front of the command. Three starts, each left on the main menu for 90 seconds after the menu appears, then the game quit the way it was quit last time:

1. `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1` (the baseline). While the menu shows, in another terminal:

   ```sh
   top -l 4 -s 5 -o cpu -n 12 -stats pid,command,cpu,threads,mem > ~/macron-perf/ow-top.txt
   ```

2. `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1 DXMT_ENABLE_NVEXT=1 WINEDEBUG=+loaddll`. DXMT then tells the game the GPU is NVIDIA's and answers its DLSS calls with MetalFX; nothing in the game's files changes. Overwatch has anti-cheat: whether to run this start is the operator's call, and skipping it is fine (write so).

3. Nothing extra is needed for the third: copy the game's graphics settings, which say what it renders at:

   ```sh
   grep -i -E 'resolution|scale|upsampl|quality|fullscreen|window|frame|vsync|limit|dlss|fsr' \
     "<compatdata>/pfx/drive_c/users/<user>/Documents/Overwatch/Settings/Settings_v0.ini"
   ```

Then, for starts 1 and 2, copy each launch log out of `~/Library/Application Support/Macron/logs/steam-2357570/` (the newest two) to `~/macron-perf/ow-1.log` and `ow-2.log`, and summarise them with the bench script's parser:

```sh
cd ~/macron-perf/src
for n in 1 2; do mkdir -p ~/macron-perf/results/ow-$n && cp ~/macron-perf/ow-$n.log ~/macron-perf/results/ow-$n/macron.log && echo "Overwatch menu, start $n" > ~/macron-perf/results/ow-$n/env.txt && tools/perf/bench.sh summary ow-$n; done
grep -i -E 'nvngx|nvapi64|NVEXT|MetalFX' ~/macron-perf/ow-2.log | sed 's/^.*\(Loaded\|info\)/\1/' | sort -u | head
```

Read:

- ow-1's `GPU busy` and median `GPU time ms` on the menu, and in `ow-top.txt` the CPU share of `wine` (the game) and of `wineserver`. A `wineserver` above about 20% of a core means server round trips are a real cost and msync (candidate 6) moves up; a few percent means it stays down.
- ow-2's `grep`: a line with `nvngx.dll` loaded by the game (`Loaded L"...nvngx.dll" ... builtin`) and DXMT's `Vendor extension enabled: NVEXT` mean Overwatch took the NGX path, so DLSS should be offered in its video settings under DXMT. Then Thalles can choose DLSS himself later (it needs a settings change in his account) and the Metal HUD will show `MetalFX: Temporal` and the scale. No `nvngx.dll` line means Overwatch does not use NGX here and candidate 1 does not apply to it. Also compare ow-2's GPU time with ow-1's: telling the game it runs on NVIDIA must not make the menu slower.
- The settings lines: the render scale and upsampling mode Thalles plays with, which says how many pixels the 15.7 ms were for.

### Request 7: is Xcode's trace tool there? (1 minute)

```sh
xcrun xctrace version
```

If it prints a version, the next batch can ask for a 5-second Metal System Trace of the benchmark, which shows each GPU pass's time (DXMT's own passes included). If it fails, that request will not be made.

### Request 8: DXMT against native Metal on the same load (about 15 minutes; added after batch 1 was written, run it with batch 1 if there is time)

```sh
cd ~/macron-perf
curl -fsSLO https://github.com/ThallesP/macron/releases/download/perf-probes-2/gpuload.exe
curl -fsSLO https://github.com/ThallesP/macron/releases/download/perf-probes-2/gpuload
chmod +x gpuload
shasum -a 256 gpuload.exe gpuload
# must print:
# 047d28f4b4b380d286d9a6aa58243397c00355c6f5e849f4bdb53937dd69f80d  gpuload.exe
# 4fff6849a06d122113255e48d06bb1e573dbaae05eea63aaddeacb18db99f2d8  gpuload
cd src && git pull && tools/perf/bench.sh pair m3 1600x900
```

`pair` runs the same GPU work nine times, 20 seconds each, waiting for the Mac to cool between runs: three loads (`fill`: eight full-screen shader passes; `draws`: 4,000 small draws, each with its own constants; `mix`: both) as Direct3D 11 through Macron's DXMT and as native Metal under Rosetta and on arm64. It prints a table (also in `~/macron-perf/results/m3-table.txt`). If macOS refuses to open `gpuload` (it is signed ad hoc, not by a developer), `xattr -d com.apple.quarantine gpuload` and run `pair` again with another tag. Read:

- For each load, the median `GPU time ms` of `d3d11` against `metal-x86_64`: the difference is what DXMT adds on the GPU for the same work (its presenting pass included). Within a few percent means DXMT's translation is not where Overwatch's 15.7 ms go and candidate 5 is closed. A large difference on `fill` points at DXMT's shaders or passes; on `draws`, at its per-draw work.
- `metal-x86_64` against `metal-arm64`: what Rosetta costs a native program, for scale.
- Each run's `bench.txt` also has the program's own figures; for the native runs, `gpu_ms` is measured from Metal's command buffers and should match the HUD's `GPU time ms`. If the HUD's lines are missing for the native runs, read `gpu_ms` instead and say so.

### What to push

`docs/perf/real-mac-results.md`: the display lines and runtime name from the setup; request 1's eight files' contents; for every run of requests 2 to 6 its `summary.txt` (or what went wrong); request 5's `grep` output and whether the network question appeared; request 6's `ow-top.txt`, the `grep` lines and the settings lines; request 7's output; request 8's table and each run's `bench.txt`. Into `docs/perf/m3-batch1/<tag>/`: each run's `summary.txt`, `env.txt`, `hud.csv`, `thermal.txt`. Then a message to this thread.

## Ultimate Custom Night (gate 1 thread, Opus 5.5)

Context: `docs/proof/gate1-m3/macron9.md`. With `macron.9` the menu runs at 31 fps; the game's thread works about 19 ms a frame (13 of them in Wine) and waits for its frame timer for the rest. The menu is the benchmark (no input needed). Same setup as macron9.md: `macron` built from main, its runtime in a separate `MACRON_HOME`, empty workspace, sound off. Results go to `docs/proof/gate1-m3/` with the home folder written `~` (`sed -E 's|/Users/[^/ ]+|~|g'`).

### U1: the raw sample, for function-level attribution (2 minutes, or none if the macron9.md run's files were kept)

Commit, scrubbed as above, to `docs/proof/gate1-m3/macron9-raw/`:
- the raw `sample` output of the game's thread in the menu, default configuration (`sample <pid> 8 2 -file ...`; the whole file, with the `???` frames and their addresses);
- from the same start, the `+loaddll` lines (`WINEDEBUG=-all,+loaddll`; only the `Loaded L"..." at <address>` lines).

What it means: the overlay's `wined3d.dll` and `opengl32.dll` carry DWARF. With the load addresses I resolve every x86 frame, the 10.2% "no named frame" included, to a function and a line, and the native frames under `wow64_flush_batch` name Apple's OpenGL calls. That says which of these is the 13 ms: d3d9's copy of the system memory vertex buffer before each draw, the index streaming, the maps that go through wined3d's command stream (four a draw for this game), state application, the batch replay, or the map and unmap in each unsynchronized write.

### U2: the menu without 0007's batching (2 minutes)

The same start with `WINE_OPENGL_BATCH=0` in the environment: the frame rate over a minute (median of the `trace:fps` readings), and, if easy, the same 8 s `sample` as U1.

What it means: on the M3 a transition into Wine's Unix side costs 0.6 to 0.9 µs, and 0007 copies every queued call and replays it through the WoW64 thunks. If the menu is as fast or faster without batching, the replay costs more than the transitions it saves there, and the next change takes work out of the replay. If it is clearly slower, batching earns its keep on real hardware, and what is left to queue is worth queuing.
