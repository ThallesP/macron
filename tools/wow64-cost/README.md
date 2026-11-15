# What a Windows program's basic operations cost under Wine and Rosetta

Gate 1 (PROGRESS.md, "Gate 1, what the sandbox shows") needed to know what Macron's side costs a game with no GPU in the way.

- `cost.c` (32-bit and 64-bit Windows): nanoseconds per `SwitchToThread` (`yield`), `VirtualQuery` (`query`, a system call Wine answers in ntdll's Unix side), `WaitForSingleObject` on a signalled event (`wait`, a wineserver round trip without esync or msync), an x87 multiply-add, the same in SSE2, and an empty loop.
  ```sh
  llvm-mingw/bin/i686-w64-mingw32-clang -O2 -msse2 -mfpmath=sse -mconsole cost.c -o cost32.exe
  llvm-mingw/bin/x86_64-w64-mingw32-clang -O2 -mconsole cost.c -o cost64.exe
  STEAM_COMPAT_DATA_PATH=/tmp/costpfx SteamAppId=1 macron waitforexitandrun "$PWD/cost32.exe"
  ```
- `native.c` (a Mac program): the pieces of Wine's x86_64 system call and Unix call dispatchers on macOS, timed one at a time, so under Rosetta when built for x86_64: a plain system call, the `thread_set_tsd_base` Mach trap (`0x3000003`) Wine makes twice per transition to switch `%gs` between the TEB and macOS's thread data, `xsave64`/`xrstor64`, `fxsave64`/`fxrstor64`, `pushfq`/`popfq`, `stmxcsr`/`ldmxcsr`.
  ```sh
  clang -O2 -arch x86_64 -o native-x86_64 native.c && ./native-x86_64
  clang -O2 -arch arm64 -o native-arm64 native.c && ./native-arm64     # the plain system call only
  ```
