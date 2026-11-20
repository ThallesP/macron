/* How regularly Wine sets a shared event-driven stream's event while the
 * stream plays without being fed, while it is stopped, after Reset, and
 * playing again: the time between events in each state.
 *
 *   evcadence.exe [FILE] [SECONDS a state]
 *
 * Writes RESULT lines to FILE (default evcadence.txt). Nothing is heard.
 * With Wine patch 0016 the event follows CoreAudio's callback; without it,
 * a timer of the driver's own.
 *
 * Build: x86_64-w64-mingw32-gcc -O2 -o evcadence.exe evcadence.c -lole32 */
#define COBJMACROS
#include <initguid.h>
#include <windows.h>
#include <audioclient.h>
#include <mmdeviceapi.h>
#include <stdio.h>
#include <stdlib.h>

static LARGE_INTEGER frequency;
static double now_ms(void) { LARGE_INTEGER c; QueryPerformanceCounter(&c); return c.QuadPart * 1000.0 / frequency.QuadPart; }
static int cmp(const void *a, const void *b) { double x = *(const double *)a, y = *(const double *)b; return (x > y) - (x < y); }

static void watch(FILE *out, const char *name, HANDLE event, double seconds)
{
    static double gaps[100000];
    size_t n = 0;
    unsigned timeouts = 0;
    double end = now_ms() + seconds * 1000, last = now_ms();
    while (now_ms() < end && n < 100000) {
        DWORD r = WaitForSingleObject(event, 500);
        double t = now_ms();
        if (r != WAIT_OBJECT_0) { timeouts++; }
        gaps[n++] = t - last;
        last = t;
    }
    qsort(gaps, n, sizeof(*gaps), cmp);
    unsigned over40 = 0, over70 = 0, over120 = 0;
    for (size_t i = 0; i < n; i++) { if (gaps[i] > 40) over40++; if (gaps[i] > 70) over70++; if (gaps[i] > 120) over120++; }
    fprintf(out, "RESULT %s: %u events in %.0f s, ms between p50 %.2f p95 %.2f p99 %.2f max %.2f, over 40 ms %u, over 70 ms %u, over 120 ms %u, 500 ms timeouts %u\n",
            name, (unsigned)n, seconds, gaps[n / 2], gaps[n * 95 / 100], gaps[n * 99 / 100], gaps[n - 1], over40, over70, over120, timeouts);
    fflush(out);
}

int main(int argc, char **argv)
{
    FILE *out = fopen(argc > 1 ? argv[1] : "evcadence.txt", "w");
    double seconds = argc > 2 ? atof(argv[2]) : 10;
    IMMDeviceEnumerator *devices = NULL; IMMDevice *device = NULL; IAudioClient *client = NULL; WAVEFORMATEX *format = NULL;
    QueryPerformanceFrequency(&frequency);
    CoInitializeEx(NULL, COINIT_MULTITHREADED);
    CoCreateInstance(&CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL, &IID_IMMDeviceEnumerator, (void **)&devices);
    IMMDeviceEnumerator_GetDefaultAudioEndpoint(devices, eRender, eConsole, &device);
    IMMDevice_Activate(device, &IID_IAudioClient, CLSCTX_ALL, NULL, (void **)&client);
    IAudioClient_GetMixFormat(client, &format);
    HRESULT hr = IAudioClient_Initialize(client, AUDCLNT_SHAREMODE_SHARED, AUDCLNT_STREAMFLAGS_EVENTCALLBACK, 5000000, 0, format, NULL);
    HANDLE event = CreateEventW(NULL, FALSE, FALSE, NULL);
    IAudioClient_SetEventHandle(client, event);
    fprintf(out, "RESULT init hr=0x%08lx rate %lu\n", (unsigned long)hr, (unsigned long)format->nSamplesPerSec);
    IAudioClient_Start(client);
    watch(out, "playing, never fed", event, seconds);
    IAudioClient_Stop(client);
    watch(out, "stopped", event, seconds);
    IAudioClient_Reset(client);
    watch(out, "stopped after Reset", event, seconds);
    IAudioClient_Start(client);
    watch(out, "playing again, never fed", event, seconds);
    IAudioClient_Release(client);
    fclose(out);
    return 0;
}
