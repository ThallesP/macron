/* The pieces of Wine's x86_64 system call dispatcher on macOS
 * (__wine_syscall_dispatcher in ntdll.so), timed one at a time in a native
 * x86_64 Mac program, so under Rosetta exactly as Wine runs them:
 *
 *   getpid   a plain macOS system call, for scale
 *   tsd      the Mach trap 0x3000003 (thread_set_tsd_base) Wine makes twice
 *            per Windows system call to switch %gs between the TEB and
 *            macOS's thread data (here set to the value it already has)
 *   xsave    xsave64 + xrstor64 of the whole extended state (Wine uses xsavec64
 *            when the CPU has it; Rosetta does not), which Wine does
 *            once per call
 *   fxsave   fxsave64 + fxrstor64, the older form
 *   flags    pushfq + popfq
 *   mxcsr    stmxcsr + ldmxcsr
 *
 *   clang -O2 -arch x86_64 -o native-x86_64 native.c   (and -arch arm64 for getpid alone)
 */
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

static double now(void)
{
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return t.tv_sec + t.tv_nsec / 1e9;
}

#define TIME(name, n, body)                                                                         \
    do {                                                                                            \
        double best = 1e30;                                                                         \
        for (int r = 0; r < 3; r++) {                                                               \
            double t = now();                                                                       \
            for (long i = 0; i < (n); i++) { body; }                                                \
            t = now() - t;                                                                          \
            if (t < best) best = t;                                                                 \
        }                                                                                           \
        printf("RESULT %s %.1f ns each (%ld)\n", name, best * 1e9 / (n), (long)(n));                \
    } while (0)

int main(void)
{
    setvbuf(stdout, NULL, _IONBF, 0);
#if defined(__x86_64__)
    printf("RESULT arch x86_64\n");
#else
    printf("RESULT arch arm64\n");
#endif
    TIME("getpid", 200000, getppid());
#if defined(__x86_64__)
    {
        uintptr_t base = 0, self = (uintptr_t)pthread_self();
        static uint8_t area[8192] __attribute__((aligned(64)));
        /* %gs points at the thread's TSD array inside its pthread_t; slot 0
         * holds pthread_self(), so the base is the address of that word. */
        for (uintptr_t *p = (uintptr_t *)self; p < (uintptr_t *)(self + 4096); p++)
            if (*p == self) { base = (uintptr_t)p; break; }
        {
            uintptr_t check;
            __asm__ volatile("movq %%gs:0, %0" : "=r"(check));
            if (!base || check != self) { printf("RESULT tsd: base not found\n"); return 1; }
        }
        TIME("tsd", 200000, {
            uintptr_t rdi = base;
            long rax = 0x3000003;
            __asm__ volatile("syscall" : "+a"(rax), "+D"(rdi) : : "rcx", "r11", "rdx", "rsi", "memory");
        });
        TIME("xsave", 200000, __asm__ volatile("xsave64 (%0)\n\txrstor64 (%0)" : : "r"(area), "a"(-1), "d"(-1) : "memory"));
        TIME("fxsave", 200000, __asm__ volatile("fxsave64 (%0)\n\tfxrstor64 (%0)" : : "r"(area) : "memory"));
        TIME("flags", 2000000, __asm__ volatile("pushfq\n\tpopfq" : : : "memory", "cc"));
        {
            uint32_t m;
            TIME("mxcsr", 2000000, __asm__ volatile("stmxcsr %0\n\tldmxcsr %0" : "+m"(m)));
        }
    }
#endif
    return 0;
}
