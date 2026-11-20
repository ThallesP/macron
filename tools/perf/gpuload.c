/* A fixed GPU load in Direct3D 11, the twin of gpuload.m (the same load in
 * native Metal), so that the GPU time a frame of the two on one Mac shows
 * what DXMT's translation costs on the GPU.
 *
 *   gpuload.exe [--mode fill|draws|mix] [--size WxH] [--seconds S]
 *               [--passes P] [--draws D] [--sync N] [--work MS]
 *               [--latency N] [--borderless 1] [--out FILE]
 *
 * fill:  P full-screen passes a frame into a WxH RGBA16F texture, each
 *        sampling the previous one and doing 16 rounds of multiply-add and
 *        sine per pixel (the shader below, the same math as gpuload.m's),
 *        then one pass that draws the result into the back buffer.
 * draws: D small quads a frame drawn straight into the back buffer, each
 *        with its own constants written through Map(WRITE_DISCARD), the
 *        way Direct3D 11 games feed per-object constants.
 * mix:   both.
 *
 * --borderless 1 makes the window what a game's "borderless window" is: no
 * frame, at the top left, the size Windows says the screen has, with a back
 * buffer of that size. The RESULT screen line then says what Windows sees
 * (the screen's size, the display's current mode and how many modes it
 * lists, the window's client size), and every click in the window is a
 * RESULT click line with its position: together with a screenshot they show
 * whether a picture smaller than the screen (Wine patch 0017,
 * MACRON_RENDER_HEIGHT) fills it and whether the mouse lands where it points.
 *
 * It presents with sync interval --sync (default 0, as fast as it can;
 * 1 waits for the display, as gpuload.m's --sync 1 does), warms up for 2
 * seconds, measures for S seconds (default 20), and quits by itself. RESULT
 * lines go to stdout and to the --out file (default gpuload.txt), because a
 * program started through Macron's steam.exe has no stdout in macron's log.
 * The GPU time a frame comes from Apple's Metal HUD (MTL_HUD_ENABLED=1,
 * MTL_HUD_LOG_ENABLED=1), as for gpuload.m.
 *
 * It also measures how far the program runs ahead of the GPU, which is what
 * a player feels as input lag when the GPU is the limit. Every frame ends
 * with an event query, and at the start of every frame (where a game reads
 * its input) the program counts the earlier frames whose query has not
 * finished: the frames queued between this input and the screen. RESULT
 * queue has their average and maximum, and the time from a frame's start
 * until its query is seen finished (checked once a frame, so up to one
 * frame late). --work MS spends MS milliseconds of CPU at the start of each
 * frame, as a game's own work does; --latency N calls
 * IDXGIDevice1::SetMaximumFrameLatency(N).
 *
 * Build: x86_64-w64-mingw32-gcc -O2 -o gpuload.exe gpuload.c -ld3d11 -ld3dcompiler_47 -ldxgi -ldxguid -luuid */
#define COBJMACROS
#include <windows.h>
#include <d3d11.h>
#include <d3dcompiler.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static FILE *result_file;

static void result(const char *format, ...)
{
    va_list args;
    va_start(args, format);
    vprintf(format, args);
    va_end(args);
    if (result_file) {
        va_start(args, format);
        vfprintf(result_file, format, args);
        va_end(args);
        fflush(result_file);
    }
    fflush(stdout);
}

static const char shaders[] =
    "Texture2D t : register(t0);\n"
    "SamplerState s : register(s0);\n"
    "cbuffer K : register(b0) { float4 k; };\n"
    "struct V { float4 pos : SV_Position; float2 uv : TEXCOORD0; };\n"
    "V vs_full(uint id : SV_VertexID) {\n"
    "  V v; v.uv = float2((id << 1) & 2, id & 2);\n"
    "  v.pos = float4(v.uv * float2(2, -2) + float2(-1, 1), 0, 1); return v; }\n"
    "float4 ps_fill(V v) : SV_Target {\n"
    "  float4 c = t.Sample(s, v.uv);\n"
    "  [unroll] for (int i = 0; i < 16; i++) c = c * k.x + sin(c.yzwx + v.uv.xyxy) * k.y;\n"
    "  return c; }\n"
    "float4 ps_show(V v) : SV_Target { return saturate(abs(t.Sample(s, v.uv))); }\n"
    "cbuffer Q : register(b1) { float4 q; };\n"
    "float4 vs_quad(uint id : SV_VertexID) : SV_Position {\n"
    "  float2 corner = float2(id & 1, id >> 1);\n"
    "  return float4(q.xy + corner * q.z, 0, 1); }\n"
    "float4 ps_quad(float4 pos : SV_Position) : SV_Target { return float4(q.w, 1 - q.w, 0.5, 1); }\n";

static ID3DBlob *compile(const char *entry, const char *target)
{
    ID3DBlob *blob = NULL, *errors = NULL;
    if (FAILED(D3DCompile(shaders, sizeof(shaders) - 1, "gpuload", NULL, NULL, entry, target, D3DCOMPILE_OPTIMIZATION_LEVEL3, 0, &blob, &errors))) {
        result("RESULT shader %s: FAILED %s\n", entry, errors ? (const char *)ID3D10Blob_GetBufferPointer(errors) : "");
        exit(1);
    }
    return blob;
}

static LRESULT CALLBACK window_proc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp)
{
    if (msg == WM_LBUTTONDOWN) {
        POINT cursor = {0, 0};
        GetCursorPos(&cursor);
        result("RESULT click at %d,%d in the window, cursor at %ld,%ld on the screen\n", (int)(short)LOWORD(lp), (int)(short)HIWORD(lp),
               cursor.x, cursor.y);
    }
    if (msg == WM_CLOSE) {
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcA(hwnd, msg, wp, lp);
}

#define QUEUE_SLOTS 32

static double seconds_between(LARGE_INTEGER from, LARGE_INTEGER to, LARGE_INTEGER freq)
{
    return (double)(to.QuadPart - from.QuadPart) / freq.QuadPart;
}

static int compare_double(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return x < y ? -1 : x > y;
}

int main(int argc, char **argv)
{
    const char *mode = "mix", *out = "gpuload.txt";
    unsigned width = 1600, height = 900, passes = 8, draws = 4000, sync = 0, latency = 0, borderless = 0;
    double seconds = 20, work_ms = 0;
    for (int i = 1; i + 1 < argc; i += 2) {
        if (!strcmp(argv[i], "--mode")) mode = argv[i + 1];
        else if (!strcmp(argv[i], "--size")) sscanf(argv[i + 1], "%ux%u", &width, &height);
        else if (!strcmp(argv[i], "--seconds")) seconds = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--passes")) passes = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--draws")) draws = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--sync")) sync = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--work")) work_ms = atof(argv[i + 1]);
        else if (!strcmp(argv[i], "--latency")) latency = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--borderless")) borderless = atoi(argv[i + 1]);
        else if (!strcmp(argv[i], "--out")) out = argv[i + 1];
    }
    int fill = strcmp(mode, "draws") != 0, quads = strcmp(mode, "fill") != 0;
    result_file = fopen(out, "w");
    if (borderless) {
        width = GetSystemMetrics(SM_CXSCREEN);
        height = GetSystemMetrics(SM_CYSCREEN);
    }
    result("RESULT config api d3d11 mode %s size %ux%u passes %u draws %u sync %u work_ms %.1f latency %u\n", mode, width, height,
           fill ? passes : 0, quads ? draws : 0, sync, work_ms, latency);

    WNDCLASSA wc = {0};
    wc.lpfnWndProc = window_proc;
    wc.hInstance = GetModuleHandleA(NULL);
    wc.lpszClassName = "gpuload";
    wc.hCursor = LoadCursorA(NULL, (LPCSTR)IDC_ARROW);
    RegisterClassA(&wc);
    RECT rect = {0, 0, (LONG)width, (LONG)height};
    DWORD style = borderless ? WS_POPUP | WS_VISIBLE : WS_OVERLAPPEDWINDOW | WS_VISIBLE;
    AdjustWindowRect(&rect, style, FALSE);
    HWND hwnd = CreateWindowA("gpuload", "Macron gpuload (Direct3D 11)", style, borderless ? 0 : 40, borderless ? 0 : 40,
                              rect.right - rect.left, rect.bottom - rect.top, NULL, NULL, wc.hInstance, NULL);
    if (borderless) {
        DEVMODEA current = {0}, listed = {0};
        RECT client = {0};
        unsigned modes = 0;
        current.dmSize = listed.dmSize = sizeof(current);
        EnumDisplaySettingsA(NULL, ENUM_CURRENT_SETTINGS, &current);
        while (EnumDisplaySettingsA(NULL, modes, &listed)) modes++;
        GetClientRect(hwnd, &client);
        SetForegroundWindow(hwnd);
        result("RESULT screen %dx%d, current mode %lux%lu at %lu Hz, %u modes listed, client %ldx%ld\n", GetSystemMetrics(SM_CXSCREEN),
               GetSystemMetrics(SM_CYSCREEN), (unsigned long)current.dmPelsWidth, (unsigned long)current.dmPelsHeight,
               (unsigned long)current.dmDisplayFrequency, modes, client.right, client.bottom);
    }

    DXGI_SWAP_CHAIN_DESC sd = {0};
    sd.BufferCount = 2;
    sd.BufferDesc.Width = width;
    sd.BufferDesc.Height = height;
    sd.BufferDesc.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
    sd.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
    sd.OutputWindow = hwnd;
    sd.SampleDesc.Count = 1;
    sd.Windowed = TRUE;
    sd.SwapEffect = DXGI_SWAP_EFFECT_FLIP_DISCARD;
    IDXGISwapChain *swap;
    ID3D11Device *dev;
    ID3D11DeviceContext *ctx;
    D3D_FEATURE_LEVEL level = D3D_FEATURE_LEVEL_11_0;
    HRESULT hr = D3D11CreateDeviceAndSwapChain(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, 0, &level, 1, D3D11_SDK_VERSION, &sd, &swap, &dev, NULL, &ctx);
    if (FAILED(hr)) {
        result("RESULT device: FAILED hr=0x%08lx\n", (unsigned long)hr);
        return 1;
    }
    if (latency) {
        IDXGIDevice1 *dxgi_dev;
        hr = ID3D11Device_QueryInterface(dev, &IID_IDXGIDevice1, (void **)&dxgi_dev);
        if (SUCCEEDED(hr)) hr = IDXGIDevice1_SetMaximumFrameLatency(dxgi_dev, latency);
        result("RESULT SetMaximumFrameLatency(%u): hr=0x%08lx\n", latency, (unsigned long)hr);
    }

    ID3DBlob *b;
    ID3D11VertexShader *vs_full, *vs_quad;
    ID3D11PixelShader *ps_fill, *ps_show, *ps_quad;
    b = compile("vs_full", "vs_5_0"); ID3D11Device_CreateVertexShader(dev, ID3D10Blob_GetBufferPointer(b), ID3D10Blob_GetBufferSize(b), NULL, &vs_full);
    b = compile("vs_quad", "vs_5_0"); ID3D11Device_CreateVertexShader(dev, ID3D10Blob_GetBufferPointer(b), ID3D10Blob_GetBufferSize(b), NULL, &vs_quad);
    b = compile("ps_fill", "ps_5_0"); ID3D11Device_CreatePixelShader(dev, ID3D10Blob_GetBufferPointer(b), ID3D10Blob_GetBufferSize(b), NULL, &ps_fill);
    b = compile("ps_show", "ps_5_0"); ID3D11Device_CreatePixelShader(dev, ID3D10Blob_GetBufferPointer(b), ID3D10Blob_GetBufferSize(b), NULL, &ps_show);
    b = compile("ps_quad", "ps_5_0"); ID3D11Device_CreatePixelShader(dev, ID3D10Blob_GetBufferPointer(b), ID3D10Blob_GetBufferSize(b), NULL, &ps_quad);

    ID3D11Texture2D *tex[2];
    ID3D11RenderTargetView *rtv[2];
    ID3D11ShaderResourceView *srv[2];
    D3D11_TEXTURE2D_DESC td = {0};
    td.Width = width;
    td.Height = height;
    td.MipLevels = 1;
    td.ArraySize = 1;
    td.Format = DXGI_FORMAT_R16G16B16A16_FLOAT;
    td.SampleDesc.Count = 1;
    td.BindFlags = D3D11_BIND_RENDER_TARGET | D3D11_BIND_SHADER_RESOURCE;
    for (int i = 0; i < 2; i++) {
        ID3D11Device_CreateTexture2D(dev, &td, NULL, &tex[i]);
        ID3D11Device_CreateRenderTargetView(dev, (ID3D11Resource *)tex[i], NULL, &rtv[i]);
        ID3D11Device_CreateShaderResourceView(dev, (ID3D11Resource *)tex[i], NULL, &srv[i]);
    }
    ID3D11Texture2D *back;
    ID3D11RenderTargetView *back_rtv;
    IDXGISwapChain_GetBuffer(swap, 0, &IID_ID3D11Texture2D, (void **)&back);
    ID3D11Device_CreateRenderTargetView(dev, (ID3D11Resource *)back, NULL, &back_rtv);

    D3D11_BUFFER_DESC bd = {16, D3D11_USAGE_DYNAMIC, D3D11_BIND_CONSTANT_BUFFER, D3D11_CPU_ACCESS_WRITE, 0, 0};
    ID3D11Buffer *k_buf, *q_buf;
    float k[4] = {0.999f, 0.01f, 0, 0};
    D3D11_SUBRESOURCE_DATA kd = {k, 0, 0};
    D3D11_BUFFER_DESC kbd = {16, D3D11_USAGE_IMMUTABLE, D3D11_BIND_CONSTANT_BUFFER, 0, 0, 0};
    ID3D11Device_CreateBuffer(dev, &kbd, &kd, &k_buf);
    ID3D11Device_CreateBuffer(dev, &bd, NULL, &q_buf);
    D3D11_SAMPLER_DESC smp = {D3D11_FILTER_MIN_MAG_MIP_LINEAR, D3D11_TEXTURE_ADDRESS_CLAMP, D3D11_TEXTURE_ADDRESS_CLAMP, D3D11_TEXTURE_ADDRESS_CLAMP, 0, 1, D3D11_COMPARISON_NEVER, {0}, 0, D3D11_FLOAT32_MAX};
    ID3D11SamplerState *sampler;
    ID3D11Device_CreateSamplerState(dev, &smp, &sampler);

    D3D11_VIEWPORT vp = {0, 0, (float)width, (float)height, 0, 1};
    ID3D11DeviceContext_RSSetViewports(ctx, 1, &vp);
    ID3D11DeviceContext_PSSetSamplers(ctx, 0, 1, &sampler);
    ID3D11DeviceContext_PSSetConstantBuffers(ctx, 0, 1, &k_buf);
    ID3D11DeviceContext_VSSetConstantBuffers(ctx, 1, 1, &q_buf);
    ID3D11DeviceContext_PSSetConstantBuffers(ctx, 1, 1, &q_buf);
    float seed[4] = {0.3f, 0.6f, 0.9f, 1};
    ID3D11DeviceContext_ClearRenderTargetView(ctx, rtv[0], seed);

    LARGE_INTEGER freq, start, prev, now;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&start);
    prev = start;
    size_t cap = 1 << 20, n = 0, done_n = 0;
    double *intervals = malloc(cap * sizeof(double));
    double *done_ms = malloc(cap * sizeof(double));
    ID3D11Query *queries[QUEUE_SLOTS] = {0};
    LARGE_INTEGER query_start[QUEUE_SLOTS];
    int query_pending[QUEUE_SLOTS] = {0};
    D3D11_QUERY_DESC query_desc = {D3D11_QUERY_EVENT, 0};
    for (int i = 0; i < QUEUE_SLOTS; i++) ID3D11Device_CreateQuery(dev, &query_desc, &queries[i]);
    unsigned long long behind_sum = 0, behind_frames = 0, lost_queries = 0;
    unsigned behind_max = 0;
    int measuring = 0;
    LARGE_INTEGER measure_start = start;
    unsigned long long frame = 0;
    ID3D11ShaderResourceView *null_srv = NULL;
    MSG msg;
    for (;;) {
        while (PeekMessageA(&msg, NULL, 0, 0, PM_REMOVE)) {
            if (msg.message == WM_QUIT) goto done;
            DispatchMessageA(&msg);
        }
        LARGE_INTEGER frame_start;
        QueryPerformanceCounter(&frame_start);
        unsigned behind = 0;
        for (int i = 0; i < QUEUE_SLOTS; i++) {
            BOOL finished = FALSE;
            if (!query_pending[i]) continue;
            if (ID3D11DeviceContext_GetData(ctx, (ID3D11Asynchronous *)queries[i], &finished, sizeof(finished), D3D11_ASYNC_GETDATA_DONOTFLUSH) != S_OK) {
                behind++;
                continue;
            }
            query_pending[i] = 0;
            if (measuring && done_n < cap) done_ms[done_n++] = 1000.0 * seconds_between(query_start[i], frame_start, freq);
        }
        if (measuring) {
            behind_sum += behind;
            behind_frames++;
            if (behind > behind_max) behind_max = behind;
        }
        if (work_ms > 0) {
            LARGE_INTEGER t;
            do QueryPerformanceCounter(&t); while (1000.0 * seconds_between(frame_start, t, freq) < work_ms);
        }
        int cur = 0;
        if (fill) {
            ID3D11DeviceContext_IASetPrimitiveTopology(ctx, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
            ID3D11DeviceContext_VSSetShader(ctx, vs_full, NULL, 0);
            ID3D11DeviceContext_PSSetShader(ctx, ps_fill, NULL, 0);
            for (unsigned p = 0; p < passes; p++) {
                ID3D11DeviceContext_PSSetShaderResources(ctx, 0, 1, &null_srv);
                ID3D11DeviceContext_OMSetRenderTargets(ctx, 1, &rtv[1 - cur], NULL);
                ID3D11DeviceContext_PSSetShaderResources(ctx, 0, 1, &srv[cur]);
                ID3D11DeviceContext_Draw(ctx, 3, 0);
                cur = 1 - cur;
            }
            ID3D11DeviceContext_OMSetRenderTargets(ctx, 1, &back_rtv, NULL);
            ID3D11DeviceContext_PSSetShader(ctx, ps_show, NULL, 0);
            ID3D11DeviceContext_PSSetShaderResources(ctx, 0, 1, &srv[cur]);
            ID3D11DeviceContext_Draw(ctx, 3, 0);
            ID3D11DeviceContext_PSSetShaderResources(ctx, 0, 1, &null_srv);
        } else {
            float clear[4] = {0.1f, 0.1f, 0.15f, 1};
            ID3D11DeviceContext_OMSetRenderTargets(ctx, 1, &back_rtv, NULL);
            ID3D11DeviceContext_ClearRenderTargetView(ctx, back_rtv, clear);
        }
        if (quads) {
            ID3D11DeviceContext_IASetPrimitiveTopology(ctx, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP);
            ID3D11DeviceContext_VSSetShader(ctx, vs_quad, NULL, 0);
            ID3D11DeviceContext_PSSetShader(ctx, ps_quad, NULL, 0);
            for (unsigned d = 0; d < draws; d++) {
                D3D11_MAPPED_SUBRESOURCE m;
                ID3D11DeviceContext_Map(ctx, (ID3D11Resource *)q_buf, 0, D3D11_MAP_WRITE_DISCARD, 0, &m);
                float *q = m.pData;
                q[0] = -1 + 2.0f * (float)((d * 37 + frame) % 100) / 100;
                q[1] = -1 + 2.0f * (float)((d * 61) % 100) / 100;
                q[2] = 0.02f;
                q[3] = (float)(d % 16) / 16;
                ID3D11DeviceContext_Unmap(ctx, (ID3D11Resource *)q_buf, 0);
                ID3D11DeviceContext_Draw(ctx, 4, 0);
            }
        }
        int slot = (int)(frame % QUEUE_SLOTS);
        if (query_pending[slot]) lost_queries++;
        ID3D11DeviceContext_End(ctx, (ID3D11Asynchronous *)queries[slot]);
        query_pending[slot] = 1;
        query_start[slot] = frame_start;
        hr = IDXGISwapChain_Present(swap, sync, 0);
        if (FAILED(hr)) {
            result("RESULT present: FAILED hr=0x%08lx at frame %llu\n", (unsigned long)hr, frame);
            return 1;
        }
        frame++;
        QueryPerformanceCounter(&now);
        double since_start = (double)(now.QuadPart - start.QuadPart) / freq.QuadPart;
        if (!measuring && since_start >= 2) {
            measuring = 1;
            measure_start = now;
        } else if (measuring) {
            if (n < cap) intervals[n++] = 1000.0 * (now.QuadPart - prev.QuadPart) / freq.QuadPart;
            if ((double)(now.QuadPart - measure_start.QuadPart) / freq.QuadPart >= seconds) break;
        }
        prev = now;
    }
done:;
    double total = (double)(now.QuadPart - measure_start.QuadPart) / freq.QuadPart;
    qsort(intervals, n, sizeof(double), compare_double);
    if (n)
        result("RESULT fps %.2f frames %zu interval_ms p50 %.3f p95 %.3f p99 %.3f\n", n / total, n, intervals[n / 2],
               intervals[n * 95 / 100], intervals[n * 99 / 100]);
    qsort(done_ms, done_n, sizeof(double), compare_double);
    if (behind_frames && done_n)
        result("RESULT queue frames_behind avg %.2f max %u start_to_gpu_done_ms p50 %.2f p95 %.2f p99 %.2f lost %llu\n",
               (double)behind_sum / behind_frames, behind_max, done_ms[done_n / 2], done_ms[done_n * 95 / 100],
               done_ms[done_n * 99 / 100], lost_queries);
    return 0;
}
