# Goal

Macron is Proton for Mac: a Mac user presses Play on a Windows-only game in the Mac Steam client and it runs. README.md has the architecture.

Phase 1 (2026-11-14) built the launcher, the pinned runtime, the fixes database and the scenario checks; its gates are at the end of this file and stay green.

Phase 2 is this file: make it fast, make Steam's own Install and Play buttons work, and ship something Thalles can install.

## The brief

Thalles, 2026-11-14, after trying phase 1 on his MacBook. Dictated; cleaned up, with the readings of garbled words marked:

> It's running, though super slow, and clearly we did not have the experience I wanted.
>
> Move this up to the dev machine and let it rip in there. Feel free for the dev machine to ask about anything that can only be investigated on this PC [the MacBook], but I think it makes sense to use GitHub Actions to investigate what's happening.
>
> See why it's so slow on Ultimate Custom Night. The game runs everywhere; it should run on an Intel Celeron without dropping under 40 FPS [reading of "the Tel Cellar ROM without 40 FPS"]. Investigate what's so slow and get the experience done.
>
> When it's done, get me the DMG or whatever I have to install so I can have the experience I wanted, because I didn't have that experience. You had to do some hacky workarounds under the hood to get this working.

Later the same day, after the loop's test sessions on his MacBook got in his way (a full-screen game, keystrokes sent to his desktop, a core and a half of CPU):

> ok my PC is off limits now, do whatever you need on the sandboxes. feel free to create a new steam account to test it out. its causing problem on whatever im trying to do

So **the MacBook is off limits** and everything is checked on the `sbx` sandboxes and in CI, with a Steam account made for testing.

"The experience" is his from the start: open the native Mac Steam client, press Install and then Play on a Windows-only game, and it runs. Macron is an installer that plugs into Steam with little or no UI of its own.

## What happened on the MacBook

From the hands-on session before it was withdrawn. Read this before planning; PROGRESS.md has the detail.

- Macron's registration alone does not switch on Steam Play. With `macron steam enable` applied to his logged-in Steam (build 1794536281), Steam logged `Mapping AppID 0 to tool "macron"`, but the Install button of a Windows-only game stayed greyed out and `steam://install/871720` answered **"An error occurred while installing Ultimate Custom Night: Invalid platform"**. Phase 1 parked exactly this question; it is now answered.
- The game only ran through a workaround: a logged-in Steam accepts `steam_osx +download_depot <app> <depot>` and downloads the Windows files, which were then started by hand with `SteamAppId=871720 macron waitforexitandrun`. That is the "hacky workaround"; it is not the product.
- Ultimate Custom Night (Steam app 871720, free, one 262 MB 32-bit Clickteam Fusion exe) reached its title screen and was "super slow". It loads `mmf2d3d9.dll`, so it draws with Direct3D 9; with the default profile (`dxmt`) that went to Wine's builtin `d3d9` → `wined3d` → `opengl32` (Apple's OpenGL, through Wine's 32-on-64 mode, under Rosetta). The window was full screen at 1920x1243 points on a Retina display. Wine's server in this build has no msync or esync. None of these has been measured yet; they are suspects, not a diagnosis.
- DXMT presents on a real Apple GPU with the patched `winemac.so`: the Direct3D 11 probe drew its picture on the M3 (60 frames in 529 ms) through `macron waitforexitandrun` with the default profile.

## Done means

Gates, in order of importance. The goal is complete when gates 1 to 4 are green, gate 5 is green or has had the fair attempt described there, and the phase 1 gates still are. "Green" means done on a Mac sandbox with the real Steam client logged in to the test account (see "The Steam test account"); where a sandbox cannot show something, the gate says what counts instead.

1. **Fast.** Ultimate Custom Night (Steam app 871720, free to play) runs at its own frame rate (it is a 60 fps game) through Macron, from its title screen into a night, measured and not judged by eye. The one real measurement so far is the MacBook's: about 5 frames a second on wined3d's OpenGL renderer (PROGRESS.md). The cause is found by measurement (which layer costs the time: Direct3D translation, the 32-bit mode, Rosetta, the Wine server, the window or Retina path, sound) and written down with the numbers before and after. The fix belongs in Macron's defaults or runtime, so a game with no profile gets it too; a profile for 871720 alone is not enough if the cause is general (every Direct3D 9 game, every 32-bit game). The sandbox's GPU is virtual and has no hardware OpenGL, so: the game holds 60 fps on the sandbox on the path Macron now picks by default; if it cannot, show with a native Metal program of the same load on the same sandbox that the virtual GPU is the limit, and that Macron's side (CPU profile, Wine server, translation layer) is not. A frame-rate check joins the scenario checks so this cannot regress silently: a legally free game or a program of ours on the same path (32-bit, Direct3D 9), with a frame-time measure and a threshold that does not come from Macron's output alone. The number on a real Apple GPU is Thalles's to see when he installs the release: make it easy for him (a frame counter he can switch on, and where the log is), and say so in the release notes.

2. **Steam's own buttons.** In a sandbox, with Macron installed and the Mac Steam client logged in to the test account: a Windows-only game the account owns shows a working Install button, Steam downloads it into its normal library folder through its normal UI, Play starts it through `macron` (Steam's `compat_log.txt` and macron's own log both show it), Steam shows the game as running, and quitting the game returns Steam to "Play". Drive Steam's real window (clicks and keys on the sandbox are fine there) and keep screenshots of each step as evidence. No `download_depot`, no hand-set environment, no `steam://` shortcut standing in for a button. If the Mac client needs its Steam Play switched on in memory, do that; Thalles has asked for this experience knowing phase 1's registration was not enough. Rules for it:
   - Prefer the least invasive way that works, and say what was tried. Changing Valve's files on disk is the last resort; whatever is done must be undone completely by Macron's uninstall, and Steam must start normally afterwards.
   - Only act on Steam builds Macron recognises (the hash table), and refuse cleanly on others, as the hook does now. Steam updates itself often, so say how a new build gets supported and make that as automatic as it can be.
   - Mac-native games must keep working exactly as before: check one (a free one the test account owns).
   - It must also hold on a Steam started from the notarised `/Applications/Steam.app` (the `macos-signed` variant Thalles has), not only the client the installer DMG downloads on a fresh machine.
   - CI keeps checking what it can without an account, and with the account's secrets where a workflow may use them (never on a fork's code).

3. **Something to install.** A DMG (or pkg inside one) published as a GitHub release of this repository: Thalles downloads it, installs it, opens Steam, and gate 2's experience is there. It bundles `macron` and fetches or carries the runtime, sets up the Steam side, and has an uninstall that puts everything back. No other UI is needed. Without an Apple Developer ID it cannot be notarised: make the unsigned path as painless as it can be, write the exact steps in the release notes, and put "a Developer ID" under "Needs Thalles". Test the published DMG itself, downloaded from the release, on a fresh sandbox the way he will use it: Steam installed from Valve's DMG into `/Applications` and logged in first, then Macron's installer, then gate 2's flow, then uninstall and check Steam is as it was.

4. **The Steam bridge.** Games that talk to Steam find it: `SteamAPI_Init` succeeds inside a game started by Play, through a Mac port of Proton's `lsteamclient` (from Valve's Proton source, see the licence rule), so games that quit without Steam run, and achievements and Steam Cloud work for at least one game. Checked on a sandbox with a free game the test account owns that uses them.

5. **Overwatch.** Thalles, 2026-11-14: "Can we run Overwatch? On ProtonDB it's rated Platinum, so it runs very well on Proton and I think it doesn't have any problems with compatibility layers. I'm wondering if we can get it to run." Overwatch (Steam app 2357570, free to play, Direct3D 11, 64-bit, about 83 GB) comes after gate 3 and does not hold the release back. On a sandbox the fair attempt is: the test account adds it, Steam installs it with its Install button if the sandbox's disk and the 5 hour 30 minute limit allow (about 97 GB is free; if not, say so with the numbers), and Play takes it as far as it goes without a Blizzard account, which is its sign-in screen. Do not create a Blizzard account. Record how far it got, with logs and a screenshot, in the README table and PROGRESS.md; the main menu and the frame rate on a real GPU are Thalles's to try with the release, so put that under "Needs Thalles".

Publish a release as soon as gate 3 is reached, even before gate 4, and say so at the top of PROGRESS.md: Thalles wants to try it. Publish again when gate 4 lands.

## Rules

- **PROGRESS.md is the loop's memory.** Read it first. Update it last: what was done, the evidence, what's next, what's blocked. Keep README.md true.
- **Thalles is away.** Don't stop to ask. A decision only he can make goes under "Needs Thalles" in PROGRESS.md, and you work on something else meanwhile.
- **Done means he would say so.** Phase 1 passed every gate and still missed what he wanted, because no gate pressed Play in a real Steam. Do not call a gate green on a stand-in for the real thing: gate 1 needs the real game, gate 2 needs the real buttons in a logged-in Steam, gate 3 needs the published DMG installed from scratch. What a sandbox cannot show (speed on a real Apple GPU) is said plainly in PROGRESS.md and the release notes, not papered over.
- **Never weaken a check to make it pass.** No raised thresholds, no dropped negative controls, no references regenerated from Macron's own output, no scenario marked as passing on partial evidence. Changing a check is fine only when the check itself was wrong; say why in the commit and in PROGRESS.md.
- **Models and usage.** Subagents run on Opus 5.5, never Fable. Run a few at a time, each with its own git worktree and a narrow brief. Use a workflow or Ultracode only when a problem clearly needs many parallel attempts, and note why in PROGRESS.md. On a usage-limit error: stop launching agents, write the state to PROGRESS.md, wait for the reset, carry on.
- **Games.** Legal downloads only: free ones from their publishers, and free-to-play Steam games the test account adds and Steam downloads. No cracks, no DRM bypass. Never commit game files, never upload them as artifacts, never copy them off the sandbox. A screenshot of a game's window as evidence is fine.
- **Licences.** Macron still has no licence. Record each dependency's licence in README.md. Changes to Wine are LGPL and get published. Don't copy code from GPL projects (NotProton, Proton-ish, Whisky); reading them to learn how something works is fine. Valve's Proton is the source for the Steam bridge; check the licence of each part you take and record it.
- **The repository is public.** Logs, artifacts and caches are world-readable and anyone can open a pull request. Never print a secret. No workflow may run code from a fork with access to anything: no `pull_request_target`, no self-hosted runners. Nothing from Thalles's own Steam (account name, id, library) goes into the repository, a log or an artifact, and the test account's password, tokens and session files never do either.
- **Sandboxes.** At most 5 Mac jobs run at once. Stop every sandbox when its work is done and leave no runs in progress. The tunnel sometimes stalls (see README.md), so run long jobs detached on the sandbox and poll them, and let CI workflows be the standing check.
- **Every step ends green and pushed.** Tests pass, it's committed and pushed to `main`, CI passes. Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Where to work

Everything happens on GitHub's Mac runners: the `sbx` sandboxes for hands-on work and the workflows as the standing checks. A sandbox has a logged-in desktop you can screenshot and drive, 5 cores, 14 GB, about 97 GB of free disk, and a virtual GPU (Metal works, no Metal 3, no hardware OpenGL). It lasts at most 5 hours 30 minutes and is erased when it stops, so script the setup (Steam, login, Macron) to be repeatable in minutes.

## The MacBook is off limits

Do not start anything on Thalles's MacBook (environment `87511d05-cf0b-4b25-9d13-83efe54da30d`): no `delegate_task`, no thread, no command, not for a measurement and not for cleanup. He withdrew it on 2026-11-14 because the test sessions disturbed his work. It was cleaned by hand afterwards (Macron's folders, the test folder, the downloaded depot, Steam quit; his Steam config was unchanged). What was learned there is in PROGRESS.md.

## The Steam test account

Thalles: "feel free to create a new steam account to test it out."

- Make one account for Macron's tests and use it only for that. It needs no payment: add free-to-play Windows-only games (Ultimate Custom Night, 871720, is one) and a free Mac-native game for the regression check. Never spend money, never trade, chat, post, add friends or join groups with it.
- Signing up needs an email address and passes a CAPTCHA. Do not use a service or a trick to defeat a CAPTCHA or any other bot check, and do not use Thalles's name, his email or his own Steam account. If an agent cannot finish the sign-up honestly, stop trying, write the exact steps and where the credentials go under "Needs Thalles" (it is a few minutes for him), and work on what needs no account meanwhile: gate 1's investigation, the switch itself against the login window, the installer.
- A fresh sandbox is a new device every time, so plan the login: Steam Guard by email needs a mailbox you can read; an account with the mobile authenticator needs its shared secret to compute codes. Decide once and write down how a sandbox logs in.
- Credentials live outside the repository: on the dev machine in `~/.config/macron/steam-test-account` (mode 600), and as GitHub Actions secrets if a workflow needs them. They never appear in a commit, a log, an artifact, a screenshot or a command line that a workflow prints. Workflows that use them run only on `main` and `workflow_dispatch`, never on pull requests.
- Online games on the test account: the rule is the same as for a person. No multiplayer or anti-cheat game is started except Overwatch as gate 5 describes; games with kernel anti-cheat are never run.

## When stuck

- If a check fails for a reason outside Macron (a flaky runner, a tool bug, a download that moved), fix the tool, write it down in PROGRESS.md, and carry on.
- If the cause of the slowness is not where the first measurement points, measure the next layer; don't ship a guess. If the fix needs a different Wine build (one with msync, CrossOver's sources, a newer DXVK or MoltenVK), build and publish it the way `runtime/wine/` does.
- If switching on Steam Play resists one approach, write down what was tried and what the client did, and try the next. How NotProton does it is readable; the code is not copyable.
- If a decision only Thalles can make blocks the way, add it under "Needs Thalles" and work on another gate.

## Phase 1 gates (green, keep them green)

1. One command runs a scenario end to end in a Mac sandbox and in CI (`macron-check sandbox <scenario>`).
2. A real game reaches its menu and matches the original on Windows (OpenTTD).
3. Scenarios launch through `macron` with a pinned runtime, a per-game prefix and the fixes database.
4. Three games from three engines pass, one of at least 1 GB (OpenTTD, Xonotic, SuperTuxKart).
5. The Steam registration applies to the current build, Steam still reaches its login window, and unknown builds are refused.
