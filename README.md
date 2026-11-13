# Macron

Mac + Proton. Press Play on a Windows-only game in the Mac Steam client and have it run.

**Status:** nothing works yet. This is the plan.

## How it will work

1. **Steam hook**: switches on the Steam Play code that already ships (disabled) in Steam for Mac and registers Macron as a compatibility tool.
2. **Launcher**: the headless program Steam calls on Play. It prepares a private Windows environment for the game and starts it under Wine.
3. **Steam bridge**: a Mac port of Proton's `lsteamclient`, so games reach the native Steam client for ownership, achievements and cloud saves.
4. **Runtime**: Wine with DXMT (DirectX 11) and D3DMetal (DirectX 12). Rosetta 2 first, FEX later.
5. **Fixes database**: per-game settings keyed by Steam app ID.
6. **Installer**: one package that sets everything up. The only UI.

## Milestones

- [ ] Steam calls a dummy compatibility tool when Play is pressed
- [ ] One DirectX 11 game installs, launches, saves and unlocks an achievement
- [ ] Installer, fixes database and ten tested games
- [ ] DirectX 12, controllers, FEX backend

## Not planned

Games that need kernel anti-cheat (EasyAntiCheat, BattlEye, Vanguard).
