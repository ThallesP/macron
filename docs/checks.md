# How a scenario is checked

A scenario (`scenarios/<name>/scenario.json`) is one Windows program, how to get it, how to know it reached its menu, and a reference picture of that menu taken on real Windows. `macron-check` runs it.

## The run on a Mac

1. Download each of the program's `downloads` from its pinned URL, check its SHA-256 and unpack it `into` its folder (or build it, for programs that live in the repo).
2. Move the mouse pointer to the screen's top-left corner (some games draw their own pointer and tooltips), then screenshot the desktop before anything starts. This is one of the negative controls.
3. `macron runtime install`, then `macron launch --game <profile> --cwd <cwd> -- <exe> <args>`. The scenario's `args` are test setup (a fixed window size, answers to a first-start dialog), passed like Steam launch options; the profile holds what any player of the game needs. The game goes through the launcher, its profile in `fixes/games/` and its own Windows environment, the same way Steam will start it.
4. Wait until the scenario's "reached" condition holds: a window whose title matches `window.title` is on screen and, if set, the game has printed `reached.output`. Then wait `reached.settleSeconds`. Fail if the game exits first or `reached.timeoutSeconds` passes.
5. Capture the window with `screencapture -l`, drop the macOS title bar, and keep the `window.width` x `window.height` picture (scaled on Retina screens). A window of another width fails.
6. Read the text in the picture with macOS's Vision framework. Every string in `compare.text` must be there (case and spacing ignored).
7. Compare the picture with the reference and run the negative controls (below). Save everything to the evidence folder: `frame.png`, `window.png`, `screen.png`, `desktop-before.png`, `game.log`, `runtime-install.log`, `windows.json`, `report.json`, `summary.md`.

## The measure

Defined in `internal/imagecheck` before any Macron output was looked at, and the same for every scenario:

- Both pictures are scaled to 320x240 by averaging. Regions listed in `compare.ignore` (fractions of the picture) are painted grey in both. Use them only for parts that move by themselves on real Windows (an animated background), shown by the reference capture's own 5-second stability score.
- **SSIM** on luminance, 8x8 windows every 4 pixels, averaged over the windows where either picture has structure (standard deviation of at least 6 grey levels). Flat areas are left out because they agree with almost anything and would hide a broken frame. Must be **at least 0.70**.
- **Colour distance**: both pictures scaled to 16x12, mean absolute difference per channel, as a fraction of 255. Must be **at most 0.10**.
- **Menu text**: every string in `compare.text` is found by OCR.

The thresholds are constants in the code. A scenario cannot loosen them.

## Negative controls

Each run also compares these with the reference. Each must fail the picture measure on its own (without the text check), or the run fails, because then the measure could not tell a broken result from a good one:

- a black frame,
- the reference with its colours inverted,
- the reference upside down,
- the reference shifted right by a fifth of its width (a misplaced viewport),
- the desktop captured before the game started,
- another game's reference (`otherGame`), once there is another game.

The unit tests in `internal/imagecheck` and `internal/check` check the same things on synthetic pictures, including that the same picture with a one-pixel offset, softer filtering, noise and slightly different colours still passes.

## References

The `Reference` workflow runs `macron-check reference <scenario>` on a GitHub Windows runner: the original program, natively, with the same profile files and arguments, the pointer parked in the same corner, waiting for the same condition, capturing the window's client area with `BitBlt`. It captures three times (at the reached condition, 5 seconds later and 30 seconds later) and reports how similar the later ones are to the first, with the scenario's ignore regions applied. Those scores are the evidence for an ignore region: a part of the picture may be ignored only if it changes by itself on real Windows. The picture is committed as `scenarios/<name>/reference.png` with its source (run URL) in `reference.source`. A reference is never made from Macron's output.
