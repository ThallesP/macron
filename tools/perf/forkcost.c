/* How long fork() holds the calling thread, by how much memory the process
 * has touched: what Wine's server would pay to hand its registry save to a
 * child (Wine patch 0021).
 *
 *   forkcost <megabytes> [<megabytes> ...]
 *
 * For each size: touches that much heap in 64-byte pieces (a registry is many
 * small allocations), forks eight times, the child exiting at once, and
 * prints the median and the longest time fork() took in the parent.
 * Build for both architectures and run the Intel one (`arch -x86_64`), which
 * is what Wine's server is under Rosetta:
 *   clang -O2 -arch arm64 -arch x86_64 -o forkcost forkcost.c */
#include <mach/mach_time.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/wait.h>
#include <unistd.h>

static int compare(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

int main(int argc, char **argv)
{
    mach_timebase_info_data_t tb;
    size_t held = 0;
    mach_timebase_info(&tb);
    for (int arg = 1; arg < argc; arg++) {
        size_t want = (size_t)atoi(argv[arg]) << 20;
        while (held < want) {
            char *piece = malloc(64);
            memset(piece, 1, 64);
            held += 64;
        }
        double ms[8];
        for (int i = 0; i < 8; i++) {
            uint64_t before = mach_absolute_time();
            pid_t pid = fork();
            if (!pid) _exit(0);
            ms[i] = (mach_absolute_time() - before) * tb.numer / tb.denom / 1e6;
            waitpid(pid, NULL, 0);
        }
        qsort(ms, 8, sizeof(*ms), compare);
#if defined(__x86_64__)
        const char *arch = "x86_64";
#else
        const char *arch = "arm64";
#endif
        printf("RESULT %s, %zu MB touched: fork() median %.2f ms, longest %.2f ms\n", arch, held >> 20, ms[4], ms[7]);
    }
    return 0;
}
