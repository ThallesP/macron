# v0.5.0-rc6, tested before publishing

- `sbx_cy4n44rd/`: the published file, `Macron-0.5.0-rc6.dmg` of Release run 44357997039 (main at 9f1eaab, SHA-256 `52b96307283459b67b27be1452f86820179de5a9ed343eb9c7301e861b892b46`). `gate3.sh` end to end on a fresh sandbox, then the package installed again and, through `/usr/local/bin/macron`: the runtime it installs, the `dlss` switch's signature check, a folder's change notification, the server's thread with and without `MACRON_WINESERVER_PRIORITY=normal`, and `sndload` registered and not (`after-gate3.txt`).
- `sbx_u6bwibf5/`: an earlier candidate of the same version that was never published (Release run 44328579476, main at 1b1913c, SHA-256 `ad22bdff...`, with Wine overlay `macron.14`). Kept for what it showed about Steam's inserted libraries (`after-gate3.txt`).

A sandbox runs everything started over ssh at priority 20, so the server's thread shows as `20R` there (fixed priority at the clamp) and its effect on wake-ups cannot be seen; on the M3 it is `63R` (runtime/wine/patches/README.md, "0015").
