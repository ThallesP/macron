// What a game that offers DLSS sees under Macron's DXMT: the adapter's
// vendor, device and revision, whether NVAPI initialises, where the registry
// sends NVIDIA's NGX for its core and whether Windows trusts the signature of
// the nvngx.dll there (NGX loads it only then; this probe loads it directly,
// so it must ask), whether NGX says super resolution is available, and
// whether a DLSS feature can be created (DXMT backs it with MetalFX's
// temporal scaler, so that last step tells whether this Mac's GPU has one).
//
// DXMT only answers as NVIDIA when DXMT_ENABLE_NVEXT=1; without it the
// vendor is Apple (0x106b) and the NVIDIA DLLs do not initialise.
//
// The RESULT lines also go to the file named by the first argument (default
// nvprobe.txt in the current folder), because a program started through
// Macron's steam.exe has no stdout that reaches macron's log.
//
// Build: x86_64-w64-mingw32-g++ -O2 -o nvprobe.exe nvprobe.cpp -ld3d11 -ldxgi -lwintrust
// The NGX parameter class is declared the way DXMT 0.80 declares it
// (src/nvngx/nvngx.hpp), so this probe has to be built with a MinGW
// compiler, as DXMT is, for the vtable to match.
#include <windows.h>
#include <d3d11.h>
#include <dxgi.h>
#include <softpub.h>
#include <wintrust.h>
#include <cstdarg>
#include <cstdio>
#include <cwchar>

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
}

class NGXParameter {
public:
    virtual void Set(const char *name, void *value) = 0;
    virtual void Set(const char *name, void *d3d12_resource, int) = 0;
    virtual void Set(const char *name, ID3D11Resource *value) = 0;
    virtual void Set(const char *name, int value) = 0;
    virtual void Set(const char *name, unsigned int value) = 0;
    virtual void Set(const char *name, double value) = 0;
    virtual void Set(const char *name, float value) = 0;
    virtual void Set(const char *name, unsigned long long value) = 0;
    virtual int Get(const char *name, void **out) const = 0;
    virtual int Get(const char *name, void **d3d12_resource, int) const = 0;
    virtual int Get(const char *name, double *out) const = 0;
    virtual int Get(const char *name, unsigned int *out) const = 0;
    virtual int Get(const char *name, int *out) const = 0;
    virtual int Get(const char *name, ID3D11Resource **out) const = 0;
    virtual int Get(const char *name, float *out) const = 0;
    virtual int Get(const char *name, unsigned long long *out) const = 0;
    virtual void Reset() = 0;
};

static const int NGX_OK = 1;
static const unsigned NGX_FEATURE_SUPERSAMPLING = 1;

typedef void *(__cdecl *nvapi_query_fn)(unsigned id);
typedef int(__cdecl *nvapi_init_fn)(void);
typedef int(__cdecl *ngx_init_fn)(unsigned long long, const wchar_t *, ID3D11Device *, const void *, unsigned);
typedef int(__cdecl *ngx_caps_fn)(NGXParameter **);
typedef int(__cdecl *ngx_alloc_fn)(NGXParameter **);
typedef int(__cdecl *ngx_create_fn)(ID3D11DeviceContext *, unsigned, NGXParameter *, unsigned **);

// The folder a registry value names for NGX's core, and whether the nvngx.dll in it is signed by someone this
// machine trusts: 0 is yes, 0x800b0100 is no signature, 0x800b0109 a signature whose root is not trusted.
static void report_ngx_core(const wchar_t *key, const wchar_t *value)
{
    wchar_t folder[MAX_PATH] = L"";
    DWORD size = sizeof(folder);
    LSTATUS status = RegGetValueW(HKEY_LOCAL_MACHINE, key, value, RRF_RT_REG_SZ, nullptr, folder, &size);
    if (status != ERROR_SUCCESS) {
        result("RESULT registry %ls: not set (error %ld)\n", value, (long)status);
        return;
    }
    wchar_t path[MAX_PATH];
    swprintf(path, MAX_PATH, L"%ls\\nvngx.dll", folder);
    WINTRUST_FILE_INFO file = {};
    file.cbStruct = sizeof(file);
    file.pcwszFilePath = path;
    WINTRUST_DATA trust = {};
    trust.cbStruct = sizeof(trust);
    trust.dwUIChoice = WTD_UI_NONE;
    trust.fdwRevocationChecks = WTD_REVOKE_NONE;
    trust.dwUnionChoice = WTD_CHOICE_FILE;
    trust.pFile = &file;
    GUID action = WINTRUST_ACTION_GENERIC_VERIFY_V2;
    LONG trusted = WinVerifyTrust(nullptr, &action, &trust);
    result("RESULT registry %ls: %ls, signature of nvngx.dll there: %s (0x%08lx)\n", value, folder,
           trusted == 0 ? "trusted" : "NOT trusted", (unsigned long)trusted);
}

int main(int argc, char **argv)
{
    result_file = fopen(argc > 1 ? argv[1] : "nvprobe.txt", "w");
    const char *nvext = getenv("DXMT_ENABLE_NVEXT");
    result("RESULT DXMT_ENABLE_NVEXT=%s\n", nvext ? nvext : "(unset)");

    ID3D11Device *device = nullptr;
    ID3D11DeviceContext *context = nullptr;
    HRESULT hr = D3D11CreateDevice(nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, 0, nullptr, 0, D3D11_SDK_VERSION, &device,
                                   nullptr, &context);
    if (FAILED(hr)) {
        result("RESULT d3d11 device: FAILED hr=0x%08lx\n", (unsigned long)hr);
        return 1;
    }
    IDXGIDevice *dxgi_device = nullptr;
    IDXGIAdapter *adapter = nullptr;
    DXGI_ADAPTER_DESC desc = {};
    if (SUCCEEDED(device->QueryInterface(__uuidof(IDXGIDevice), (void **)&dxgi_device)) &&
        SUCCEEDED(dxgi_device->GetAdapter(&adapter)) && SUCCEEDED(adapter->GetDesc(&desc)))
        result("RESULT adapter vendor 0x%04x device 0x%04x revision 0x%02x \"%ls\" dedicated %llu MB\n", desc.VendorId,
               desc.DeviceId, desc.Revision, desc.Description, (unsigned long long)desc.DedicatedVideoMemory >> 20);
    report_ngx_core(L"SYSTEM\\CurrentControlSet\\Services\\nvlddmkm\\NGXCore", L"NGXPath");
    report_ngx_core(L"SOFTWARE\\NVIDIA Corporation\\Global\\NGXCore", L"FullPath");

    HMODULE nvapi = LoadLibraryA("nvapi64.dll");
    if (!nvapi) {
        result("RESULT nvapi64.dll: not loaded (error %lu)\n", GetLastError());
    } else {
        nvapi_query_fn query = (nvapi_query_fn)GetProcAddress(nvapi, "nvapi_QueryInterface");
        nvapi_init_fn init = query ? (nvapi_init_fn)query(0x0150e828) : nullptr;
        result("RESULT NvAPI_Initialize: %s\n", init ? (init() == 0 ? "OK" : "failed") : "not found");
    }

    HMODULE ngx = LoadLibraryA("nvngx.dll");
    if (!ngx) {
        result("RESULT nvngx.dll: not loaded (error %lu)\n", GetLastError());
        return 0;
    }
    ngx_init_fn init = (ngx_init_fn)GetProcAddress(ngx, "NVSDK_NGX_D3D11_Init");
    ngx_caps_fn caps = (ngx_caps_fn)GetProcAddress(ngx, "NVSDK_NGX_D3D11_GetCapabilityParameters");
    ngx_alloc_fn alloc = (ngx_alloc_fn)GetProcAddress(ngx, "NVSDK_NGX_D3D11_AllocateParameters");
    ngx_create_fn create = (ngx_create_fn)GetProcAddress(ngx, "NVSDK_NGX_D3D11_CreateFeature");
    if (!init || !caps || !alloc || !create) {
        result("RESULT nvngx.dll: missing exports\n");
        return 0;
    }
    result("RESULT NGX init: %s\n", init(1, L"C:\\users\\Public", device, nullptr, 0x15) == NGX_OK ? "OK" : "failed");
    NGXParameter *capabilities = nullptr;
    unsigned available = 0;
    if (caps(&capabilities) == NGX_OK && capabilities)
        capabilities->Get("SuperSampling.Available", &available);
    result("RESULT NGX SuperSampling.Available=%u\n", available);

    NGXParameter *params = nullptr;
    alloc(&params);
    params->Set("Width", 1280u);
    params->Set("Height", 720u);
    params->Set("OutWidth", 1920u);
    params->Set("OutHeight", 1080u);
    params->Set("PerfQualityValue", 2u);
    unsigned *handle = nullptr;
    int created = create(context, NGX_FEATURE_SUPERSAMPLING, params, &handle);
    result("RESULT DLSS feature 1280x720->1920x1080: %s (0x%x)\n",
           created == NGX_OK ? "created (MetalFX temporal scaler)" : "not created", (unsigned)created);
    return 0;
}
