# Request M1: Overwatch's memory with the DXMT patch on Thalles's MacBook Air M3, 2026-11-16 09:50 to 10:02 UTC

Two starts, each on an APFS clone of his Overwatch Windows environment (his own is untouched), started the way Steam starts it with `MTL_HUD_ENABLED=1 MTL_HUD_LOG_ENABLED=1`, sound off, an empty workspace in front, nothing clicked, about four minutes each; `tools/overwatch-memory/ow-mon.sh` following the process. No screenshot (it would show his account), so the screen is whatever the game shows by itself after signing in.

| start | `d3d11.dll` (x86_64) SHA-256 | footprint after it levels | memory pressure | swap in use, start to end |
|---|---|---|---|---|
| `m3-m1`: branch `dxmt-m1` at 66cf62b (DXMT overlay `0.80-macron.1`, Wine `macron.11`) | `68b71e3cd730e5e5153f9a114d055a045295dbbbe022b5ad4cfe3eefdac54dd4` (the one the request names) | **3.40 to 3.47 GB** from 55 s on | 1 (normal) throughout | 5,990 to 5,974 MB |
| `m3-m1-control`: main at acf610a (DXMT 0.80 as released, the same Wine) | `7ca382af0eb32d8a432f6efb14d594fefb45673663be1f7e6682254bff885c47` | **14.24 to 14.25 GB** from 111 s on (11.2 GB at 37 s) | 2 (warning) from the first reading after the start | 5,966 to 7,699 MB |

`fp-192.txt` of the patched start: WINE_RESERVE 2,405 MB, graphics 554 + 102 MB, Rosetta JIT 128 MB; no MALLOC category among the first rows. The control's has them as on 0.4.0.

The HUD's own figures:

```
m1: every 20 s (clock, HUD memory MB, frames a second, median GPU ms, frames over 50 ms)
  06:51:16   3422  29.0   4.0 136
  06:51:38   3459  54.2   7.2   1
  06:51:59   3430  48.6   6.2   7
  06:52:21   3445  48.5   6.2   4
  06:52:42   3445  53.8   6.6   1
  06:53:03   3410  48.6   6.5   6
  06:53:24   3405  48.6   6.2   4
  06:53:46   3405  54.0   6.7   1
  06:54:06   3408  42.9   7.6   5
  06:54:30   3408  54.8   8.0   1
  06:54:50   3433  53.8   7.2   4
  06:55:10   3411  53.5   6.3   2
  06:55:30   3421  60.3   7.4   2
m1-control: every 20 s (clock, HUD memory MB, frames a second, median GPU ms, frames over 50 ms)
  06:57:26  12841  20.8   2.2 199
  06:57:48  13642  54.9   8.2   1
  06:58:08  13899  45.4   7.7   5
  06:58:31  14224  45.1   7.7   8
  06:58:54  14272  45.4   7.7   6
  06:59:17  14252  54.0   7.8   2
  06:59:37  14252  46.0   7.7   4
  07:00:01  14246  21.9   7.7  10
  07:00:49  14239  54.3   8.3   2
  07:01:09  14263  54.3   8.1   0
  07:01:29  14263  54.8   7.6   1
```

`bench.sh summary`: patched 50.0 frames a second, GPU busy 31%; control 43.3, GPU busy 33% (request 6's start 1 on 0.4.0: 53.5 and 40%). The two starts ran six minutes apart on a Mac that had been running benchmarks for three hours, so the difference between them is not evidence about the patch either way; the frames over 50 ms after the first 20 s are 0 to 3 per 20 s in the patched start and 0 to 5 in the control.

Neither start crashed; each ended when asked. `heap.txt` is from the patched start, taken 195 s after its launch.

Files: `mon.tsv`, `pid`, `fp-*.txt`, `vm-*.txt`, `heap.txt` (patched only), `summary.txt`, `hud.csv`, `d3d11.sha256`; the control's are in `../m3-m1-control/`.
