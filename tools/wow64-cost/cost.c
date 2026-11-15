/* What one 32-bit Windows program's basic operations cost under Wine and
 * Rosetta, in nanoseconds each, to tell apart the layers a 32-bit game pays
 * for without a GPU in the way:
 *
 *   yield    SwitchToThread(): NtYieldExecution, which may hand the core to
 *            another thread
 *   query    VirtualQuery() on the program's own code: NtQueryVirtualMemory,
 *            which Wine answers in ntdll's Unix side without the server; in a
 *            32-bit process that is one switch from 32-bit to 64-bit code and
 *            back (Wine's wow64cpu.dll) plus WoW64's thunk
 *   wait     WaitForSingleObject() on a signalled event with no timeout: a
 *            round trip to wineserver when Wine has no esync or msync
 *   x87      a dependent chain of x87 multiply-adds (long double), which
 *            32-bit compilers of the 2000s and 2010s emit for every float
 *   sse      the same chain in SSE2 doubles
 *   loop     an empty loop of the same length, subtracted from nothing (for
 *            scale)
 *
 *   cost.exe [--n N]   (default 2,000,000 per test, 3 repetitions each)
 *
 * Prints one RESULT line per test with the best of the repetitions.
 */
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static double now(void)
{
    LARGE_INTEGER t, f;
    QueryPerformanceCounter(&t);
    QueryPerformanceFrequency(&f);
    return (double)t.QuadPart / (double)f.QuadPart;
}

static volatile double sink;

static double run_yield(long n)
{
    double t = now();
    for (long i = 0; i < n; i++) SwitchToThread();
    return now() - t;
}

static double run_query(long n)
{
    MEMORY_BASIC_INFORMATION info;
    double t = now();
    for (long i = 0; i < n; i++) VirtualQuery((void *)run_query, &info, sizeof(info));
    return now() - t;
}

static double run_wait(long n)
{
    HANDLE event = CreateEventA(NULL, TRUE, TRUE, NULL);
    double t = now();
    for (long i = 0; i < n; i++) WaitForSingleObject(event, 0);
    t = now() - t;
    CloseHandle(event);
    return t;
}

static double __attribute__((noinline)) run_x87(long n)
{
    volatile long double a = 1.0000001L, b = 0.9999999L;
    long double x = 1.0L, ma = a, mb = b;
    double t = now();
    for (long i = 0; i < n; i++) x = x * ma + mb;
    sink = (double)x;
    return now() - t;
}

static double __attribute__((noinline)) run_sse(long n)
{
    volatile double a = 1.0000001, b = 0.9999999;
    double x = 1.0, ma = a, mb = b;
    double t = now();
    for (long i = 0; i < n; i++) x = x * ma + mb;
    sink = x;
    return now() - t;
}

static double __attribute__((noinline)) run_loop(long n)
{
    volatile long k = 0;
    double t = now();
    for (long i = 0; i < n; i++) k = k + 1;
    return now() - t;
}

int main(int argc, char **argv)
{
    long n = 2000000;
    struct { const char *name; double (*fn)(long); long scale; } tests[] = {
        {"yield", run_yield, 100}, {"query", run_query, 100}, {"wait", run_wait, 100}, {"x87", run_x87, 1}, {"sse", run_sse, 1}, {"loop", run_loop, 1}};

    setvbuf(stdout, NULL, _IONBF, 0);
    for (int i = 1; i < argc; i++)
        if (!strcmp(argv[i], "--n") && i + 1 < argc) n = atol(argv[++i]);
    printf("RESULT bits %u n %ld\n", (unsigned)(sizeof(void *) * 8), n);
    for (unsigned t = 0; t < sizeof(tests) / sizeof(*tests); t++) {
        long count = n / tests[t].scale;
        double best = 1e30;
        for (int r = 0; r < 3; r++) {
            double s = tests[t].fn(count);
            if (s < best) best = s;
        }
        printf("RESULT %s %.1f ns each (%ld in %.3f s)\n", tests[t].name, best * 1e9 / count, count, best);
    }
    return 0;
}
