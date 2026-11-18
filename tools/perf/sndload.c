/* How late a game's sound thread wakes while the game keeps every core busy,
 * with and without registering it for audio (Wine patch 0014).
 *
 *   sndload.exe [--task NAME|none] [--seconds S] [--load N] [--high N]
 *               [--mix MS] [--stream 0|1] [--wake event|address]
 *               [--timer sleep|address] [--out FILE]
 *
 * The sound thread does what a game's does: it opens a shared, event-driven
 * WASAPI stream on the default output with a 512-frame period (as Deadlock
 * does), registers with AvSetMmThreadCharacteristicsW(NAME) unless NAME is
 * `none` (default "Pro Audio"), and at every wake-up fills the free part of
 * the buffer with silence after spinning for MS milliseconds (default 2: a
 * game mixes). Nothing is heard. Without an output device, or with
 * --stream 0, a time-critical thread sets the event every 10 ms instead, as
 * Wine's own audio timer thread does, against a 35 ms buffer on paper. That
 * thread notes when each tick was due and when it signalled, so RESULT tick
 * splits a late wake-up in two: how late the timer was, and how long the
 * oldest signal the sound thread had not yet seen took to wake it. Under Wine an event's signal goes
 * through wineserver; --wake address (with --stream 0) signals with
 * WakeByAddressSingle instead, which does not, so the two side by side show
 * what wineserver adds. The timer thread sleeps with Sleep(), as Wine's does
 * (NtDelayExecution, which yields the processor before it waits); --timer
 * address makes it wait on an address with a timeout instead, which does not
 * yield.
 *
 * The load is N threads that never sleep at normal priority (default: one a
 * processor) and N at THREAD_PRIORITY_HIGHEST (default: half as many), which
 * is what a game's worker threads look like to the scheduler.
 *
 * RESULT lines go to stdout and to the --out file (default sndload.txt):
 * the thread's priority before and after registering, the time between its
 * wake-ups (median, p95, p99, longest), how many came later than two periods
 * and later than the whole buffer, and how often it found the buffer empty
 * (an audible gap).
 *
 * Build: x86_64-w64-mingw32-gcc -O2 -o sndload.exe sndload.c -lole32 -lavrt -lsynchronization */
#define COBJMACROS
#include <initguid.h>
#include <windows.h>
#include <audioclient.h>
#include <avrt.h>
#include <mmdeviceapi.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define PERIOD_FRAMES 512
#define TIMER_PERIOD_MS 10.0
#define TIMER_BUFFER_MS 35.0
#define MAX_WAKES 200000
#define TICK_RING 4096

static FILE *result_file;
static volatile LONG quit;
static LARGE_INTEGER frequency;
static HANDLE wake_event;
static BOOL wake_by_address, timer_waits_on_address;
static volatile LONG tick_count;
static volatile double tick_due_ms[TICK_RING], tick_signalled_ms[TICK_RING];

static void result(const char *format, ...)
{
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
    fflush(stdout);
    if (result_file) {
        va_start(args, format);
        vfprintf(result_file, format, args);
        va_end(args);
        fflush(result_file);
    }
}

static double now_ms(void)
{
    LARGE_INTEGER counter;
    QueryPerformanceCounter(&counter);
    return counter.QuadPart * 1000.0 / frequency.QuadPart;
}

static void spin_ms(double ms)
{
    double end = now_ms() + ms;
    volatile unsigned sink = 0;
    while (now_ms() < end)
        for (unsigned i = 0; i < 2000; i++) sink += i;
}

static DWORD WINAPI load_thread(void *priority)
{
    volatile unsigned sink = 0;
    SetThreadPriority(GetCurrentThread(), (int)(INT_PTR)priority);
    while (!quit)
        for (unsigned i = 0; i < 100000; i++) sink += i;
    return 0;
}

static DWORD WINAPI timer_thread(void *unused)
{
    double next = now_ms();
    SetThreadPriority(GetCurrentThread(), THREAD_PRIORITY_TIME_CRITICAL);
    while (!quit) {
        LONG tick = tick_count + 1;
        tick_due_ms[tick % TICK_RING] = next;
        tick_signalled_ms[tick % TICK_RING] = now_ms();
        InterlockedExchange(&tick_count, tick);
        if (wake_by_address) WakeByAddressSingle((void *)&tick_count);
        else SetEvent(wake_event);
        next += TIMER_PERIOD_MS;
        double wait = next - now_ms();
        if (wait <= 0) continue;
        if (timer_waits_on_address) {
            LONG never_changes = 0, same = 0;
            WaitOnAddress(&never_changes, &same, sizeof(same), (DWORD)(wait + 0.5));
        } else {
            Sleep((DWORD)(wait + 0.5));
        }
    }
    return 0;
}

static int compare_doubles(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return (x > y) - (x < y);
}

static BOOL wait_for_tick(LONG consumed)
{
    if (!wake_by_address) return WaitForSingleObject(wake_event, 2000) == WAIT_OBJECT_0;
    while (tick_count == consumed)
        if (!WaitOnAddress((void *)&tick_count, &consumed, sizeof(consumed), 2000)) return FALSE;
    return TRUE;
}

static void result_spread(const char *name, double *values, size_t count)
{
    qsort(values, count, sizeof(*values), compare_doubles);
    result("RESULT tick %s ms p50 %.2f p95 %.2f p99 %.2f max %.2f\n", name, values[count / 2], values[count * 95 / 100],
           values[count * 99 / 100], values[count - 1]);
}

struct stream {
    IAudioClient *client;
    IAudioRenderClient *render;
    UINT32 buffer_frames;
    double period_ms;
    double buffer_ms;
};

static BOOL open_stream(struct stream *stream)
{
    IMMDeviceEnumerator *devices = NULL;
    IMMDevice *device = NULL;
    IAudioClient3 *client3 = NULL;
    WAVEFORMATEX *format = NULL;
    HRESULT hr;

    hr = CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL, &IID_IMMDeviceEnumerator, (void **)&devices);
    if (FAILED(hr)) { result("RESULT stream: no device enumerator hr=0x%08lx\n", (unsigned long)hr); return FALSE; }
    hr = IMMDeviceEnumerator_GetDefaultAudioEndpoint(devices, eRender, eConsole, &device);
    if (FAILED(hr)) { result("RESULT stream: no default output hr=0x%08lx\n", (unsigned long)hr); return FALSE; }
    hr = IMMDevice_Activate(device, &IID_IAudioClient, CLSCTX_ALL, NULL, (void **)&stream->client);
    if (FAILED(hr)) { result("RESULT stream: Activate hr=0x%08lx\n", (unsigned long)hr); return FALSE; }
    hr = IAudioClient_GetMixFormat(stream->client, &format);
    if (FAILED(hr)) { result("RESULT stream: GetMixFormat hr=0x%08lx\n", (unsigned long)hr); return FALSE; }

    hr = IAudioClient_QueryInterface(stream->client, &IID_IAudioClient3, (void **)&client3);
    if (SUCCEEDED(hr))
        hr = IAudioClient3_InitializeSharedAudioStream(client3, AUDCLNT_STREAMFLAGS_EVENTCALLBACK, PERIOD_FRAMES, format, NULL);
    if (FAILED(hr))
        hr = IAudioClient_Initialize(stream->client, AUDCLNT_SHAREMODE_SHARED, AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
                                     (REFERENCE_TIME)(3.0 * PERIOD_FRAMES * 10000000.0 / format->nSamplesPerSec + 0.5), 0, format, NULL);
    if (FAILED(hr)) { result("RESULT stream: Initialize hr=0x%08lx\n", (unsigned long)hr); return FALSE; }

    IAudioClient_GetBufferSize(stream->client, &stream->buffer_frames);
    IAudioClient_SetEventHandle(stream->client, wake_event);
    hr = IAudioClient_GetService(stream->client, &IID_IAudioRenderClient, (void **)&stream->render);
    if (FAILED(hr)) { result("RESULT stream: GetService hr=0x%08lx\n", (unsigned long)hr); return FALSE; }

    stream->period_ms = PERIOD_FRAMES * 1000.0 / format->nSamplesPerSec;
    stream->buffer_ms = stream->buffer_frames * 1000.0 / format->nSamplesPerSec;
    result("RESULT stream: %lu Hz, %u channels, buffer %u frames (%.2f ms), period %.2f ms\n", (unsigned long)format->nSamplesPerSec,
           format->nChannels, stream->buffer_frames, stream->buffer_ms, stream->period_ms);
    return TRUE;
}

static UINT32 fill(struct stream *stream)
{
    UINT32 padding = 0;
    BYTE *data;

    IAudioClient_GetCurrentPadding(stream->client, &padding);
    UINT32 frames = stream->buffer_frames - padding;
    if (frames && SUCCEEDED(IAudioRenderClient_GetBuffer(stream->render, frames, &data)))
        IAudioRenderClient_ReleaseBuffer(stream->render, frames, AUDCLNT_BUFFERFLAGS_SILENT);
    return padding;
}

int main(int argc, char **argv)
{
    const char *task = "Pro Audio", *out = "sndload.txt";
    double seconds = 20, mix_ms = 2;
    SYSTEM_INFO info;
    GetSystemInfo(&info);
    int load = (int)info.dwNumberOfProcessors, high = load / 2, want_stream = 1;

    for (int i = 1; i + 1 < argc; i += 2) {
        if (!strcmp(argv[i], "--task")) task = argv[i + 1];
        else if (!strcmp(argv[i], "--seconds")) seconds = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--load")) load = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--high")) high = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--mix")) mix_ms = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--stream")) want_stream = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--wake")) wake_by_address = !strcmp(argv[i + 1], "address");
        else if (!strcmp(argv[i], "--timer")) timer_waits_on_address = !strcmp(argv[i + 1], "address");
        else if (!strcmp(argv[i], "--out")) out = argv[i + 1];
    }
    result_file = fopen(out, "w");
    QueryPerformanceFrequency(&frequency);
    wake_event = CreateEventW(NULL, FALSE, FALSE, NULL);
    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    struct stream stream = {0};
    BOOL streaming = want_stream && !wake_by_address && open_stream(&stream);
    double period_ms = streaming ? stream.period_ms : TIMER_PERIOD_MS;
    double buffer_ms = streaming ? stream.buffer_ms : TIMER_BUFFER_MS;
    result("RESULT config task %s seconds %.0f load %d high %d mix_ms %.1f source %s wake %s timer %s processors %lu\n", task, seconds, load,
           high, mix_ms, streaming ? "stream" : "timer", wake_by_address ? "address" : "event",
           timer_waits_on_address ? "address" : "sleep", (unsigned long)info.dwNumberOfProcessors);

    int before = GetThreadPriority(GetCurrentThread());
    HANDLE registration = NULL;
    DWORD task_index = 0;
    if (strcmp(task, "none")) {
        WCHAR wide[64];
        MultiByteToWideChar(CP_ACP, 0, task, -1, wide, 64);
        registration = AvSetMmThreadCharacteristicsW(wide, &task_index);
        if (!registration) result("RESULT register: FAILED error %lu\n", (unsigned long)GetLastError());
    }
    int after = GetThreadPriority(GetCurrentThread());
    result("RESULT priority before %d after %d registered %s\n", before, after, registration ? "yes" : "no");

    for (int i = 0; i < load; i++) CloseHandle(CreateThread(NULL, 0, load_thread, (void *)(INT_PTR)THREAD_PRIORITY_NORMAL, 0, NULL));
    for (int i = 0; i < high; i++) CloseHandle(CreateThread(NULL, 0, load_thread, (void *)(INT_PTR)THREAD_PRIORITY_HIGHEST, 0, NULL));

    if (streaming) {
        fill(&stream);
        IAudioClient_Start(stream.client);
    } else {
        CloseHandle(CreateThread(NULL, 0, timer_thread, NULL, 0, NULL));
    }

    double *intervals = malloc(MAX_WAKES * sizeof(*intervals));
    double *timer_late = malloc(MAX_WAKES * sizeof(*timer_late));
    double *signal_to_wake = malloc(MAX_WAKES * sizeof(*signal_to_wake));
    size_t wakes = 0, ticks = 0;
    LONG consumed = 0;
    unsigned empty = 0, timeouts = 0;
    double start = now_ms(), last = 0, warm_until = start + 2000, end = start + 2000 + seconds * 1000;
    while (now_ms() < end && wakes < MAX_WAKES) {
        if (!wait_for_tick(consumed)) { timeouts++; continue; }
        double woke = now_ms();
        BOOL measuring = woke >= warm_until;
        if (measuring && last) intervals[wakes++] = woke - last;
        LONG newest = tick_count, oldest = consumed + 1;
        if (measuring && !streaming && newest >= oldest && newest - oldest < TICK_RING) {
            timer_late[ticks] = tick_signalled_ms[oldest % TICK_RING] - tick_due_ms[oldest % TICK_RING];
            signal_to_wake[ticks++] = woke - tick_signalled_ms[oldest % TICK_RING];
        }
        consumed = newest;
        last = woke;
        spin_ms(mix_ms);
        if (streaming && fill(&stream) == 0 && measuring) empty++;
    }
    InterlockedExchange(&quit, 1);
    if (streaming) IAudioClient_Stop(stream.client);
    int at_end = GetThreadPriority(GetCurrentThread());
    if (registration) AvRevertMmThreadCharacteristics(registration);
    int reverted = GetThreadPriority(GetCurrentThread());

    if (!wakes) {
        result("RESULT wake: FAILED no wake-ups, %u timeouts\n", timeouts);
        return 1;
    }
    qsort(intervals, wakes, sizeof(*intervals), compare_doubles);
    unsigned late_periods = 0, late_buffer = 0;
    for (size_t i = 0; i < wakes; i++) {
        if (intervals[i] > 2 * period_ms) late_periods++;
        if (intervals[i] > buffer_ms) late_buffer++;
    }
    result("RESULT wake intervals %zu ms p50 %.2f p95 %.2f p99 %.2f max %.2f later_than_two_periods %u later_than_buffer %u found_empty %u timeouts %u\n",
           wakes, intervals[wakes / 2], intervals[wakes * 95 / 100], intervals[wakes * 99 / 100], intervals[wakes - 1], late_periods,
           late_buffer, empty, timeouts);
    if (ticks) {
        result_spread("timer_late", timer_late, ticks);
        result_spread("signal_to_wake", signal_to_wake, ticks);
    }
    result("RESULT priority at_end %d reverted %d\n", at_end, reverted);
    return 0;
}
