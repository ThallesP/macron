/* What Direct3D 9 says the display adapter and its driver are: the name, the
 * PCI ids and the driver's version, which is what Source 2 (Deadlock) reads
 * to decide whether a graphics driver is out of date.
 *
 *   d3d9ver.exe [--out FILE]
 *
 * Under Wine this is wined3d's answer whatever draws Direct3D 11 (DXMT has no
 * Direct3D 9), so it is where a game that is told it has an NVIDIA card
 * (Macron's `dlss` switch) gets that card's driver version from.
 *
 * The program says it knows Windows 10 (d3d9ver.manifest, linked in through
 * d3d9ver.rc), as a current game does: without that Windows, and Wine, answer
 * it as Windows 8, and the driver's version starts with 9.14 instead of the
 * 31.0 or 32.0 a game sees.
 *
 * Build: x86_64-w64-mingw32-windres d3d9ver.rc -O coff -o d3d9ver.res
 *        x86_64-w64-mingw32-gcc -O2 -o d3d9ver.exe d3d9ver.c d3d9ver.res -ld3d9 */
#include <d3d9.h>
#include <stdio.h>
#include <string.h>

int main(int argc, char **argv)
{
    FILE *out = stdout;
    for (int i = 1; i + 1 < argc; i++)
        if (!strcmp(argv[i], "--out")) out = fopen(argv[i + 1], "w");
    if (!out) return 2;

    IDirect3D9 *d3d = Direct3DCreate9(D3D_SDK_VERSION);
    if (!d3d) {
        fprintf(out, "RESULT d3d9: Direct3DCreate9 failed\n");
        return 1;
    }
    D3DADAPTER_IDENTIFIER9 id;
    HRESULT hr = IDirect3D9_GetAdapterIdentifier(d3d, D3DADAPTER_DEFAULT, 0, &id);
    if (FAILED(hr)) {
        fprintf(out, "RESULT d3d9: GetAdapterIdentifier failed %#lx\n", (unsigned long)hr);
        return 1;
    }
    fprintf(out, "RESULT d3d9: \"%s\", driver %s, vendor %04lx, device %04lx, version %u.%u.%u.%u\n", id.Description, id.Driver,
            (unsigned long)id.VendorId, (unsigned long)id.DeviceId, HIWORD(id.DriverVersion.HighPart), LOWORD(id.DriverVersion.HighPart),
            HIWORD(id.DriverVersion.LowPart), LOWORD(id.DriverVersion.LowPart));
    IDirect3D9_Release(d3d);
    return 0;
}
