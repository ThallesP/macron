/* startup.exe [seconds]: prints the Unix time with milliseconds at which its
 * main() runs, shows a window for the given seconds (default 3), and exits. */
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static LRESULT CALLBACK proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp)
{
    if (msg == WM_DESTROY) { PostQuitMessage(0); return 0; }
    return DefWindowProcA(hwnd, msg, wp, lp);
}

int main(int argc, char **argv)
{
    FILETIME ft;
    ULARGE_INTEGER t;
    int seconds = argc > 1 ? atoi(argv[1]) : 3;
    WNDCLASSA wc = {0};
    HWND hwnd;
    MSG msg;
    DWORD end;

    GetSystemTimeAsFileTime(&ft);
    t.LowPart = ft.dwLowDateTime;
    t.HighPart = ft.dwHighDateTime;
    printf("STARTUP main %.3f\n", (double)(t.QuadPart - 116444736000000000ULL) / 1e7);
    fflush(stdout);
    {
        /* through steam.exe the program's output does not reach macron's log */
        char path[MAX_PATH], *slash;
        FILE *f;
        GetModuleFileNameA(NULL, path, MAX_PATH);
        if ((slash = strrchr(path, '\\'))) strcpy(slash + 1, "startup-main.txt");
        if ((f = fopen(path, "a"))) { fprintf(f, "STARTUP main %.3f\n", (double)(t.QuadPart - 116444736000000000ULL) / 1e7); fclose(f); }
    }

    wc.lpfnWndProc = proc;
    wc.hInstance = GetModuleHandleA(NULL);
    wc.lpszClassName = "startup";
    RegisterClassA(&wc);
    hwnd = CreateWindowA("startup", "Startup probe", WS_OVERLAPPEDWINDOW | WS_VISIBLE, 100, 100, 400, 300, 0, 0, wc.hInstance, 0);
    end = GetTickCount() + seconds * 1000;
    while (GetTickCount() < end)
    {
        while (PeekMessageA(&msg, 0, 0, 0, PM_REMOVE)) { TranslateMessage(&msg); DispatchMessageA(&msg); }
        Sleep(10);
    }
    DestroyWindow(hwnd);
    printf("STARTUP exit\n");
    return 0;
}
