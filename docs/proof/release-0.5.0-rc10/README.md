# v0.5.0-rc10, tested before publishing

`sbx_ll7a6jqt/`: the published file, `Macron-0.5.0-rc10.dmg` of Release run 44863474569 (main at 5390ace, SHA-256 `781036dc32f570502730c18b150c186d12f1e6f89059667d7bfefbf3bfb8cd88`), on a fresh sandbox.

- `gate3-all.log` and the `gate3-*` files: `ci/steam-play/gate3.sh` end to end, the same steps as rc9's, passed. Its screenshots stay on the sandbox's side (they show the test account's library).
- `after-gate3.txt` and `after-gate3-server-stats.txt`: the package installed again. The runtime it installs is `wine-11.21-macron.19-...`; `wineserver` in both Wine trees is SHA-256 `5f3a995d...8ffc`, ad hoc signed, **the file Thalles's M3 ran byte for byte** (`runtime/wine/patches/README.md`, "0021": the server's turn at the registry save 14 to 16 ms where it had been 63 to 152 ms, and no sound missing). Then, through `/usr/local/bin/macron` with `MACRON_SERVER_STATS` set, a new prefix and a stream fed as SDL does for 50 s:
  - the server writes its log (twelve five-second lines and its slow turns), so 0020 is in and on;
  - `system.reg` and `user.reg` are rewritten 30 s after the start (3,301,821 to 3,306,201 bytes and 67,397 to 69,097), by the child, with no `reg*.tmp` file and no zombie left; `system.reg` has the 83,111 lines it has without the patch and begins `WINE REGISTRY Version 2`; a second start of the same prefix runs.
  - **The turn at the save took 71.8 ms here**, and 48 and 49 ms in two earlier runs on another sandbox, where the save itself had taken 79 and 96 ms: on a virtual machine `fork()` under Rosetta is nearly as slow as the write it replaces. The M3's 14 to 16 ms are a real Mac's. This sandbox's sound had gaps at other times too (3,712 frames in the third report), as sandboxes' do.

Nothing of the test account is in these files (checked against its tokens before committing).
