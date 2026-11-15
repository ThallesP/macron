/* Macron's Direct3D 11 probe: opens a 640x480 window and draws the same
 * picture into it every frame through a DXGI swap chain, the way a game does.
 * The picture is computed per pixel in the shader, so it is the same on every
 * GPU up to rounding. Prints RESULT lines to stdout. */
#define COBJMACROS
#include <windows.h>
#include <d3d11.h>
#include <d3dcompiler.h>
#include <dxgi.h>
#include <stdio.h>
#include <string.h>

#define WIDTH 640
#define HEIGHT 480

static const char vs_src[] =
    "float4 main(uint id : SV_VertexID) : SV_Position {\n"
    "    float2 p = float2(id == 2 ? 3.0 : -1.0, id == 1 ? 3.0 : -1.0);\n"
    "    return float4(p, 0.0, 1.0);\n"
    "}\n";

static const char ps_src[] =
    "float4 main(float4 pos : SV_Position) : SV_Target {\n"
    "    float2 uv = pos.xy / float2(640.0, 480.0);\n"
    "    float3 c = lerp(float3(0.08, 0.10, 0.22), float3(0.10, 0.35, 0.45), uv.y);\n"
    "    float2 cells = floor(pos.xy / 40.0);\n"
    "    if (uv.x > 0.62 && uv.y > 0.10 && uv.y < 0.55 && fmod(cells.x + cells.y, 2.0) < 1.0) c = float3(0.85, 0.85, 0.80);\n"
    "    float d = length(pos.xy - float2(180.0, 170.0));\n"
    "    if (d < 90.0) c = float3(0.90, 0.30, 0.20);\n"
    "    if (d < 45.0) c = float3(0.98, 0.80, 0.25);\n"
    "    for (int i = 0; i < 4; i++) {\n"
    "        float top = 300.0 + i * 38.0;\n"
    "        if (pos.y > top && pos.y < top + 24.0 && pos.x > 60.0 && pos.x < 300.0 - i * 40.0) c = float3(0.20 + 0.2 * i, 0.75, 0.40);\n"
    "    }\n"
    "    if (pos.x > 400.0 && pos.x < 600.0 && pos.y > 330.0 && pos.y < 440.0 && (uint(pos.x) / 8 + uint(pos.y) / 8) % 3 == 0) c = float3(0.55, 0.35, 0.85);\n"
    "    return float4(c, 1.0);\n"
    "}\n";

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
    wc.lpszClassName = L"MacronProbe";
    RegisterClassW(&wc);
    AdjustWindowRect(&r, style, FALSE);
    hwnd = CreateWindowW(L"MacronProbe", L"Macron D3D11 probe", style, 40, 40, r.right - r.left,
                         r.bottom - r.top, NULL, NULL, wc.hInstance, NULL);
    if (hwnd) {
        ShowWindow(hwnd, SW_SHOW);
        UpdateWindow(hwnd);
    }
    return hwnd;
}

static ID3DBlob *compile(const char *src, size_t len, const char *target)
{
    ID3DBlob *blob = NULL, *err = NULL;
    HRESULT hr = D3DCompile(src, len, target, NULL, NULL, "main", target, 0, 0, &blob, &err);
    if (FAILED(hr)) {
        printf("RESULT d3d11 shaders: FAILED (%s compile hr=0x%08lx: %s)\n", target, (unsigned long)hr,
               err ? (const char *)ID3D10Blob_GetBufferPointer(err) : "no message");
        return NULL;
    }
    return blob;
}

int main(void)
{
    D3D_FEATURE_LEVEL want[] = { D3D_FEATURE_LEVEL_11_0 }, got = 0;
    DXGI_SWAP_CHAIN_DESC sd;
    D3D11_RASTERIZER_DESC rd;
    D3D11_VIEWPORT vp = { 0, 0, WIDTH, HEIGHT, 0, 1 };
    ID3D11Device *dev = NULL;
    ID3D11DeviceContext *ctx = NULL;
    IDXGISwapChain *swap = NULL;
    ID3D11Texture2D *back = NULL;
    ID3D11RenderTargetView *rtv = NULL;
    ID3D11RasterizerState *rs = NULL;
    ID3D11VertexShader *vs = NULL;
    ID3D11PixelShader *ps = NULL;
    ID3DBlob *vsb, *psb;
    IDXGIDevice *dxgidev = NULL;
    IDXGIAdapter *adapter = NULL;
    DXGI_ADAPTER_DESC ad;
    float clear[4] = { 0.0f, 0.0f, 0.0f, 1.0f };
    DWORD started;
    unsigned frames = 0;
    HWND hwnd;
    HRESULT hr;
    MSG msg;

    setvbuf(stdout, NULL, _IONBF, 0);
    hwnd = open_window();
    if (!hwnd) {
        printf("RESULT window: FAILED (CreateWindow error %lu)\n", GetLastError());
        return 1;
    }
    printf("RESULT window: OK\n");

    memset(&sd, 0, sizeof(sd));
    sd.BufferCount = 2;
    sd.BufferDesc.Width = WIDTH;
    sd.BufferDesc.Height = HEIGHT;
    sd.BufferDesc.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
    sd.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
    sd.OutputWindow = hwnd;
    sd.SampleDesc.Count = 1;
    sd.Windowed = TRUE;
    sd.SwapEffect = DXGI_SWAP_EFFECT_FLIP_DISCARD;
    hr = D3D11CreateDeviceAndSwapChain(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, 0, want, 1, D3D11_SDK_VERSION,
                                       &sd, &swap, &dev, &got, &ctx);
    if (FAILED(hr)) {
        sd.SwapEffect = DXGI_SWAP_EFFECT_DISCARD;
        sd.BufferCount = 1;
        hr = D3D11CreateDeviceAndSwapChain(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, 0, want, 1,
                                           D3D11_SDK_VERSION, &sd, &swap, &dev, &got, &ctx);
    }
    if (FAILED(hr)) {
        printf("RESULT d3d11 device: FAILED (D3D11CreateDeviceAndSwapChain hr=0x%08lx)\n", (unsigned long)hr);
        return 1;
    }
    if (SUCCEEDED(ID3D11Device_QueryInterface(dev, &IID_IDXGIDevice, (void **)&dxgidev)) &&
        SUCCEEDED(IDXGIDevice_GetAdapter(dxgidev, &adapter)) && SUCCEEDED(IDXGIAdapter_GetDesc(adapter, &ad)))
        printf("adapter: %ls (vendor 0x%04x, device 0x%04x)\n", ad.Description, ad.VendorId, ad.DeviceId);
    printf("RESULT d3d11 device: OK (feature level 0x%x, swap effect %d)\n", (unsigned)got, (int)sd.SwapEffect);

    vsb = compile(vs_src, sizeof(vs_src) - 1, "vs_5_0");
    psb = compile(ps_src, sizeof(ps_src) - 1, "ps_5_0");
    if (!vsb || !psb)
        return 1;
    hr = ID3D11Device_CreateVertexShader(dev, ID3D10Blob_GetBufferPointer(vsb), ID3D10Blob_GetBufferSize(vsb), NULL, &vs);
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreatePixelShader(dev, ID3D10Blob_GetBufferPointer(psb), ID3D10Blob_GetBufferSize(psb), NULL, &ps);
    memset(&rd, 0, sizeof(rd));
    rd.FillMode = D3D11_FILL_SOLID;
    rd.CullMode = D3D11_CULL_NONE;
    rd.DepthClipEnable = TRUE;
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreateRasterizerState(dev, &rd, &rs);
    if (SUCCEEDED(hr))
        hr = IDXGISwapChain_GetBuffer(swap, 0, &IID_ID3D11Texture2D, (void **)&back);
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreateRenderTargetView(dev, (ID3D11Resource *)back, NULL, &rtv);
    if (FAILED(hr)) {
        printf("RESULT d3d11 shaders: FAILED (creating shaders, state or views hr=0x%08lx)\n", (unsigned long)hr);
        return 1;
    }
    printf("RESULT d3d11 shaders: OK\n");

    started = GetTickCount();
    for (;;) {
        while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
            if (msg.message == WM_QUIT)
                return 0;
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        }
        ID3D11DeviceContext_OMSetRenderTargets(ctx, 1, &rtv, NULL);
        ID3D11DeviceContext_ClearRenderTargetView(ctx, rtv, clear);
        ID3D11DeviceContext_RSSetViewports(ctx, 1, &vp);
        ID3D11DeviceContext_RSSetState(ctx, rs);
        ID3D11DeviceContext_IASetPrimitiveTopology(ctx, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
        ID3D11DeviceContext_VSSetShader(ctx, vs, NULL, 0);
        ID3D11DeviceContext_PSSetShader(ctx, ps, NULL, 0);
        ID3D11DeviceContext_Draw(ctx, 3, 0);
        hr = IDXGISwapChain_Present(swap, 1, 0);
        if (FAILED(hr)) {
            printf("RESULT present: FAILED (hr=0x%08lx after %u frames)\n", (unsigned long)hr, frames);
            return 1;
        }
        if (++frames == 60)
            printf("RESULT present: OK (60 frames in %lu ms)\n", GetTickCount() - started);
        if (GetTickCount() - started > 30 * 60 * 1000)
            return 0;
    }
}
