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

- `avx-1*.txt`, the line `RESULT avx2 ... x the SSE2 time`. On the sandbox it was 1.16 to 1.81 over seven runs (AVX2 code slower than SSE2 code under Rosetta). Similar here means `ROSETTA_ADVERTISE_AVX` stays a per-game compatibility switch and is no speed-up. Below 1.0 would mean AVX code is faster on the M3, and the variable becomes a candidate for games with AVX paths.
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

## Batch 2 (2026-11-16, after batch 1's first answers)

Same ground rules, plus what batch 1 taught: keep each benchmark or `gpuload` window floating (AeroSpace) and, for Superposition, the active app for the whole run; check the render resolution in each `bench.txt`. These need the new build of the two `gpuload` programs (a `--sync` switch). The Direct3D one is published; the native one is built on the Mac with Xcode's compiler (Xcode is there, request 7), because no sandbox was free to build it:

```sh
cd ~/macron-perf/src && git pull && cd ..
curl -fsSL -o gpuload.exe https://github.com/ThallesP/macron/releases/download/perf-probes-3/gpuload.exe
shasum -a 256 gpuload.exe
# must print: 9b77bd9cc2d28d5e1d822f47287479b8a0c1e2b9224715005348779deb8071b4  gpuload.exe
clang -O2 -fobjc-arc -arch x86_64 -arch arm64 -o gpuload src/tools/perf/gpuload.m -framework Metal -framework QuartzCore -framework AppKit
./gpuload --mode fill --size 640x360 --seconds 2 --sync 1 --out check.txt >/dev/null && cat check.txt   # must print config ... sync 1 and an fps line
```

### Request 9: what holds DXMT's presents to a fixed rate? (3 minutes)

(Batch 1 as committed: the program's own count was 120.0 a second, the HUD's 60.0. The test below stands; read "60" as "the fixed rate".) In batch 1's first `pair` pass the Direct3D 11 program returned from `Present` at a fixed rate with sync interval 0, while the same program through DXMT on the sandbox (HUD off) ran at 97 to 210, and native Metal on the M3 ran at 133 to 253. The M3 runs had the Metal HUD on; the sandbox's had it off. This separates the two:

```sh
cd ~/macron-perf
for hud in 0 1; do
  env MTL_HUD_ENABLED=$hud MTL_HUD_LOG_ENABLED=$hud SteamAppId=990004 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/gpuload-prefix \
    macron waitforexitandrun "$PWD/gpuload.exe" --mode draws --size 1280x720 --seconds 15 --sync 0 --out "Z:$HOME/macron-perf/cap-d3d-hud$hud.txt" >/dev/null 2>&1
  env MTL_HUD_ENABLED=$hud MTL_HUD_LOG_ENABLED=$hud arch -arm64 ./gpuload --mode draws --size 1280x720 --seconds 15 --sync 0 --out cap-metal-hud$hud.txt >/dev/null 2>&1
done
grep -H 'RESULT fps' cap-*.txt
```

Read the `fps` of each (the program's own clock, not the HUD's):

- `cap-d3d-hud0` well above 60 and `cap-d3d-hud1` at 60: the HUD holds DXMT's presents to the display; numbers taken with the HUD on (show-fps, batch 1) are capped at 60 for any game faster than that, and the ceiling is not something players without the HUD have.
- `cap-d3d-hud0` also at 60: something in the DXMT/Wine window path holds every DXMT game in a window to the display's rate whatever the game asks for. Then it is worth a fix, and the next request will look for where.
- The two `cap-metal` lines are the reference: native Metal with the same settings.

### Request 10: DXMT against native Metal, both at the display's rate (about 15 minutes)

```sh
cd ~/macron-perf/src && tools/perf/bench.sh pair m3sync 1600x900
```

The same as request 8, but both sides now present with sync interval 1 (`SYNC=1`, the default now), so both run at 60 a second and the HUD's GPU time a frame is one rendered frame's on both. Read, for each load, the median `GPU time ms` of `d3d11` against `metal-x86_64`: the difference is what DXMT adds to the same work on the GPU (its presenting pass included). Within a few percent closes candidate 5; a large difference on `fill` points at DXMT's shaders or passes, on `draws` at its per-draw work. If a load's GPU time is above 16 ms on either side, that side cannot keep 60 and the comparison for that load is per shown frame again; say which.

### Request 11: Metal System Traces (about 10 minutes)

`xctrace` is on the Mac (`xctrace version 16.0`), so the GPU's own record of each pass can be had. If attaching asks for a password or a Developer Tools permission, stop this request and say so. Three 5-second traces: `gpuload.exe` through DXMT, native `gpuload`, and Superposition in the middle of its run (window active and floating).

```sh
cd ~/macron-perf
trace() {   # trace <name> <pid>
  xcrun xctrace record --template 'Metal System Trace' --time-limit 5s --attach "$2" --output "trace-$1.trace" >/dev/null
  xcrun xctrace export --input "trace-$1.trace" --toc > "trace-$1-toc.xml"
}
# 1. Direct3D 11 through DXMT (start it, wait 5 s, trace the Windows program's own process):
SteamAppId=990004 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/gpuload-prefix macron waitforexitandrun "$PWD/gpuload.exe" --mode fill --size 1600x900 --seconds 25 --sync 1 --out "Z:$HOME/macron-perf/tr-d3d.txt" >/dev/null 2>&1 &
sleep 8; trace d3d "$(pgrep -f '^Z:.*gpuload.exe' | head -1)"; wait
# 2. native:
arch -x86_64 ./gpuload --mode fill --size 1600x900 --seconds 25 --sync 1 --out tr-metal.txt >/dev/null 2>&1 &
sleep 8; trace metal "$(pgrep -f '/gpuload --mode' | head -1)"; wait
# 3. Superposition: start `tools/perf/bench.sh run superposition tr-sp 1600x900 windowed` as usual, and about a minute into the run:
#    trace sp "$(pgrep -f '^C:.*superposition.exe' | head -1)"
```

Then, for each trace, export the tables about GPU work: list the schemas with `grep -o 'schema="[^"]*"' trace-<name>-toc.xml | sort -u`, and export every one whose name contains `gpu` or `encoder`:

```sh
for name in d3d metal sp; do
  for schema in $(grep -o 'schema="[^"]*"' trace-$name-toc.xml | sort -u | sed 's/schema="//;s/"//' | grep -E 'gpu|encoder'); do
    xcrun xctrace export --input trace-$name.trace --xpath "/trace-toc/run[@number=\"1\"]/data/table[@schema=\"$schema\"]" | sed -E 's|/Users/[^/ <"]+|~|g' | gzip > trace-$name-$schema.xml.gz
  done
done
```

Push the `.xml.gz` files and the three `-toc.xml` files (home folder replaced the same way) to `docs/perf/m3-batch2/`; the `.trace` bundles stay on the Mac. They show every GPU pass with its duration and label: what DXMT's frame is made of next to the native one's, and in Superposition whether DXMT's own passes (the present copy, clears, its stream-output emulation) take a share worth chasing.

### Request 12: what heat costs, with a load that never changes (6 minutes plus cooling)

Batch 1's runs reached thermal level 2 within about a minute, and settings were compared across runs with different thermal histories. This measures the effect itself: the same GPU work every frame through DXMT for five minutes, presented at the display's rate, from a cool start.

```sh
until [ "$(notifyutil -g com.apple.system.thermalpressurelevel | awk '{print $2}')" = 0 ]; do sleep 10; done
mkdir -p ~/macron-perf/results/soak && cd ~/macron-perf/results/soak
(while :; do printf '%s %s\n' "$(date +%H:%M:%S)" "$(notifyutil -g com.apple.system.thermalpressurelevel | awk '{print $2}')"; sleep 1; done) > thermal.txt & logger=$!
echo "gpuload fill 12 passes, d3d11, 1600x900, sync 1, 300 s" > env.txt
env MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1 SteamAppId=990004 STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/gpuload-prefix \
  macron waitforexitandrun "$HOME/macron-perf/gpuload.exe" --mode fill --passes 12 --size 1600x900 --seconds 300 --sync 1 \
  --out "Z:$HOME/macron-perf/results/soak/bench.txt" > macron.log 2>&1
kill $logger
cd ~/macron-perf/src && tools/perf/bench.sh summary soak
```

Push `summary.txt`, `hud.csv`, `thermal.txt`, `bench.txt` and `env.txt` of `soak`. What it shows: the work is fixed and at 60 frames a second each frame should finish before the next starts, so the HUD's GPU time a frame tracks how fast the GPU is running; reading it against the thermal level second by second gives how much slower the GPU gets at levels 1 and 2. If it climbs past 16.7 ms the frame rate drops below 60 at level 2 with the same work, which is what a player sees after a minute or two. (I do the analysis from the files.)

### Request 13: three raw HUD lines (no run needed)

From any batch 2 `macron.log` (the soak's is fine):

```sh
grep -m3 'metal-HUD' ~/macron-perf/results/soak/macron.log | cut -c1-600
```

Paste them into the results. Every frame record appears twice in a row in what `bench.sh` parsed from batch 1 (50 to 56% of rows); the raw lines say whether the HUD writes it twice or the parser reads it twice.

### Request 14: MetalFX spatial upscaling, in alternating order (about 30 minutes)

Batch 1's `mfx-off` and `mfx-on-2` (both 800x450 rendered) ran at different temperatures. Same pair, ABBA, window active and floating as before:

```sh
cd ~/macron-perf/src
tools/perf/bench.sh run superposition abba-off-1 800x450 windowed
tools/perf/bench.sh run superposition abba-on-1 800x450 windowed DXMT_METALFX_SPATIAL_SWAPCHAIN=1
tools/perf/bench.sh run superposition abba-on-2 800x450 windowed DXMT_METALFX_SPATIAL_SWAPCHAIN=1
tools/perf/bench.sh run superposition abba-off-2 800x450 windowed
```

Read the benchmark's own FPS and the HUD's GPU time median of the two `on` runs against the two `off` runs. If the `on` runs are not slower, the upscale is close to free on the M3, and a game rendered at half the width and height and upscaled by DXMT is a cheap way to a sharp full-size picture. Also take one screenshot of the window in an `on` run and one in an `off` run (`screencapture -l <window id>`, the benchmark's window only), so the picture can be judged. If `screencapture` asks for a permission, skip the screenshots and say so.

### Request 15: which Wine server requests Overwatch makes on its menu (about 5 minutes; his account, nothing clicked, the same start as request 6's start 1)

On the menu `wineserver` used half a core next to the game's one (start 1). Which requests those are decides what a fix looks like: msync replaces waits and event, semaphore and mutex operations, and nothing else. This counts them by type for 5 seconds of the menu. It is start 1 again with Wine's server trace on, so the game runs slower while traced; the mix of requests is what matters, not the rate.

1. Start Overwatch exactly as for start 1 (`MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1`, no `show-fps` file), adding `WINEDEBUG=-all,+server` in front of the command. No Wine process of that prefix may be running before (the trace is on only for a server started by this launch).
2. When the menu has been up for about a minute (the HUD's frame rate level, or 120 s after the start), in another terminal:

   ```sh
   cd ~/macron-perf/src && git pull
   L=$(ls -t ~/Library/Application\ Support/Macron/logs/steam-2357570/*.log | head -1)
   tools/perf/server-count.sh "$L" 5 ~/macron-perf/results/ow-server
   ```
3. Quit the game as before, then delete the launch log: with the trace it can be several GB (`rm "$L"`).

Push `~/macron-perf/results/ow-server/` (`window.txt`, `requests.txt`, `threads.txt`, `thread-requests.txt`, `selects.txt`: request names, thread ids and counts only, no arguments) to `docs/perf/m3-batch2/ow-server/`. If the game does not reach its menu with the trace on (too slow, or it quits), say so and how far it got. Read: if `select` (waits) and `set_event`, `release_semaphore`, `release_mutex`, `event_op` lead the list, the server's half core is synchronisation and msync's ground; if window, message-queue, input or file requests lead (`get_message`, `send_message`, `get_window_*`, `set_cursor`, `get_thread_input`...), msync would not touch it and the fix is elsewhere.

### What to push for batch 2

`docs/perf/real-mac-results.md`: request 9's four `RESULT fps` lines; request 10's table and each run's `bench.txt`; request 11's list of exported files and anything that failed; request 12's summary; request 13's three lines; request 14's four summaries; request 15's files. Files into `docs/perf/m3-batch2/` as above (each run's `summary.txt`, `env.txt`, `hud.csv`, `thermal.txt`, `bench.txt`, the trace exports, the two screenshots). Suggested order: 13 needs a log, so 12 first; then 9, 10, 11, 14.

## Batch 3 (2026-11-16, after batch 2)

### Request 16: what Overwatch's threads do on its menu (about 5 minutes; his account, nothing clicked, the same start as request 6's start 1)

Request 15 found one thread polling the Wine server about 40,000 times a second (zero-timeout waits), about 6 ms of round trips a frame at 54 frames a second. This shows whether that thread is the one holding the frame: macOS's `sample` of the game's process for 5 seconds, with no trace on.

1. Start Overwatch exactly as for start 1 (`MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1`, no `show-fps` file, no `WINEDEBUG`).
2. When the menu has been up for about a minute, in another terminal:

   ```sh
   pid=$(ps -axo pid=,%cpu=,command= | grep -i 'overwatch\.exe' | grep -v -e grep -e 'steam\.exe' -e macron | sort -k2 -rn | head -1 | awk '{print $1}')
   ps -o pid=,%cpu=,command= -p "$pid" | cut -c1-80
   sample "$pid" 5 2 -mayDie -file ~/macron-perf/ow-sample.txt
   sed -E 's|/Users/[^/ ]+|~|g' ~/macron-perf/ow-sample.txt | gzip > ~/macron-perf/ow-sample.txt.gz
   ```
3. Quit the game as before.

Push `ow-sample.txt.gz` to `docs/perf/m3-batch3/` and the `ps` line (its CPU share) into the results. If `sample` asks for a permission or a password, stop and say so. Read (I do the analysis): for each thread, its number of samples and how many sit under Wine's server call (`server_call`, `wait_select_reply`, `read` inside `ntdll.so`). If the busiest thread spends a large share there, the polls are on the frame's path and answering them in the game's process (docs/perf/README.md, candidate 6) would raise the menu's frame rate; if the busy threads are elsewhere and the poller mostly waits, it would mostly save the server's half core.

### Request 17: what kind of object the polling thread polls (about 5 minutes; same start as request 15)

A fix that answers zero-timeout polls in the game's own process has to know the polled objects' state, and what it takes depends on the kind of object (an event or semaphore is simple; a thread, process or timer handle is more). Request 15 kept no arguments, so this finds the kind.

1. Start Overwatch exactly as for request 15 (`WINEDEBUG=-all,+server` added in front, no Wine process of that prefix running before).
2. When the menu has been up for about a minute:

   ```sh
   cd ~/macron-perf/src && git pull
   L=$(ls -t ~/Library/Application\ Support/Macron/logs/steam-2357570/*.log | head -1)
   tools/perf/server-handles.sh "$L" 5 ~/macron-perf/results/ow-handles
   ```
3. Quit the game, then `rm "$L"` (several GB).

Push `~/macron-perf/results/ow-handles/handles.txt` (a thread id, handle numbers, counts and request names only) to `docs/perf/m3-batch3/ow-handles/`. Read: the request that made each polled handle names its kind (`create_event`, `create_semaphore`, `create_mutex`, `open_*`, `new_thread`, `create_timer`...). `dup_handle` means it was copied from another handle and the kind is not shown; say so if that is what comes out.

## Ultimate Custom Night (gate 1 thread, Opus 5.5)

Context: `docs/proof/gate1-m3/macron9.md`. With `macron.9` the menu runs at 31 fps; the game's thread works about 19 ms a frame (13 of them in Wine) and waits for its frame timer for the rest. The menu is the benchmark (no input needed). Same setup as macron9.md: `macron` built from main, its runtime in a separate `MACRON_HOME`, empty workspace, sound off. Results go to `docs/proof/gate1-m3/` with the home folder written `~` (`sed -E 's|/Users/[^/ ]+|~|g'`).

### U1: the raw sample, for function-level attribution (2 minutes, or none if the macron9.md run's files were kept)

Commit, scrubbed as above, to `docs/proof/gate1-m3/macron9-raw/`:
- the raw `sample` output of the game's thread in the menu, default configuration (`sample <pid> 8 2 -file ...`; the whole file, with the `???` frames and their addresses);
- from the same start, the `+loaddll` lines (`WINEDEBUG=-all,+loaddll`; only the `Loaded L"..." at <address>` lines).

What it means: the overlay's `wined3d.dll` and `opengl32.dll` carry DWARF, and `tools/clickteam-games/sample-functions.py` resolves the sample's x86 frames with them (the sandboxes cannot: their `sample` does not unwind into 32-bit x86 frames under Rosetta). With the load addresses I resolve every x86 frame, the 10.2% "no named frame" included, to a function and a line, and the native frames under `wow64_flush_batch` name Apple's OpenGL calls. That says which of these is the 13 ms: d3d9's copy of the system memory vertex buffer before each draw, the index streaming, the maps that go through wined3d's command stream (four a draw for this game), state application, the batch replay, or the map and unmap in each unsynchronized write.

### U2: the menu without 0007's batching (2 minutes)

The same start with `WINE_OPENGL_BATCH=0` in the environment: the frame rate over a minute (median of the `trace:fps` readings), and, if easy, the same 8 s `sample` as U1.

What it means: on the M3 a transition into Wine's Unix side costs 0.6 to 0.9 µs, and 0007 copies every queued call and replays it through the WoW64 thunks. If the menu is as fast or faster without batching, the replay costs more than the transitions it saves there, and the next change takes work out of the replay. If it is clearly slower, batching earns its keep on real hardware, and what is left to queue is worth queuing.

### U3: Wine patch 0010, the menu with the command stream on the game's thread and on its own (about 10 minutes)

What changed (`runtime/wine/patches/README.md`, 0010): from U1's sample, the game's three maps per draw of its system memory buffers went through wined3d's command stream (a round trip each, and with `csmt=0x1` a wait for everything queued, hence that run's 23% of yielding and waking); they are now done on the game's thread. And each index upload rebound the vertex array's element buffer, which Apple's OpenGL rebuilt at the next draw (`buildVertexArrayState`); uploads now go through `GL_COPY_WRITE_BUFFER`. On the sandbox the pictures still match, and with `csmt=0x1` the probe in the game's drawing mode went from 1.3 to 6 fps; the single-thread change is below the sandbox's noise, so the M3 decides.

Build `macron` from branch `wine-0010` (it pins the test overlay `runtime-wine-11.21-macron.10`) and install its runtime into a new `MACRON_HOME`, as for macron9.md:

```sh
git -C ~/macron-perf/src fetch origin wine-0010 && git -C ~/macron-perf/src checkout FETCH_HEAD
cd ~/macron-perf/src && go build -o ~/macron-perf/macron-0010 ./cmd/macron
MACRON_HOME=~/macron-perf/home-0010 ~/macron-perf/macron-0010 runtime install
```

Then the menu, the way macron9.md started it, three times:
1. default: the frame rate over a minute (with `show-fps` in that `MACRON_HOME`), then a start with `WINEDEBUG=-all,+loaddll` and the same 8 s `sample` as U1;
2. `WINE_D3D_CONFIG=csmt=0x1`: the frame rate over a minute, and one `sample` of the whole process (both the game's thread and `wined3d_cs`) with its `+loaddll` lines;
3. a screenshot of the game's window in each, to compare by eye with the 0.4.0 one (an upload landing in the wrong place shows as broken sprites).

Commit to `docs/proof/gate1-m3/macron10/`, scrubbed as before. What the results mean:
- **60 fps in 1 or 2**: the menu meets the bar. If only in 2, the next step is making the command stream's own thread the default for 32-bit programs again (0004 turned it off for crashes that 0006 has since fixed), after stability runs on the sandboxes.
- **Still about 31 in 1**: the share of the game's thread waiting in `NtUserMsgWaitForMultipleObjectsEx` against U1's 40.7% says how much work 0010 removed; the sample says where the rest is.
- **Still about 30 in 2**: the two threads' samples say what they still wait for.
- **Broken sprites, or a crash**: `WINE_D3D_CONFIG` unset and `WINE_OPENGL_UNSYNCHRONIZED=0` tell 0009 from 0010; say which.

### U4: Wine patch 0011, the command stream on its own thread by default (about 10 minutes)

What changed (`runtime/wine/patches/README.md`, 0011): 0004 had put a 32-bit program's command stream on the game's thread; 0011 reverts that, so `macron.11` does by default what U3's `WINE_D3D_CONFIG=csmt=0x1` did. Stability on a sandbox with `macron.10` and `csmt=0x1`: 10 of 10 starts of Ultimate Custom Night itself (through the Steam bridge, as Steam starts it) alive 40 s after their window with no fault, 20 of 20 of the probe, and `d3d9-sysmem-probe` passing with it.

Build `macron` from branch `wine-0011` (it pins the test overlay `runtime-wine-11.21-macron.11`) into a new `MACRON_HOME`, as for U3:

```sh
git -C ~/macron-perf/src fetch origin wine-0011 && git -C ~/macron-perf/src checkout FETCH_HEAD
cd ~/macron-perf/src && go build -o ~/macron-perf/macron-0011 ./cmd/macron
MACRON_HOME=~/macron-perf/home-0011 ~/macron-perf/macron-0011 runtime install
```

Then the menu, three starts:
1. no extra environment at all: the frame rate over two minutes (with `show-fps`), the game's CPU, a screenshot;
2. the same with `WINE_D3D_CONFIG=csmt=0`, a minute: the switch back to one thread, for anyone a game misbehaves for;
3. one more start with no extra environment, `WINEDEBUG=-all,+loaddll` and an 8 s `sample` of the whole process, taken while the frame counter of start 1 showed a dip if you can catch one (otherwise any 8 s).

Commit to `docs/proof/gate1-m3/macron11/`, scrubbed as before. What the results mean:
- **About 60 in 1 and about 31 in 2**: 0011 does what U3 showed, and the switch back works. Then `wine-0011` goes to main and into v0.5.0-rc2 for Thalles (tested on a fresh sandbox first), and the top of PROGRESS.md says to install that.
- **Below U3's 60 in 1**: something differs between `csmt=0x1` set by hand and the default; the sample says what.
- **Broken sprites, a crash, or a hang on quit**: say which start; `WINE_D3D_CONFIG=csmt=0` is the way back.
- The sample of start 3: the game's thread spent about 10% in `cthread_yield` and `NtDelayExecution` in U3's csmt sample; whether the dips line up with that, or with the command stream thread being busy (it waited for work 49% of the time on average), says what to cut next. A night (1.7 times the draws) is still unmeasured; U3's numbers put each thread at about 14 ms of a 16.7 ms frame there, so expect it to be close.

## Overwatch's memory (Overwatch memory thread, Opus 5.5)

Context: `docs/overwatch-memory.md`. On the M3 with 0.4.0, Overwatch's process held 15 GB, about 12 GB of it macOS `malloc`, and the HUD's memory figure was 13.7 to 14.3 GB on the menu (request 6, start 1). On sandboxes the same `malloc` is DXMT keeping every shader the game creates in its parsed form (82,706 shaders, 7.9 GB of 8.0 GB of `malloc`); DXMT patch 0001 parses a shader again on its first compile instead, and took the process from 7.9 GB to 1.8 GB at the Battle.net sign-in screen there. Whether the M3's 12 GB is the same thing has not been measured on the M3.

### M1: Overwatch left alone for three minutes with the DXMT patch (about 15 minutes; his account, nothing clicked in the game)

Build `macron` from branch `dxmt-m1` (commit `66cf62bd01dd7dad4a84cd2500ed05588611c415`: main at `acf610a` plus the pin of the DXMT overlay `runtime-dxmt-0.80-macron.1`; Wine is `macron.11`) and install its runtime into a new `MACRON_HOME`, as for U3:

```sh
git -C ~/macron-perf/src fetch origin dxmt-m1 && git -C ~/macron-perf/src checkout FETCH_HEAD
cd ~/macron-perf/src && go build -o ~/macron-perf/macron-m1 ./cmd/macron
MACRON_HOME=~/macron-perf/home-m1 ~/macron-perf/macron-m1 runtime install
shasum -a 256 ~/macron-perf/home-m1/runtimes/*/wine-dxmt/lib/wine/x86_64-windows/d3d11.dll
# must print: 68b71e3cd730e5e5153f9a114d055a045295dbbbe022b5ad4cfe3eefdac54dd4
```

So that his own Overwatch prefix is not touched by another runtime, start the game on an APFS clone of it (no extra disk; Overwatch's own files stay where Steam put them):

```sh
compat="$HOME/Library/Application Support/Steam/steamapps/compatdata/2357570"
cp -cR "$compat" ~/macron-perf/ow-compat-m1
```

Then the start of request 6's start 1, with `MACRON_HOME=~/macron-perf/home-m1`, `STEAM_COMPAT_DATA_PATH=$HOME/macron-perf/ow-compat-m1`, `~/macron-perf/macron-m1` as the `macron`, `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1` and no `show-fps` file in that `MACRON_HOME`, Steam running. Just before it, in another terminal, start the memory follower from `tools/overwatch-memory/` (on main): it finds the game's process by itself, writes one line every 3 s (`t`, footprint MB, swap MB, memory pressure level, DXMT's shader cache MB) to `mon.tsv`, and a full `footprint` and `vmmap -summary` every 30 s:

```sh
cd ~/macron-perf/src && tools/overwatch-memory/ow-mon.sh ~/macron-perf/m1 200 &
```

Leave the game alone for three minutes after its menu shows, then, before quitting it:

```sh
pid=$(awk 'NR == 1 { print $2 }' ~/macron-perf/m1/pid)
heap -s -H "$pid" | head -80 > ~/macron-perf/m1/heap.txt
```

Quit the game the way request 6 did. Commit to `docs/proof/overwatch-memory/m3-m1/`, home folder written `~` (`sed -E 's|/Users/[^/ ]+|~|g'`): `mon.tsv`, `pid`, every `fp-*.txt` and `vm-*.txt`, `heap.txt`, and the `bench.sh summary` of the launch log as in request 6 (`summary.txt`, `hud.csv`) with the HUD's memory figure every 20 s written into the results as request 6's were. No screenshot (it shows his account).

Optional, a second start (5 more minutes), to tell the DXMT patch from everything else that changed since 0.4.0: the same from commit `acf610a` (main without the pin, the same Wine) into `~/macron-perf/home-m1-control`, on a fresh clone of the compatdata (`ow-compat-m1-control`), its follower writing to `~/macron-perf/m1-control`, committed to `m3-m1-control/`.

What the results mean (the M3 with 0.4.0: HUD 13.7 to 14.3 GB on the menu, `footprint` 15 GB with `MALLOC_SMALL` 10 GB):

- **The footprint and the HUD figure level near 2 to 4 GB, and `MALLOC_SMALL` is gone from `fp-*.txt`'s top categories:** the M3's 12 GB was the parsed shaders, as on the sandboxes, and the patch removes it there too. The swapping and the hitches that came with it should then be gone; the DXMT overlay rides in the next release candidate (PROGRESS.md says which).
- **Level well below 14 GB but well above 4 GB:** the patch removes part of it; `heap.txt` (the largest `malloc` users by class and binary) and the `fp-*.txt` categories say what the rest is, and that is the next thing to look at.
- **Still about 14 GB:** first check the `shasum` above (the patched `d3d11.dll` must be the one installed); if it is, the M3's `malloc` is something else than on the sandboxes, and `heap.txt` says what. If the control start was run and shows the same, the patch does nothing there.
- **In every case:** the HUD's frames a second and median GPU time on the menu against request 6's 53.5 fps and 7.8 ms (with `macron.11` in both builds, the control start says whether any difference is the patch's). A shader is now parsed again before its first compile; a lower frame rate or more frames over 50 ms in the first minute would be that cost.
- **A crash or a black screen:** say when; on the sandboxes one of six patched starts crashed early (`page fault on execute access` inside Overwatch.exe), and none of six with DXMT's own file, which is too few to tell.
