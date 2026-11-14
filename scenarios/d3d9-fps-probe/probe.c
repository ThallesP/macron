/* Macron's Direct3D 9 frame-rate probe. Draws a fixed 2D scene into a 1280x720
 * window every frame the way a 2D engine does (one small draw per sprite, a
 * texture and blend change between them), presents without vsync, and prints
 * RESULT lines. docs/checks.md describes the modes and the measure.
 *
 *   probe.exe [mix|up|vb|tex] [--sprites N] [--warmup S] [--seconds S]
 *             [--hold S] [--throttle-ms N] [--no-present]
 */
#define COBJMACROS
#include <windows.h>
#include <d3d9.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define WIDTH 1280
#define HEIGHT 720
#define SPRITE_TEXTURES 64
#define SPRITE_TEXTURE_SIZE 64
#define PANEL_WIDTH 512
#define PANEL_HEIGHT 256
#define VERTEX_BUFFER_QUADS 4096
#define FVF (D3DFVF_XYZRHW | D3DFVF_DIFFUSE | D3DFVF_TEX1)

typedef struct {
    float x, y, z, rhw;
    DWORD color;
    float u, v;
} Vertex;

typedef enum { MODE_MIX, MODE_UP, MODE_VB, MODE_TEX } Mode;

typedef struct {
    Mode mode;
    int sprites;
    double warmup_seconds;
    double measure_seconds;
    double hold_seconds;
    int throttle_ms;
    BOOL present;
} Options;

typedef struct {
    IDirect3DDevice9 *device;
    Options options;
    IDirect3DTexture9 *sprite_textures[SPRITE_TEXTURES];
    IDirect3DTexture9 *backdrop;
    IDirect3DTexture9 *overlay;
    IDirect3DTexture9 *panel;
    BOOL panel_is_dynamic;
    IDirect3DTexture9 *scene_target;
    IDirect3DSurface9 *scene_surface;
    IDirect3DSurface9 *back_buffer;
    IDirect3DVertexBuffer9 *vertex_buffer;
    UINT vertex_buffer_quad;
} Scene;

static const char *mode_names[] = { "mix", "up", "vb", "tex" };

static const DWORD palette[] = {
    0xffe8553a, 0xfff2b134, 0xff4fb477, 0xff3a86e8, 0xff9b5de5, 0xfff15bb5, 0xff00bbf9, 0xfffee440,
};

static LRESULT CALLBACK window_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp)
{
    if (msg == WM_DESTROY) {
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

static HWND open_window(void)
{
    WNDCLASSW wc;
    RECT r = { 0, 0, WIDTH, HEIGHT };
    DWORD style = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX;
    HWND hwnd;

    memset(&wc, 0, sizeof(wc));
    wc.lpfnWndProc = window_proc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursorW(NULL, (LPCWSTR)IDC_ARROW);
    wc.lpszClassName = L"MacronD3D9FpsProbe";
    RegisterClassW(&wc);
    AdjustWindowRect(&r, style, FALSE);
    hwnd = CreateWindowW(L"MacronD3D9FpsProbe", L"Macron D3D9 fps probe", style, 0, 0, r.right - r.left,
                         r.bottom - r.top, NULL, NULL, wc.hInstance, NULL);
    if (!hwnd)
        return NULL;
    ShowWindow(hwnd, SW_SHOW);
    UpdateWindow(hwnd);
    return hwnd;
}

static BOOL parse_options(int argc, char **argv, Options *o)
{
    int i, m;

    o->mode = MODE_MIX;
    o->sprites = 400;
    o->warmup_seconds = 2;
    o->measure_seconds = 10;
    o->hold_seconds = 0;
    o->throttle_ms = 0;
    o->present = TRUE;
    for (i = 1; i < argc; i++) {
        const char *a = argv[i];
        const char *value = i + 1 < argc ? argv[i + 1] : NULL;
        BOOL known_mode = FALSE;
        for (m = 0; m < 4; m++) {
            if (strcmp(a, mode_names[m]) == 0) {
                o->mode = (Mode)m;
                known_mode = TRUE;
            }
        }
        if (known_mode)
            continue;
        if (strcmp(a, "--no-present") == 0) {
            o->present = FALSE;
            continue;
        }
        if (!value)
            return FALSE;
        if (strcmp(a, "--sprites") == 0)
            o->sprites = atoi(value);
        else if (strcmp(a, "--warmup") == 0)
            o->warmup_seconds = atof(value);
        else if (strcmp(a, "--seconds") == 0)
            o->measure_seconds = atof(value);
        else if (strcmp(a, "--hold") == 0)
            o->hold_seconds = atof(value);
        else if (strcmp(a, "--throttle-ms") == 0)
            o->throttle_ms = atoi(value);
        else
            return FALSE;
        i++;
    }
    return o->sprites > 0 && o->sprites <= VERTEX_BUFFER_QUADS - 8 && o->measure_seconds > 0 && o->warmup_seconds >= 0 &&
           o->hold_seconds >= 0 && o->throttle_ms >= 0;
}

static DWORD argb(int a, int r, int g, int b)
{
    return ((DWORD)a << 24) | ((DWORD)r << 16) | ((DWORD)g << 8) | (DWORD)b;
}

static int clamp255(int v)
{
    return v < 0 ? 0 : v > 255 ? 255 : v;
}

static DWORD sprite_pixel(int shape, DWORD color, int x, int y)
{
    int c = SPRITE_TEXTURE_SIZE / 2;
    int dx = x - c, dy = y - c;
    int d2 = dx * dx + dy * dy;
    int ad = abs(dx) + abs(dy);
    int alpha = 0;

    if (shape == 0)
        alpha = (int)((29.0 * 29.0 - d2) * 0.25);
    if (shape == 1)
        alpha = d2 < 29 * 29 && d2 > 18 * 18 ? 255 : 0;
    if (shape == 2)
        alpha = (30 - ad) * 40;
    if (shape == 3)
        alpha = abs(dx) < 26 && abs(dy) < 26 ? 230 : 0;
    alpha = clamp255(alpha);
    if (shape == 3 && ((x / 8 + y / 8) & 1))
        return argb(alpha, ((color >> 16) & 0xff) / 2, ((color >> 8) & 0xff) / 2, (color & 0xff) / 2);
    return (color & 0x00ffffff) | ((DWORD)alpha << 24);
}

static DWORD backdrop_pixel(int x, int y)
{
    int band = (x / 32 + y / 32) & 1;
    return argb(255, 20 + y / 8, 30 + y / 5 + band * 12, 70 + x / 6);
}

static DWORD overlay_pixel(int x, int y)
{
    int dx = x - 64, dy = y - 64;
    return argb(clamp255((dx * dx + dy * dy) / 24), 0, 0, 0);
}

static DWORD panel_pixel(int x, int y)
{
    int row = y / 32;
    if (x < 8 || y < 8 || x >= PANEL_WIDTH - 8 || y >= PANEL_HEIGHT - 8)
        return 0xfff4f1de;
    if (y % 32 > 6 && y % 32 < 24 && x > 24 && x < 24 + (row * 53 + 120) % 440)
        return argb(255, 60 + row * 20, 200 - row * 15, 120);
    return 0xe0202838;
}

static BOOL fill_texture(IDirect3DTexture9 *texture, int width, int height, DWORD flags,
                         DWORD (*pixel)(int, int))
{
    D3DLOCKED_RECT locked;
    int x, y;

    if (FAILED(IDirect3DTexture9_LockRect(texture, 0, &locked, NULL, flags)))
        return FALSE;
    for (y = 0; y < height; y++) {
        DWORD *row = (DWORD *)((BYTE *)locked.pBits + y * locked.Pitch);
        for (x = 0; x < width; x++)
            row[x] = pixel(x, y);
    }
    IDirect3DTexture9_UnlockRect(texture, 0);
    return TRUE;
}

static BOOL fill_sprite_texture(IDirect3DTexture9 *texture, int index)
{
    D3DLOCKED_RECT locked;
    int x, y;

    if (FAILED(IDirect3DTexture9_LockRect(texture, 0, &locked, NULL, 0)))
        return FALSE;
    for (y = 0; y < SPRITE_TEXTURE_SIZE; y++) {
        DWORD *row = (DWORD *)((BYTE *)locked.pBits + y * locked.Pitch);
        for (x = 0; x < SPRITE_TEXTURE_SIZE; x++)
            row[x] = sprite_pixel(index % 4, palette[(index / 4) % 8], x, y);
    }
    IDirect3DTexture9_UnlockRect(texture, 0);
    return TRUE;
}

static BOOL create_texture(Scene *s, int width, int height, DWORD usage, D3DPOOL pool, IDirect3DTexture9 **texture)
{
    return SUCCEEDED(IDirect3DDevice9_CreateTexture(s->device, width, height, 1, usage, D3DFMT_A8R8G8B8, pool,
                                                    texture, NULL));
}

static const char *create_resources(Scene *s)
{
    int i;

    for (i = 0; i < SPRITE_TEXTURES; i++) {
        if (!create_texture(s, SPRITE_TEXTURE_SIZE, SPRITE_TEXTURE_SIZE, 0, D3DPOOL_MANAGED, &s->sprite_textures[i]) ||
            !fill_sprite_texture(s->sprite_textures[i], i))
            return "sprite textures";
    }
    if (!create_texture(s, 256, 256, 0, D3DPOOL_MANAGED, &s->backdrop) ||
        !fill_texture(s->backdrop, 256, 256, 0, backdrop_pixel))
        return "backdrop texture";
    if (!create_texture(s, 128, 128, 0, D3DPOOL_MANAGED, &s->overlay) ||
        !fill_texture(s->overlay, 128, 128, 0, overlay_pixel))
        return "overlay texture";
    s->panel_is_dynamic = create_texture(s, PANEL_WIDTH, PANEL_HEIGHT, D3DUSAGE_DYNAMIC, D3DPOOL_DEFAULT, &s->panel);
    if (!s->panel_is_dynamic && !create_texture(s, PANEL_WIDTH, PANEL_HEIGHT, 0, D3DPOOL_MANAGED, &s->panel))
        return "panel texture";
    if (!fill_texture(s->panel, PANEL_WIDTH, PANEL_HEIGHT, s->panel_is_dynamic ? D3DLOCK_DISCARD : 0, panel_pixel))
        return "panel texture lock";
    if (FAILED(IDirect3DDevice9_GetBackBuffer(s->device, 0, 0, D3DBACKBUFFER_TYPE_MONO, &s->back_buffer)))
        return "back buffer";
    if (s->options.mode == MODE_MIX) {
        if (FAILED(IDirect3DDevice9_CreateTexture(s->device, WIDTH, HEIGHT, 1, D3DUSAGE_RENDERTARGET, D3DFMT_X8R8G8B8,
                                                  D3DPOOL_DEFAULT, &s->scene_target, NULL)) ||
            FAILED(IDirect3DTexture9_GetSurfaceLevel(s->scene_target, 0, &s->scene_surface)))
            return "scene render target";
    }
    if (s->options.mode == MODE_VB) {
        if (FAILED(IDirect3DDevice9_CreateVertexBuffer(s->device, VERTEX_BUFFER_QUADS * 4 * sizeof(Vertex),
                                                       D3DUSAGE_DYNAMIC | D3DUSAGE_WRITEONLY, FVF, D3DPOOL_DEFAULT,
                                                       &s->vertex_buffer, NULL)))
            return "dynamic vertex buffer";
    }
    return NULL;
}

static void quad(Vertex *v, float x, float y, float w, float h, DWORD color)
{
    float left = x - 0.5f, top = y - 0.5f, right = x + w - 0.5f, bottom = y + h - 0.5f;
    Vertex corners[4] = {
        { left, top, 0, 1, color, 0, 0 },
        { right, top, 0, 1, color, 1, 0 },
        { left, bottom, 0, 1, color, 0, 1 },
        { right, bottom, 0, 1, color, 1, 1 },
    };
    memcpy(v, corners, sizeof(corners));
}

static HRESULT draw_quad(Scene *s, const Vertex *v)
{
    void *dst;
    DWORD flags = D3DLOCK_NOOVERWRITE;
    HRESULT hr;

    if (s->options.mode != MODE_VB)
        return IDirect3DDevice9_DrawPrimitiveUP(s->device, D3DPT_TRIANGLESTRIP, 2, v, sizeof(Vertex));
    if (s->vertex_buffer_quad == 0 || s->vertex_buffer_quad >= VERTEX_BUFFER_QUADS) {
        flags = D3DLOCK_DISCARD;
        s->vertex_buffer_quad = 0;
    }
    hr = IDirect3DVertexBuffer9_Lock(s->vertex_buffer, s->vertex_buffer_quad * 4 * sizeof(Vertex), 4 * sizeof(Vertex),
                                     &dst, flags);
    if (FAILED(hr))
        return hr;
    memcpy(dst, v, 4 * sizeof(Vertex));
    IDirect3DVertexBuffer9_Unlock(s->vertex_buffer);
    hr = IDirect3DDevice9_DrawPrimitive(s->device, D3DPT_TRIANGLESTRIP, s->vertex_buffer_quad * 4, 2);
    s->vertex_buffer_quad++;
    return hr;
}

static void set_blend(IDirect3DDevice9 *d, BOOL additive)
{
    IDirect3DDevice9_SetRenderState(d, D3DRS_ALPHABLENDENABLE, TRUE);
    IDirect3DDevice9_SetRenderState(d, D3DRS_SRCBLEND, D3DBLEND_SRCALPHA);
    IDirect3DDevice9_SetRenderState(d, D3DRS_DESTBLEND, additive ? D3DBLEND_ONE : D3DBLEND_INVSRCALPHA);
}

static void set_filter(IDirect3DDevice9 *d, D3DTEXTUREFILTERTYPE filter)
{
    IDirect3DDevice9_SetSamplerState(d, 0, D3DSAMP_MINFILTER, filter);
    IDirect3DDevice9_SetSamplerState(d, 0, D3DSAMP_MAGFILTER, filter);
}

static void set_fixed_states(IDirect3DDevice9 *d)
{
    IDirect3DDevice9_SetRenderState(d, D3DRS_LIGHTING, FALSE);
    IDirect3DDevice9_SetRenderState(d, D3DRS_CULLMODE, D3DCULL_NONE);
    IDirect3DDevice9_SetRenderState(d, D3DRS_ZENABLE, D3DZB_FALSE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_COLOROP, D3DTOP_MODULATE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_COLORARG1, D3DTA_TEXTURE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_COLORARG2, D3DTA_DIFFUSE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_ALPHAOP, D3DTOP_MODULATE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_ALPHAARG1, D3DTA_TEXTURE);
    IDirect3DDevice9_SetTextureStageState(d, 0, D3DTSS_ALPHAARG2, D3DTA_DIFFUSE);
    IDirect3DDevice9_SetSamplerState(d, 0, D3DSAMP_ADDRESSU, D3DTADDRESS_CLAMP);
    IDirect3DDevice9_SetSamplerState(d, 0, D3DSAMP_ADDRESSV, D3DTADDRESS_CLAMP);
}

static HRESULT draw_scene(Scene *s)
{
    IDirect3DDevice9 *d = s->device;
    Vertex v[4];
    HRESULT hr;
    int i;
    int columns = 25;

    if (s->options.mode == MODE_TEX || s->options.mode == MODE_MIX) {
        if (!fill_texture(s->panel, PANEL_WIDTH, PANEL_HEIGHT, s->panel_is_dynamic ? D3DLOCK_DISCARD : 0, panel_pixel))
            return E_FAIL;
    }
    if (s->options.mode == MODE_MIX)
        IDirect3DDevice9_SetRenderTarget(d, 0, s->scene_surface);
    IDirect3DDevice9_Clear(d, 0, NULL, D3DCLEAR_TARGET, 0xff000000, 1.0f, 0);
    IDirect3DDevice9_BeginScene(d);
    IDirect3DDevice9_SetFVF(d, FVF);
    if (s->options.mode == MODE_VB)
        IDirect3DDevice9_SetStreamSource(d, 0, s->vertex_buffer, 0, sizeof(Vertex));
    set_fixed_states(d);

    IDirect3DDevice9_SetRenderState(d, D3DRS_ALPHABLENDENABLE, FALSE);
    set_filter(d, D3DTEXF_LINEAR);
    IDirect3DDevice9_SetTexture(d, 0, (IDirect3DBaseTexture9 *)s->backdrop);
    quad(v, 0, 0, WIDTH, HEIGHT, 0xffffffff);
    hr = draw_quad(s, v);

    for (i = 0; i < s->options.sprites && SUCCEEDED(hr); i++) {
        int column = i % columns, row = (i / columns) % 16;
        float size = 40.0f + (float)((i * 37) % 5) * 12.0f;
        float x = 26.0f + column * 50.0f + (float)((i / (columns * 16)) * 13) - size / 2 + 12;
        float y = 40.0f + row * 42.0f + (float)((i / (columns * 16)) * 9) - size / 2 + 6;
        DWORD color = i % 3 == 0 ? 0xb0ffffff : 0xffffffff;
        set_blend(d, i % 5 == 4);
        set_filter(d, i % 2 ? D3DTEXF_LINEAR : D3DTEXF_POINT);
        IDirect3DDevice9_SetTexture(d, 0, (IDirect3DBaseTexture9 *)s->sprite_textures[(i * 7) % SPRITE_TEXTURES]);
        quad(v, x, y, size, size, color);
        hr = draw_quad(s, v);
    }

    if (SUCCEEDED(hr)) {
        set_blend(d, FALSE);
        set_filter(d, D3DTEXF_POINT);
        IDirect3DDevice9_SetTexture(d, 0, (IDirect3DBaseTexture9 *)s->panel);
        quad(v, 740, 440, PANEL_WIDTH, PANEL_HEIGHT, 0xffffffff);
        hr = draw_quad(s, v);
    }
    if (SUCCEEDED(hr)) {
        set_filter(d, D3DTEXF_LINEAR);
        IDirect3DDevice9_SetTexture(d, 0, (IDirect3DBaseTexture9 *)s->overlay);
        quad(v, 0, 0, WIDTH, HEIGHT, 0xffffffff);
        hr = draw_quad(s, v);
    }
    if (SUCCEEDED(hr) && s->options.mode == MODE_MIX) {
        IDirect3DDevice9_SetRenderTarget(d, 0, s->back_buffer);
        IDirect3DDevice9_SetRenderState(d, D3DRS_ALPHABLENDENABLE, FALSE);
        set_filter(d, D3DTEXF_POINT);
        IDirect3DDevice9_SetTexture(d, 0, (IDirect3DBaseTexture9 *)s->scene_target);
        quad(v, 0, 0, WIDTH, HEIGHT, 0xffffffff);
        hr = draw_quad(s, v);
    }
    IDirect3DDevice9_EndScene(d);
    s->vertex_buffer_quad = 0;
    return hr;
}

static int compare_floats(const void *a, const void *b)
{
    float x = *(const float *)a, y = *(const float *)b;
    return (x > y) - (x < y);
}

static float percentile(const float *sorted, size_t n, double q)
{
    size_t rank = (size_t)(q * (double)n + 0.999999);
    if (rank < 1)
        rank = 1;
    if (rank > n)
        rank = n;
    return sorted[rank - 1];
}

static IDirect3DDevice9 *create_device(IDirect3D9 *d3d, HWND hwnd, const char **processing)
{
    D3DPRESENT_PARAMETERS pp;
    IDirect3DDevice9 *device = NULL;
    HRESULT hr;

    memset(&pp, 0, sizeof(pp));
    pp.BackBufferWidth = WIDTH;
    pp.BackBufferHeight = HEIGHT;
    pp.BackBufferFormat = D3DFMT_UNKNOWN;
    pp.BackBufferCount = 1;
    pp.SwapEffect = D3DSWAPEFFECT_DISCARD;
    pp.hDeviceWindow = hwnd;
    pp.Windowed = TRUE;
    pp.PresentationInterval = D3DPRESENT_INTERVAL_IMMEDIATE;
    *processing = "hardware";
    hr = IDirect3D9_CreateDevice(d3d, D3DADAPTER_DEFAULT, D3DDEVTYPE_HAL, hwnd, D3DCREATE_HARDWARE_VERTEXPROCESSING, &pp,
                                 &device);
    if (SUCCEEDED(hr))
        return device;
    printf("hardware vertex processing failed (hr=0x%08lx), trying software\n", (unsigned long)hr);
    *processing = "software";
    hr = IDirect3D9_CreateDevice(d3d, D3DADAPTER_DEFAULT, D3DDEVTYPE_HAL, hwnd, D3DCREATE_SOFTWARE_VERTEXPROCESSING, &pp,
                                 &device);
    if (SUCCEEDED(hr))
        return device;
    printf("RESULT d3d9 device: FAILED (CreateDevice hr=0x%08lx)\n", (unsigned long)hr);
    return NULL;
}

static BOOL pump_messages(void)
{
    MSG msg;
    while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
        if (msg.message == WM_QUIT)
            return FALSE;
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    return TRUE;
}

int main(int argc, char **argv)
{
    Scene s;
    IDirect3D9 *d3d;
    D3DADAPTER_IDENTIFIER9 adapter;
    LARGE_INTEGER frequency, now, started, previous;
    const char *processing, *failed;
    float *frame_ms = NULL, *sorted;
    size_t frames = 0, capacity = 0, warmup_frames = 0;
    double elapsed_seconds, measured_seconds;
    HWND hwnd;
    HRESULT hr;
    int phase = 0;

    setvbuf(stdout, NULL, _IONBF, 0);
    memset(&s, 0, sizeof(s));
    if (!parse_options(argc, argv, &s.options)) {
        printf("usage: probe.exe [mix|up|vb|tex] [--sprites N] [--warmup S] [--seconds S] [--hold S] [--throttle-ms N] "
               "[--no-present]\n");
        return 2;
    }
    hwnd = open_window();
    if (!hwnd) {
        printf("RESULT window: FAILED (CreateWindow error %lu)\n", GetLastError());
        return 1;
    }
    printf("RESULT window: OK\n");

    d3d = Direct3DCreate9(D3D_SDK_VERSION);
    if (!d3d) {
        printf("RESULT d3d9 device: FAILED (Direct3DCreate9 returned NULL)\n");
        return 1;
    }
    if (SUCCEEDED(IDirect3D9_GetAdapterIdentifier(d3d, D3DADAPTER_DEFAULT, 0, &adapter)))
        printf("RESULT adapter %s (vendor 0x%04lx, device 0x%04lx)\n", adapter.Description,
               (unsigned long)adapter.VendorId, (unsigned long)adapter.DeviceId);
    s.device = create_device(d3d, hwnd, &processing);
    if (!s.device)
        return 1;
    printf("RESULT d3d9 device: OK (%s vertex processing)\n", processing);
    failed = create_resources(&s);
    if (failed) {
        printf("RESULT d3d9 resources: FAILED (%s)\n", failed);
        return 1;
    }
    printf("RESULT config mode %s sprites %d bits %d warmup_s %.1f measure_s %.1f throttle_ms %d present %s "
           "dynamic_panel %s\n",
           mode_names[s.options.mode], s.options.sprites, (int)(sizeof(void *) * 8), s.options.warmup_seconds,
           s.options.measure_seconds, s.options.throttle_ms, s.options.present ? "yes" : "no",
           s.panel_is_dynamic ? "yes" : "no");

    QueryPerformanceFrequency(&frequency);
    QueryPerformanceCounter(&started);
    previous = started;
    for (;;) {
        if (!pump_messages())
            return 0;
        hr = draw_scene(&s);
        if (SUCCEEDED(hr) && s.options.present)
            hr = IDirect3DDevice9_Present(s.device, NULL, NULL, NULL, NULL);
        if (FAILED(hr)) {
            printf("RESULT present: FAILED (hr=0x%08lx after %lu frames)\n", (unsigned long)hr,
                   (unsigned long)(warmup_frames + frames));
            return 1;
        }
        if (s.options.throttle_ms > 0)
            Sleep(s.options.throttle_ms);
        QueryPerformanceCounter(&now);
        elapsed_seconds = (double)(now.QuadPart - started.QuadPart) / (double)frequency.QuadPart;

        if (phase == 0) {
            if (++warmup_frames == 60)
                printf("RESULT present: OK (60 frames in %.0f ms)\n", elapsed_seconds * 1000.0);
            if (elapsed_seconds >= s.options.warmup_seconds && warmup_frames >= 60) {
                phase = 1;
                started = now;
                previous = now;
                printf("RESULT measuring\n");
            }
            continue;
        }
        if (phase == 1) {
            if (frames == capacity) {
                capacity = capacity ? capacity * 2 : 4096;
                frame_ms = realloc(frame_ms, capacity * sizeof(float));
                if (!frame_ms) {
                    printf("RESULT measure: FAILED (out of memory)\n");
                    return 1;
                }
            }
            frame_ms[frames++] = (float)((double)(now.QuadPart - previous.QuadPart) * 1000.0 / (double)frequency.QuadPart);
            previous = now;
            if (elapsed_seconds < s.options.measure_seconds)
                continue;
            measured_seconds = elapsed_seconds;
            sorted = malloc(frames * sizeof(float));
            if (!sorted) {
                printf("RESULT measure: FAILED (out of memory)\n");
                return 1;
            }
            memcpy(sorted, frame_ms, frames * sizeof(float));
            qsort(sorted, frames, sizeof(float), compare_floats);
            printf("RESULT fps %.2f p50_ms %.3f p95_ms %.3f p99_ms %.3f frames %lu\n", (double)frames / measured_seconds,
                   percentile(sorted, frames, 0.50), percentile(sorted, frames, 0.95),
                   percentile(sorted, frames, 0.99), (unsigned long)frames);
            free(sorted);
            phase = 2;
            started = now;
            continue;
        }
        if (elapsed_seconds >= s.options.hold_seconds)
            return 0;
    }
}
