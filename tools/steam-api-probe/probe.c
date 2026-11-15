#include <windows.h>
#include <stdio.h>

typedef int(__cdecl *init_flat_fn)(char *message);
typedef BOOL(__cdecl *init_fn)(void);
typedef void *(__cdecl *accessor_fn)(void);
typedef unsigned int(__cdecl *app_id_fn)(void *utils);
typedef void(__cdecl *shutdown_fn)(void);

static const char *init_results[] = {"OK", "FailedGeneric", "NoSteamClient", "VersionMismatch"};

static void print_registry(const char *name)
{
    char value[MAX_PATH];
    DWORD size = sizeof(value), number, number_size = sizeof(number);

    if (!RegGetValueA(HKEY_CURRENT_USER, "Software\\Valve\\Steam\\ActiveProcess", name, RRF_RT_REG_SZ, NULL, value, &size))
        printf("RESULT registry %s: %s\n", name, value);
    else if (!RegGetValueA(HKEY_CURRENT_USER, "Software\\Valve\\Steam\\ActiveProcess", name, RRF_RT_REG_DWORD, NULL, &number, &number_size))
        printf("RESULT registry %s: %lu\n", name, number);
    else
        printf("RESULT registry %s: (not set)\n", name);
}

static void print_steamclient_modules(void)
{
    const char *names[] = {"steamclient64.dll", "steamclient.dll", "lsteamclient.dll"};
    char path[MAX_PATH];
    unsigned int i;

    for (i = 0; i < sizeof(names) / sizeof(*names); i++)
    {
        HMODULE module = GetModuleHandleA(names[i]);
        if (!module) continue;
        GetModuleFileNameA(module, path, sizeof(path));
        printf("RESULT module %s: loaded from %s\n", names[i], path);
    }
}

static void *interface_of(HMODULE steam_api, const char *accessor)
{
    accessor_fn get = (accessor_fn)GetProcAddress(steam_api, accessor);
    void *iface;

    if (!get)
    {
        printf("RESULT %s: not exported\n", accessor);
        return NULL;
    }
    iface = get();
    printf("RESULT %s: %s\n", accessor, iface ? "returned an interface" : "returned NULL");
    return iface;
}

int main(void)
{
    const char *dll = sizeof(void *) == 8 ? "steam_api64.dll" : "steam_api.dll";
    char message[1024] = "";
    init_flat_fn init_flat;
    HMODULE steam_api;
    void *utils;
    BOOL ok;

    setvbuf(stdout, NULL, _IONBF, 0);
    printf("RESULT bits: %u\n", (unsigned int)(sizeof(void *) * 8));
    printf("RESULT env SteamAppId: %s\n", getenv("SteamAppId") ? getenv("SteamAppId") : "(not set)");
    printf("RESULT env SteamGameId: %s\n", getenv("SteamGameId") ? getenv("SteamGameId") : "(not set)");
    print_registry("pid");
    print_registry(sizeof(void *) == 8 ? "SteamClientDll64" : "SteamClientDll");

    if (!(steam_api = LoadLibraryA(dll)))
    {
        printf("RESULT load %s: FAIL, error %lu\n", dll, GetLastError());
        return 2;
    }
    printf("RESULT load %s: OK\n", dll);

    if ((init_flat = (init_flat_fn)GetProcAddress(steam_api, "SteamAPI_InitFlat")))
    {
        int result = init_flat(message);
        printf("RESULT SteamAPI_InitFlat: %d (%s), message \"%s\"\n", result,
               result >= 0 && result < 4 ? init_results[result] : "unknown", message);
        ok = result == 0;
    }
    else
    {
        ok = ((init_fn)GetProcAddress(steam_api, "SteamAPI_Init"))();
        printf("RESULT SteamAPI_Init: %s\n", ok ? "true" : "false");
    }
    print_steamclient_modules();
    if (!ok)
        return 1;

    if ((utils = interface_of(steam_api, "SteamAPI_SteamUtils_v011")))
        printf("RESULT app id: %u\n", ((app_id_fn)GetProcAddress(steam_api, "SteamAPI_ISteamUtils_GetAppID"))(utils));
    interface_of(steam_api, "SteamAPI_SteamUserStats_v013");
    interface_of(steam_api, "SteamAPI_SteamRemoteStorage_v016");
    ((shutdown_fn)GetProcAddress(steam_api, "SteamAPI_Shutdown"))();
    return 0;
}
