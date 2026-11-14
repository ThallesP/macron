# Goal

Macron is Proton for Mac: a Mac user presses Play on a Windows-only game in the Mac Steam client and it runs. README.md has the architecture.

This loop builds Macron's runtime and launcher against a real feedback loop: launch a real Windows game through Macron, get it to its main menu, and compare a screenshot with how the original game looks.

## The brief

Thalles, 2026-11-14, lightly cleaned up from dictation:

> Now that we got the CLI out of the way, I think we have everything to start.
>
> I want ways of verifying the work, so I have a feedback loop. I think the feedback loop could be launching a small-to-medium game. I know there's a limit on the GitHub Actions size, but maybe we could get the maximum we can have, or work around it if it's a problem. I'm just looking for a game. I'll leave this MacBook running in the meantime, so if you want to test some stuff out on it, feel free.
>
> For the feedback loop, try to launch a Windows game and see how far we can get, maybe up to the game's menu. Then see if you can compare against the original game screenshots.
>
> Feel free to write tests, and to use Ultracode if necessary, though we should be a little careful about it because of usage limits. Feel free to use sub-agents to get this done. Do not use Fable for this, use Opus and just let it rip. Grind until done, I won't be here to help you.

## Done means

Gates, in order. Each one stays green once reached. The goal is complete when gates 1 to 4 are green in CI on `main` and gate 5 is either green or parked as described there.

1. **The loop runs.** One command, run from the dev machine, takes a scenario name and does the whole check: starts a Mac sandbox, installs Macron's runtime, launches the scenario's Windows program, waits until the scenario's "reached" condition holds, takes a screenshot, compares it with the committed reference, saves the evidence (screenshots, logs, timings), stops the sandbox, and exits non-zero on any failure. The same check runs as a CI workflow on `main`. The first scenario is the Direct3D 11 probe from the `ci-probe` branch, drawing into a visible window.

2. **A real game at its menu.** A scenario for a real Windows game passes: the game reaches its main menu and the screenshot check against the original passes.
   - The game is free to download legally with no account and no payment (freeware, an official demo, open source), from a pinned URL with a pinned checksum. Thalles asked for small-to-medium, so prefer a real game over a tech demo.
   - "The original" is the same build running on real Windows wherever that can be captured. GitHub's Windows runners are real Windows with a software Direct3D renderer, so try them first. If a game can't be captured there, use an official screenshot or a frame of the developer's own footage, and record the source.
   - The comparison can't be pixel-exact across GPUs. Define it before looking at Macron's output: a fixed window size, a wait condition, a similarity measure with a threshold, and a check that the menu's text is there. Prove it with negative controls that must fail: a black frame, the desktop without the game, another game's menu, and a deliberately broken render.

3. **Through Macron, not scripts.** The scenarios launch through the product: a headless `macron` launcher that Steam will later call. It installs and pins a versioned runtime (Wine plus the Direct3D-to-Metal layers), creates a private Windows environment per game, applies that game's profile from a fixes database (graphics backend, environment, DLL overrides, extra installs, launch arguments), starts the game, and reports what happened through logs and exit codes. Unit tests run on Linux; the scenarios are the integration tests.

4. **Three games.** Three games from different engines pass, at least one of them 1 GB or larger. README.md has a table of every game tried: engine, graphics API, the backend that worked, how far it gets (starts, menu, in game), where it was checked (sandbox or MacBook), and what failed for the ones that don't work.

5. **Steam's Play button, as far as it can be checked without an account.** The hook that switches on Steam Play in the Mac Steam client and registers Macron as a compatibility tool. Without a Steam login, show at least this in CI: the hook applies to the current stable Steam build, Steam still reaches its login window afterwards, and the hook refuses cleanly on a build it doesn't recognise. Pressing Play needs a logged-in account with a game. If none has been provided, write what is built, what was checked and what is left under "Needs Thalles" in PROGRESS.md; that parks the gate.

If GitHub's runners can't run a game because of their virtual GPU (it has no Metal 3), a pass on the MacBook with its evidence committed counts for that game. Show the evidence that the runner was the problem, and keep every game that does run on the runners in CI.

## Rules

- **PROGRESS.md is the loop's memory.** Read it first. Update it last: what was done, the evidence, what's next, what's blocked. Keep README.md true.
- **Thalles is away.** Don't stop to ask. A decision only he can make goes under "Needs Thalles" in PROGRESS.md, and you work on something else meanwhile.
- **Never weaken a check to make it pass.** No raised thresholds, no dropped negative controls, no references regenerated from Macron's own output, no scenario marked as passing on partial evidence. Changing a check is fine only when the check itself was wrong; say why in the commit and in PROGRESS.md.
- **Models and usage.** Subagents run on Opus 5.5, never Fable. Run a few at a time, each with its own git worktree and a narrow brief. Use a workflow or Ultracode only when a problem clearly needs many parallel attempts, and note why in PROGRESS.md. On a usage-limit error: stop launching agents, write the state to PROGRESS.md, wait for the reset, carry on.
- **Games.** Legal free downloads only. No cracks, no DRM bypass, no accounts. Never commit game files or upload them as artifacts; download at test time and cache. A menu screenshot as a test reference is fine.
- **Licences.** Record each dependency's licence in README.md. Changes to Wine are LGPL and get published. Don't copy code from GPL projects (NotProton, Proton-ish, Whisky) until Thalles picks a licence for Macron; reading them to learn how something works is fine.
- **The repository is public.** Logs, artifacts and caches are world-readable and anyone can open a pull request. Never print a secret. No workflow may run code from a fork with access to anything: no `pull_request_target`, no self-hosted runners.
- **Sandboxes.** At most 5 Mac jobs run at once. Stop every sandbox when its work is done and leave no runs in progress. The tunnel sometimes stalls (see README.md), so run long jobs detached on the sandbox and poll them, and let CI workflows be the standing check.
- **Every step ends green and pushed.** Tests pass, it's committed and pushed to `main`, CI passes. Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## The MacBook

The dev machine has a T3 link to "Thalles's MacBook Air" (environment `87511d05-cf0b-4b25-9d13-83efe54da30d`, access `auto`). It is the only machine here with a real Apple GPU (M3, 16 GB, macOS 26.5). Reach it with `delegate_task` and `target.environmentId`. Thalles offered it for testing; treat it as a guest would.

- **It can go offline at any time.** Nothing may depend on it except the fallback in "Done means".
- **About 5 GB of disk was free on 2026-11-14.** Check `df` before every download, keep Macron's footprint under 3 GB in one folder (`~/macron-test`), and remove it when a test is done. Never delete anything you didn't create.
- **The screen was locked.** `screencapture` returned the lock screen, so screenshots of a game have to come from inside the game's process, or the MacBook can only show that a game runs and presents frames.
- **Hands off Thalles's things.** Don't touch `/Applications/Steam.app`, `~/Library/Application Support/Steam`, his Steam account, his browser or his files. Don't change system settings. Kill what you start (`wineserver -k`) and leave no processes behind.
- A `caffeinate -s` was started on 2026-11-14 to keep it awake on AC power for 72 hours. Start another if it has ended and you still need the machine.

## When stuck

- If a check fails for a reason outside Macron (a flaky runner, a tool bug, a download that moved), fix the tool, write it down in PROGRESS.md, and carry on.
- If a game won't run after a fair attempt across the backends, record what failed in the README table and try another game. The table of failures is useful too.
- If a decision only Thalles can make blocks the way, add it under "Needs Thalles" and work on another gate.
