/* How late a thread wakes on a busy Mac, by the scheduling policy it has:
 * the question behind every late sound wake-up under Wine, without Wine.
 *
 *   wakelat <seconds> <busy threads> timeshare|fixed|realtime
 *
 * A real-time thread, as CoreAudio's I/O thread is, signals a semaphore every
 * 10 ms. A second thread waits for it and takes the time from the signal to
 * its own wake-up. That thread has the policy named:
 *   timeshare  what any thread has
 *   fixed      not timeshare, importance 32 (priority 63): what Wine's server
 *              gives a Windows thread of THREAD_PRIORITY_TIME_CRITICAL, and
 *              what Macron's patches 0014 and 0015 give a game's sound thread
 *              and the server itself
 *   realtime   a time constraint (0.5 ms of work every 10 ms, done within
 *              2 ms): the real-time band, where CoreAudio's own threads are
 * Meanwhile <busy threads> threads never sleep, at ordinary priority.
 *
 * Prints one RESULT line. Build it for both architectures and run the Intel
 * one too (`arch -x86_64 ./wakelat ...`), which is what a game is under
 * Rosetta:
 *   clang -O2 -arch arm64 -arch x86_64 -o wakelat wakelat.c */
#include <mach/mach.h>
#include <mach/mach_time.h>
#include <mach/thread_policy.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define PERIOD_NS 10000000ull
#define MAX_WAKES 100000

static mach_timebase_info_data_t timebase;
static semaphore_t wake;
static volatile uint64_t signalled_at;
static volatile int quit;
static double seconds;

static uint64_t now_ns(void)
{
    return mach_absolute_time() * timebase.numer / timebase.denom;
}

static uint64_t ticks(uint64_t ns)
{
    return ns * timebase.denom / timebase.numer;
}

static int set_realtime(uint64_t period_ns, uint64_t computation_ns, uint64_t constraint_ns)
{
    thread_time_constraint_policy_data_t policy;
    policy.period = (uint32_t)ticks(period_ns);
    policy.computation = (uint32_t)ticks(computation_ns);
    policy.constraint = (uint32_t)ticks(constraint_ns);
    policy.preemptible = 1;
    return thread_policy_set(mach_thread_self(), THREAD_TIME_CONSTRAINT_POLICY, (thread_policy_t)&policy,
                             THREAD_TIME_CONSTRAINT_POLICY_COUNT);
}

static int set_fixed(void)
{
    thread_extended_policy_data_t extended = {.timeshare = 0};
    thread_precedence_policy_data_t precedence = {.importance = 32};
    kern_return_t ret = thread_policy_set(mach_thread_self(), THREAD_EXTENDED_POLICY, (thread_policy_t)&extended,
                                          THREAD_EXTENDED_POLICY_COUNT);
    if (ret != KERN_SUCCESS) return ret;
    return thread_policy_set(mach_thread_self(), THREAD_PRECEDENCE_POLICY, (thread_policy_t)&precedence,
                             THREAD_PRECEDENCE_POLICY_COUNT);
}

static void *busy(void *unused)
{
    volatile unsigned sink = 0;
    while (!quit)
        for (unsigned i = 0; i < 100000; i++) sink += i;
    return NULL;
}

static void *signaller(void *unused)
{
    uint64_t next = mach_absolute_time();
    set_realtime(PERIOD_NS, 200000, 1000000);
    while (!quit) {
        next += ticks(PERIOD_NS);
        mach_wait_until(next);
        signalled_at = now_ns();
        semaphore_signal(wake);
    }
    return NULL;
}

static int compare(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

int main(int argc, char **argv)
{
    if (argc != 4) {
        fprintf(stderr, "usage: wakelat <seconds> <busy threads> timeshare|fixed|realtime\n");
        return 2;
    }
    seconds = atof(argv[1]);
    int load = atoi(argv[2]);
    const char *mode = argv[3];
    mach_timebase_info(&timebase);
    semaphore_create(mach_task_self(), &wake, SYNC_POLICY_FIFO, 0);

    int policy = 0;
    if (!strcmp(mode, "fixed")) policy = set_fixed();
    else if (!strcmp(mode, "realtime")) policy = set_realtime(PERIOD_NS, 500000, 2000000);
    else if (strcmp(mode, "timeshare")) {
        fprintf(stderr, "unknown policy %s\n", mode);
        return 2;
    }

    pthread_t thread;
    for (int i = 0; i < load; i++) pthread_create(&thread, NULL, busy, NULL);
    pthread_create(&thread, NULL, signaller, NULL);

    double *late = malloc(MAX_WAKES * sizeof(*late));
    size_t count = 0;
    unsigned over_1 = 0, over_2 = 0, over_10 = 0, over_35 = 0;
    uint64_t start = now_ns(), measure_from = start + 1000000000ull, end = measure_from + (uint64_t)(seconds * 1e9);
    while (now_ns() < end && count < MAX_WAKES) {
        semaphore_wait(wake);
        uint64_t woke = now_ns();
        if (woke < measure_from) continue;
        double ms = (woke - signalled_at) / 1e6;
        late[count++] = ms;
        if (ms > 1) over_1++;
        if (ms > 2) over_2++;
        if (ms > 10) over_10++;
        if (ms > 35) over_35++;
    }
    quit = 1;
    if (!count) {
        printf("RESULT wake: none\n");
        return 1;
    }
    qsort(late, count, sizeof(*late), compare);
#if defined(__x86_64__)
    const char *arch = "x86_64";
#else
    const char *arch = "arm64";
#endif
    printf("RESULT %s, %s (policy call %d), %d busy threads: %zu wake-ups in %.0f s, ms from the signal p50 %.3f p95 %.3f p99 %.3f max %.3f, over 1 ms %u, over 2 ms %u, over 10 ms %u, over 35 ms %u\n",
           mode, arch, policy, load, count, seconds, late[count / 2], late[count * 95 / 100], late[count * 99 / 100], late[count - 1],
           over_1, over_2, over_10, over_35);
    return 0;
}
