/* How long a mouse movement takes from macOS to a game's message loop under
 * Wine, apart from the game's own frame time.
 *
 * Usage: inputlag.exe [--seconds S] [--frame MS] [--load N] [--high N]
 *                     [--sink 0|1] [--thread 0|1] [--legacy 0|1] [--out FILE]
 *
 * Opens a topmost window, registers for raw mouse input as a game does, and
 * reads its messages the way a game's main thread does: everything queued,
 * once a frame. For every WM_INPUT and WM_MOUSEMOVE it takes the time since
 * the message's own time stamp, which Wine's Mac driver sets from the macOS
 * event's (1 ms steps). With --frame 0 the thread blocks in GetMessage and
 * handles each movement as it arrives, so the number is the path alone:
 * Cocoa's thread, Wine's event queue, wineserver, the message. With --frame
 * MS it spins MS milliseconds between reads, as a game computing a frame
 * does, and the number includes the wait for the next read (half a frame on
 * average), which Windows has too.
 *
 * Raw input reaches a program only while one of its windows is the
 * foreground window, so the RESULT foreground line says how often it was
 * (checked at every read); --sink 1 registers with RIDEV_INPUTSINK, which
 * asks for it in the background too.
 *
 * --thread 1 reads raw input as SDL 3 does: on a thread of its own, with a
 * message-only window as the target, while the main thread still reads its
 * own messages once a frame. Two times are then taken for every movement:
 * when the raw thread got it, and when a frame's read first had it (the main
 * thread looks at what the raw thread has collected right after reading its
 * messages, as a game polls SDL's events after pumping them). On Windows the
 * raw thread gets a movement when the mouse makes it. Under Wine's Mac
 * driver a movement enters Windows only when the thread that owns the window
 * under the pointer reads its messages, so the raw thread gets it during the
 * main thread's read, and the frame may not see it until its next read:
 * the RESULT line "between reads" counts the movements that reached the raw
 * thread only after the read that brought them in was over, which a game
 * sees a frame late. The raw thread is time-critical, as SDL's is. --legacy 0
 * registers with RIDEV_NOLEGACY, so the main thread gets no WM_MOUSEMOVE and
 * its read is as short as it can be.
 *
 * The load is N threads that never sleep at normal priority and N at
 * THREAD_PRIORITY_HIGHEST (default none), a game's workers to the scheduler.
 *
 * Something has to move the mouse over the window meanwhile: a hand, or on a
 * sandbox tools/perf/mousegen.c. RESULT lines go to stdout and the --out
 * file (default inputlag.txt).
 *
 * Build: x86_64-w64-mingw32-gcc -O2 -o inputlag.exe inputlag.c -luser32 -lgdi32 */
#include <windows.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_EVENTS 400000

static FILE *result_file;
static volatile LONG quit;
static LARGE_INTEGER frequency;
static int *raw_lag, *move_lag;
static size_t raw_count, move_count;
static long raw_distance;
static BOOL measuring;
static unsigned raw_messages, raw_failed, raw_other, other_messages;

/* --thread 1: what the raw input thread has collected, for the main thread. */
#define RING 65536
static DWORD ring_event_tick[RING];
static volatile LONG ring_count;
static int *frame_lag;
static size_t frame_count;
static volatile LONG raw_thread_ready;
static HWND raw_window;
static unsigned in_read_arrivals, between_read_arrivals;
static volatile LONG main_is_reading;
static int legacy = 1;

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

static LRESULT CALLBACK window_proc(HWND window, UINT message, WPARAM wparam, LPARAM lparam)
{
    if (message == WM_DESTROY) PostQuitMessage(0);
    return DefWindowProcW(window, message, wparam, lparam);
}

static void take(const MSG *message)
{
    int lag = (int)(GetTickCount() - message->time);

    if (message->message == WM_INPUT) {
        RAWINPUT input;
        UINT size = sizeof(input);
        raw_messages++;
        if (GetRawInputData((HRAWINPUT)message->lParam, RID_INPUT, &input, &size, sizeof(RAWINPUTHEADER)) == (UINT)-1) {
            raw_failed++;
        } else if (input.header.dwType != RIM_TYPEMOUSE) {
            raw_other++;
        } else if (measuring && raw_count < MAX_EVENTS) {
            raw_distance += labs(input.data.mouse.lLastX) + labs(input.data.mouse.lLastY);
            raw_lag[raw_count++] = lag;
        }
    } else if (message->message == WM_MOUSEMOVE) {
        if (measuring && move_count < MAX_EVENTS) move_lag[move_count++] = lag;
    } else if (message->message != WM_TIMER) {
        other_messages++;
    }
}

static void take_raw(const MSG *message)
{
    RAWINPUT input;
    UINT size = sizeof(input);
    int lag = (int)(GetTickCount() - message->time);

    raw_messages++;
    if (GetRawInputData((HRAWINPUT)message->lParam, RID_INPUT, &input, &size, sizeof(RAWINPUTHEADER)) == (UINT)-1) {
        raw_failed++;
        return;
    }
    if (input.header.dwType != RIM_TYPEMOUSE) {
        raw_other++;
        return;
    }
    if (!measuring || raw_count >= MAX_EVENTS) return;
    raw_distance += labs(input.data.mouse.lLastX) + labs(input.data.mouse.lLastY);
    raw_lag[raw_count++] = lag;
    if (main_is_reading) in_read_arrivals++;
    else between_read_arrivals++;
    LONG slot = ring_count;
    ring_event_tick[slot % RING] = message->time;
    InterlockedExchange(&ring_count, slot + 1);
}

static DWORD WINAPI raw_thread(void *sink)
{
    WNDCLASSW class = {0};
    class.lpfnWndProc = DefWindowProcW;
    class.hInstance = GetModuleHandleW(NULL);
    class.lpszClassName = L"MacronInputLagRaw";
    RegisterClassW(&class);
    raw_window = CreateWindowExW(0, class.lpszClassName, L"raw", 0, 0, 0, 0, 0, HWND_MESSAGE, NULL, class.hInstance, NULL);
    RAWINPUTDEVICE device = {1, 2, (sink ? RIDEV_INPUTSINK : 0) | (legacy ? 0 : RIDEV_NOLEGACY), raw_window};
    SetThreadPriority(GetCurrentThread(), THREAD_PRIORITY_TIME_CRITICAL);
    InterlockedExchange(&raw_thread_ready, raw_window && RegisterRawInputDevices(&device, 1, sizeof(device)) ? 1 : -1);

    MSG message;
    while (!quit && GetMessageW(&message, NULL, 0, 0) > 0) {
        if (message.message == WM_INPUT) take_raw(&message);
        DispatchMessageW(&message);
    }
    return 0;
}

static int compare_ints(const void *a, const void *b)
{
    return *(const int *)a - *(const int *)b;
}

static void result_lag(const char *name, int *lag, size_t count, double seconds)
{
    if (!count) {
        result("RESULT %s: none received\n", name);
        return;
    }
    qsort(lag, count, sizeof(*lag), compare_ints);
    size_t over_2 = 0, over_8 = 0, over_16 = 0;
    for (size_t i = 0; i < count; i++) {
        if (lag[i] > 2) over_2++;
        if (lag[i] > 8) over_8++;
        if (lag[i] > 16) over_16++;
    }
    result("RESULT %s: %zu in %.1f s (%.0f a second), ms since the event p50 %d p95 %d p99 %d max %d min %d, over 2 ms %zu, over 8 ms %zu, over 16 ms %zu\n",
           name, count, seconds, count / seconds, lag[count / 2], lag[count * 95 / 100], lag[count * 99 / 100], lag[count - 1],
           lag[0], over_2, over_8, over_16);
}

int main(int argc, char **argv)
{
    const char *out = "inputlag.txt";
    double seconds = 15, frame_ms = 0;
    int load = 0, high = 0, sink = 0, thread = 0;

    for (int i = 1; i + 1 < argc; i += 2) {
        if (!strcmp(argv[i], "--seconds")) seconds = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--frame")) frame_ms = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--load")) load = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--high")) high = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--sink")) sink = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--thread")) thread = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--legacy")) legacy = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--out")) out = argv[i + 1];
    }
    result_file = fopen(out, "w");
    QueryPerformanceFrequency(&frequency);
    raw_lag = malloc(MAX_EVENTS * sizeof(*raw_lag));
    move_lag = malloc(MAX_EVENTS * sizeof(*move_lag));
    frame_lag = malloc(MAX_EVENTS * sizeof(*frame_lag));

    WNDCLASSW class = {0};
    class.lpfnWndProc = window_proc;
    class.hInstance = GetModuleHandleW(NULL);
    class.hCursor = LoadCursorW(NULL, (LPCWSTR)IDC_ARROW);
    class.hbrBackground = (HBRUSH)GetStockObject(DKGRAY_BRUSH);
    class.lpszClassName = L"MacronInputLag";
    RegisterClassW(&class);
    HWND window = CreateWindowExW(WS_EX_TOPMOST, class.lpszClassName, L"Macron input lag", WS_POPUP | WS_VISIBLE, 100, 100, 800, 600,
                                  NULL, NULL, class.hInstance, NULL);
    if (!window) {
        result("RESULT window: FAILED error %lu\n", (unsigned long)GetLastError());
        return 1;
    }
    SetForegroundWindow(window);
    BOOL registered;
    HANDLE raw_handle = NULL;
    if (thread) {
        raw_handle = CreateThread(NULL, 0, raw_thread, (void *)(INT_PTR)sink, 0, NULL);
        while (!raw_thread_ready) Sleep(1);
        registered = raw_thread_ready > 0;
    } else {
        RAWINPUTDEVICE device = {1, 2, (sink ? RIDEV_INPUTSINK : 0) | (legacy ? 0 : RIDEV_NOLEGACY), window};
        registered = RegisterRawInputDevices(&device, 1, sizeof(device));
    }
    result("RESULT config seconds %.0f frame_ms %.1f load %d high %d raw_input %s%s%s\n", seconds, frame_ms, load, high,
           registered ? "registered" : "FAILED", sink ? " (input sink)" : "", thread ? " on a thread of its own" : "");
    if (!legacy) result("RESULT config no legacy mouse messages\n");

    for (int i = 0; i < load; i++) CloseHandle(CreateThread(NULL, 0, load_thread, (void *)(INT_PTR)THREAD_PRIORITY_NORMAL, 0, NULL));
    for (int i = 0; i < high; i++) CloseHandle(CreateThread(NULL, 0, load_thread, (void *)(INT_PTR)THREAD_PRIORITY_HIGHEST, 0, NULL));

    double start = now_ms(), warm_until = start + 2000, end = warm_until + seconds * 1000;
    unsigned frames = 0, reads = 0, foreground = 0;
    LONG seen = 0;
    MSG message;
    SetTimer(window, 1, 100, NULL); /* so a blocking read notices the end */
    while (now_ms() < end) {
        measuring = now_ms() >= warm_until;
        if (measuring) {
            reads++;
            if (GetForegroundWindow() == window) foreground++;
        }
        if (frame_ms > 0) {
            InterlockedExchange(&main_is_reading, 1);
            while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) {
                take(&message);
                DispatchMessageW(&message);
            }
            InterlockedExchange(&main_is_reading, 0);
            /* the game looks at what the raw thread has for it */
            LONG collected = ring_count;
            DWORD now = GetTickCount();
            for (; seen != collected; seen++)
                if (frame_count < MAX_EVENTS) frame_lag[frame_count++] = (int)(now - ring_event_tick[seen % RING]);
            if (measuring) frames++;
            spin_ms(frame_ms);
        } else {
            if (GetMessageW(&message, NULL, 0, 0) <= 0) break;
            take(&message);
            DispatchMessageW(&message);
        }
    }
    InterlockedExchange(&quit, 1);

    result_lag(thread ? "raw input, when its thread got it" : "raw input", raw_lag, raw_count, seconds);
    if (thread && frame_ms > 0) {
        result_lag("raw input, when a frame's read had it", frame_lag, frame_count, seconds);
        result("RESULT the raw thread got %u while the main thread was reading its messages and %u between reads\n", in_read_arrivals,
               between_read_arrivals);
    }
    result_lag("mouse move", move_lag, move_count, seconds);
    result("RESULT raw distance %ld counts, frames %u\n", raw_distance, frames);
    result("RESULT foreground at %u of %u reads; WM_INPUT %u (%u unreadable, %u not a mouse), other messages %u\n", foreground, reads,
           raw_messages, raw_failed, raw_other, other_messages);
    if (raw_handle) {
        PostMessageW(raw_window, WM_NULL, 0, 0);
        WaitForSingleObject(raw_handle, 1000);
    }
    DestroyWindow(window);
    return 0;
}
