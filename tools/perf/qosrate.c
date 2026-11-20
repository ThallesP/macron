/* What a hot Mac does to a thread's speed, by the scheduling policy the
 * thread has: whether the game's own work, which on a throttled MacBook Air
 * took four to seven times as long a frame, would be spared with another
 * policy.
 *
 *   qosrate <seconds> <heaters>
 *
 * Six worker threads, one for each policy, each do the same fixed piece of
 * work once every 10 ms (about 4 ms of it on a cool performance core) and
 * note how long it took, as a game's thread does a frame's work and waits:
 *   default      what any thread has, and what a game's threads have in Wine
 *   interactive  QOS_CLASS_USER_INTERACTIVE
 *   initiated    QOS_CLASS_USER_INITIATED
 *   utility      QOS_CLASS_UTILITY
 *   fixed        not timeshare, importance 32 (priority 63): Wine's
 *                time-critical threads
 *   realtime     a time constraint of 5 ms in every 10
 * Meanwhile <heaters> threads never sleep, at the default policy, to heat
 * the Mac. Every 5 s one RESULT line: the thermal pressure level (what
 * `notifyutil -g com.apple.system.thermalpressurelevel` prints), each
 * worker's median and 95th percentile for the piece of work in ms, and how
 * busy each processor core was (on an M3, cores 0 to 3 are the efficiency
 * cores). The first lines, at level 0, are the reference.
 *
 * Build it for both architectures and run the Intel one too
 * (`arch -x86_64 ./qosrate ...`), which is what a game is under Rosetta:
 *   clang -O2 -arch arm64 -arch x86_64 -o qosrate qosrate.c */
#include <mach/mach.h>
#include <mach/mach_time.h>
#include <mach/processor_info.h>
#include <mach/thread_policy.h>
#include <notify.h>
#include <pthread.h>
#include <pthread/qos.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define PERIOD_NS 10000000ull
#define WORKERS 6
#define MAX_SAMPLES 600
#define MAX_CORES 64

enum policy { POLICY_DEFAULT, POLICY_INTERACTIVE, POLICY_INITIATED, POLICY_UTILITY, POLICY_FIXED, POLICY_REALTIME };

static const char *names[WORKERS] = {"default", "interactive", "initiated", "utility", "fixed", "realtime"};

struct worker {
    enum policy policy;
    pthread_mutex_t lock;
    double ms[MAX_SAMPLES];
    size_t count;
};

static mach_timebase_info_data_t timebase;
static struct worker workers[WORKERS];
static unsigned long work_iterations;
static volatile int quit;

static uint64_t now_ns(void)
{
    return mach_absolute_time() * timebase.numer / timebase.denom;
}

static uint64_t ticks(uint64_t ns)
{
    return ns * timebase.denom / timebase.numer;
}

static unsigned long spin(unsigned long iterations)
{
    volatile unsigned long sink = 0;
    for (unsigned long i = 0; i < iterations; i++) sink += i ^ (sink >> 3);
    return sink;
}

static void apply(enum policy policy)
{
    thread_extended_policy_data_t extended = {.timeshare = 0};
    thread_precedence_policy_data_t precedence = {.importance = 32};
    thread_time_constraint_policy_data_t constraint = {
        .period = (uint32_t)ticks(PERIOD_NS),
        .computation = (uint32_t)ticks(PERIOD_NS / 2),
        .constraint = (uint32_t)ticks(PERIOD_NS),
        .preemptible = 1,
    };
    thread_t self = mach_thread_self();

    if (policy == POLICY_INTERACTIVE) pthread_set_qos_class_self_np(QOS_CLASS_USER_INTERACTIVE, 0);
    if (policy == POLICY_INITIATED) pthread_set_qos_class_self_np(QOS_CLASS_USER_INITIATED, 0);
    if (policy == POLICY_UTILITY) pthread_set_qos_class_self_np(QOS_CLASS_UTILITY, 0);
    if (policy == POLICY_FIXED) {
        thread_policy_set(self, THREAD_EXTENDED_POLICY, (thread_policy_t)&extended, THREAD_EXTENDED_POLICY_COUNT);
        thread_policy_set(self, THREAD_PRECEDENCE_POLICY, (thread_policy_t)&precedence, THREAD_PRECEDENCE_POLICY_COUNT);
    }
    if (policy == POLICY_REALTIME)
        thread_policy_set(self, THREAD_TIME_CONSTRAINT_POLICY, (thread_policy_t)&constraint, THREAD_TIME_CONSTRAINT_POLICY_COUNT);
}

static void *work(void *arg)
{
    struct worker *worker = arg;
    uint64_t next = mach_absolute_time();

    apply(worker->policy);
    while (!quit) {
        uint64_t start = now_ns();
        spin(work_iterations);
        double ms = (now_ns() - start) / 1e6;
        pthread_mutex_lock(&worker->lock);
        if (worker->count < MAX_SAMPLES) worker->ms[worker->count++] = ms;
        pthread_mutex_unlock(&worker->lock);
        next += ticks(PERIOD_NS);
        if (next < mach_absolute_time()) next = mach_absolute_time();
        mach_wait_until(next);
    }
    return NULL;
}

static void *heat(void *unused)
{
    while (!quit) spin(1000000);
    return NULL;
}

static int compare(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

static unsigned core_ticks(unsigned long long busy[MAX_CORES], unsigned long long total[MAX_CORES])
{
    processor_info_array_t info;
    mach_msg_type_number_t info_count;
    natural_t cores;

    if (host_processor_info(mach_host_self(), PROCESSOR_CPU_LOAD_INFO, &cores, &info, &info_count) != KERN_SUCCESS) return 0;
    if (cores > MAX_CORES) cores = MAX_CORES;
    for (natural_t core = 0; core < cores; core++) {
        integer_t *load = info + core * CPU_STATE_MAX;
        busy[core] = (unsigned long long)load[CPU_STATE_USER] + load[CPU_STATE_SYSTEM] + load[CPU_STATE_NICE];
        total[core] = busy[core] + load[CPU_STATE_IDLE];
    }
    vm_deallocate(mach_task_self(), (vm_address_t)info, info_count * sizeof(integer_t));
    return cores;
}

int main(int argc, char **argv)
{
    if (argc != 3) {
        fprintf(stderr, "usage: qosrate <seconds> <heaters>\n");
        return 2;
    }
    int seconds = atoi(argv[1]), heaters = atoi(argv[2]);
    mach_timebase_info(&timebase);

    /* The piece of work: what takes about 4 ms here and now, before anything else runs. */
    uint64_t start = now_ns();
    spin(20000000);
    work_iterations = (unsigned long)(20000000.0 * 4e6 / (now_ns() - start));

    int token = NOTIFY_TOKEN_INVALID;
    notify_register_check("com.apple.system.thermalpressurelevel", &token);

    pthread_t thread;
    for (int i = 0; i < WORKERS; i++) {
        workers[i].policy = (enum policy)i;
        pthread_mutex_init(&workers[i].lock, NULL);
        pthread_create(&thread, NULL, work, &workers[i]);
    }
    for (int i = 0; i < heaters; i++) pthread_create(&thread, NULL, heat, NULL);

#if defined(__x86_64__)
    const char *arch = "x86_64";
#else
    const char *arch = "arm64";
#endif
    printf("RESULT config %s, %d s, %d heaters, the piece of work is %lu iterations\n", arch, seconds, heaters, work_iterations);

    unsigned long long busy_before[MAX_CORES], total_before[MAX_CORES], busy[MAX_CORES], total[MAX_CORES];
    core_ticks(busy_before, total_before);
    for (int elapsed = 5; elapsed <= seconds; elapsed += 5) {
        sleep(5);
        uint64_t level = 0;
        if (token != NOTIFY_TOKEN_INVALID) notify_get_state(token, &level);
        printf("RESULT %4d s, level %llu, work ms median/p95:", elapsed, (unsigned long long)level);
        for (int i = 0; i < WORKERS; i++) {
            double ms[MAX_SAMPLES];
            pthread_mutex_lock(&workers[i].lock);
            size_t count = workers[i].count;
            memcpy(ms, workers[i].ms, count * sizeof(*ms));
            workers[i].count = 0;
            pthread_mutex_unlock(&workers[i].lock);
            if (!count) {
                printf(" %s none", names[i]);
                continue;
            }
            qsort(ms, count, sizeof(*ms), compare);
            printf(" %s %.2f/%.2f (%zu)", names[i], ms[count / 2], ms[count * 95 / 100], count);
        }
        unsigned cores = core_ticks(busy, total);
        printf("; cores busy %%:");
        for (unsigned core = 0; core < cores; core++) {
            unsigned long long span = total[core] - total_before[core];
            printf(" %llu", span ? (busy[core] - busy_before[core]) * 100 / span : 0);
            busy_before[core] = busy[core];
            total_before[core] = total[core];
        }
        printf("\n");
        fflush(stdout);
    }
    quit = 1;
    return 0;
}
