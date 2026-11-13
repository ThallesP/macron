/* Probe: a Windows Direct3D 11 program that creates a GPU device, clears a
 * texture, then compiles shaders and draws, reading the pixels back each time.
 * Run it under Wine to see whether the DirectX translation layer works. */
#define COBJMACROS
#include <windows.h>
#include <d3d11.h>
#include <d3dcompiler.h>
#include <dxgi.h>
#include <stdio.h>
#include <string.h>

static const char vs_src[] =
    "float4 main(uint id : SV_VertexID) : SV_Position {\n"
    "    float2 p = float2(id == 2 ? 3.0 : -1.0, id == 1 ? 3.0 : -1.0);\n"
    "    return float4(p, 0.0, 1.0);\n"
    "}\n";
static const char ps_src[] =
    "float4 main() : SV_Target { return float4(0.25, 0.5, 0.75, 1.0); }\n";

static ID3D11Device *dev;
static ID3D11DeviceContext *ctx;
static ID3D11Texture2D *rt, *staging;

static int read_pixel(unsigned char out[4])
{
    D3D11_MAPPED_SUBRESOURCE m;
    HRESULT hr;
    ID3D11DeviceContext_CopyResource(ctx, (ID3D11Resource *)staging, (ID3D11Resource *)rt);
    hr = ID3D11DeviceContext_Map(ctx, (ID3D11Resource *)staging, 0, D3D11_MAP_READ, 0, &m);
    if (FAILED(hr)) {
        printf("Map failed hr=0x%08lx\n", (unsigned long)hr);
        return 0;
    }
    /* centre pixel */
    memcpy(out, (unsigned char *)m.pData + 32 * m.RowPitch + 32 * 4, 4);
    ID3D11DeviceContext_Unmap(ctx, (ID3D11Resource *)staging, 0);
    return 1;
}

static int close_to(int a, int b) { return a - b <= 2 && b - a <= 2; }

int main(void)
{
    D3D_FEATURE_LEVEL want[] = { D3D_FEATURE_LEVEL_11_0 }, got = 0;
    D3D11_TEXTURE2D_DESC td;
    D3D11_RASTERIZER_DESC rd;
    D3D11_VIEWPORT vp = { 0, 0, 64, 64, 0, 1 };
    ID3D11RenderTargetView *rtv = NULL;
    ID3D11RasterizerState *rs = NULL;
    ID3D11VertexShader *vs = NULL;
    ID3D11PixelShader *ps = NULL;
    ID3DBlob *vsb = NULL, *psb = NULL, *err = NULL;
    IDXGIDevice *dxgidev = NULL;
    IDXGIAdapter *adapter = NULL;
    DXGI_ADAPTER_DESC ad;
    float red[4] = { 1.0f, 0.0f, 0.0f, 1.0f };
    unsigned char px[4];
    HRESULT hr;

    setvbuf(stdout, NULL, _IONBF, 0);

    hr = D3D11CreateDevice(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, 0, want, 1,
                           D3D11_SDK_VERSION, &dev, &got, &ctx);
    if (FAILED(hr)) {
        printf("RESULT d3d11 device: FAILED (D3D11CreateDevice hr=0x%08lx)\n", (unsigned long)hr);
        return 1;
    }
    printf("feature level: 0x%x\n", (unsigned)got);
    if (SUCCEEDED(ID3D11Device_QueryInterface(dev, &IID_IDXGIDevice, (void **)&dxgidev)) &&
        SUCCEEDED(IDXGIDevice_GetAdapter(dxgidev, &adapter)) &&
        SUCCEEDED(IDXGIAdapter_GetDesc(adapter, &ad)))
        printf("adapter: %ls (vendor 0x%04x, device 0x%04x, %u MB)\n", ad.Description,
               ad.VendorId, ad.DeviceId, (unsigned)(ad.DedicatedVideoMemory >> 20));
    printf("RESULT d3d11 device: OK\n");

    memset(&td, 0, sizeof(td));
    td.Width = 64;
    td.Height = 64;
    td.MipLevels = 1;
    td.ArraySize = 1;
    td.Format = DXGI_FORMAT_R8G8B8A8_UNORM;
    td.SampleDesc.Count = 1;
    td.Usage = D3D11_USAGE_DEFAULT;
    td.BindFlags = D3D11_BIND_RENDER_TARGET;
    hr = ID3D11Device_CreateTexture2D(dev, &td, NULL, &rt);
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreateRenderTargetView(dev, (ID3D11Resource *)rt, NULL, &rtv);
    td.Usage = D3D11_USAGE_STAGING;
    td.BindFlags = 0;
    td.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreateTexture2D(dev, &td, NULL, &staging);
    if (FAILED(hr)) {
        printf("RESULT d3d11 clear: FAILED (creating textures hr=0x%08lx)\n", (unsigned long)hr);
        return 1;
    }

    /* Stage 1: clear and read back. */
    ID3D11DeviceContext_ClearRenderTargetView(ctx, rtv, red);
    if (!read_pixel(px)) {
        printf("RESULT d3d11 clear: FAILED (readback)\n");
        return 1;
    }
    printf("clear pixel rgba=%u,%u,%u,%u (expected 255,0,0,255)\n", px[0], px[1], px[2], px[3]);
    if (!(close_to(px[0], 255) && close_to(px[1], 0) && close_to(px[2], 0))) {
        printf("RESULT d3d11 clear: FAILED (wrong pixel)\n");
        return 1;
    }
    printf("RESULT d3d11 clear: OK\n");

    /* Stage 2: compile shaders, draw a full-screen triangle, read back. */
    hr = D3DCompile(vs_src, sizeof(vs_src) - 1, "vs", NULL, NULL, "main", "vs_5_0", 0, 0, &vsb, &err);
    if (FAILED(hr)) {
        printf("RESULT d3d11 draw: FAILED (vertex shader compile hr=0x%08lx: %s)\n", (unsigned long)hr,
               err ? (const char *)ID3D10Blob_GetBufferPointer(err) : "no message");
        return 1;
    }
    hr = D3DCompile(ps_src, sizeof(ps_src) - 1, "ps", NULL, NULL, "main", "ps_5_0", 0, 0, &psb, &err);
    if (FAILED(hr)) {
        printf("RESULT d3d11 draw: FAILED (pixel shader compile hr=0x%08lx: %s)\n", (unsigned long)hr,
               err ? (const char *)ID3D10Blob_GetBufferPointer(err) : "no message");
        return 1;
    }
    hr = ID3D11Device_CreateVertexShader(dev, ID3D10Blob_GetBufferPointer(vsb),
                                         ID3D10Blob_GetBufferSize(vsb), NULL, &vs);
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreatePixelShader(dev, ID3D10Blob_GetBufferPointer(psb),
                                            ID3D10Blob_GetBufferSize(psb), NULL, &ps);
    memset(&rd, 0, sizeof(rd));
    rd.FillMode = D3D11_FILL_SOLID;
    rd.CullMode = D3D11_CULL_NONE;
    rd.DepthClipEnable = TRUE;
    if (SUCCEEDED(hr))
        hr = ID3D11Device_CreateRasterizerState(dev, &rd, &rs);
    if (FAILED(hr)) {
        printf("RESULT d3d11 draw: FAILED (creating shaders/state hr=0x%08lx)\n", (unsigned long)hr);
        return 1;
    }

    ID3D11DeviceContext_OMSetRenderTargets(ctx, 1, &rtv, NULL);
    ID3D11DeviceContext_RSSetViewports(ctx, 1, &vp);
    ID3D11DeviceContext_RSSetState(ctx, rs);
    ID3D11DeviceContext_IASetPrimitiveTopology(ctx, D3D11_PRIMITIVE_TOPOLOGY_TRIANGLELIST);
    ID3D11DeviceContext_VSSetShader(ctx, vs, NULL, 0);
    ID3D11DeviceContext_PSSetShader(ctx, ps, NULL, 0);
    ID3D11DeviceContext_Draw(ctx, 3, 0);
    ID3D11DeviceContext_Flush(ctx);

    if (!read_pixel(px)) {
        printf("RESULT d3d11 draw: FAILED (readback)\n");
        return 1;
    }
    printf("draw pixel rgba=%u,%u,%u,%u (expected about 64,128,191,255)\n", px[0], px[1], px[2], px[3]);
    if (!(close_to(px[0], 64) && close_to(px[1], 128) && close_to(px[2], 191))) {
        printf("RESULT d3d11 draw: FAILED (wrong pixel)\n");
        return 1;
    }
    printf("RESULT d3d11 draw: OK\n");
    return 0;
}
