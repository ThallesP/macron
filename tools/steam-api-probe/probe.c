#include <windows.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <string.h>

typedef int(__cdecl *init_flat_fn)(char *message);
typedef BOOL(__cdecl *init_fn)(void);
typedef void *(__cdecl *accessor_fn)(void);
typedef unsigned int(__cdecl *app_id_fn)(void *utils);
typedef void(__cdecl *shutdown_fn)(void);
typedef int(__cdecl *pipe_fn)(void);
typedef void(__cdecl *dispatch_init_fn)(void);
typedef void(__cdecl *run_frame_fn)(int pipe);
typedef bool(__cdecl *free_callback_fn)(int pipe);
typedef unsigned int(__cdecl *num_achievements_fn)(void *stats);
typedef const char *(__cdecl *achievement_name_fn)(void *stats, unsigned int index);
typedef bool(__cdecl *get_achievement_fn)(void *stats, const char *name, bool *achieved);
typedef bool(__cdecl *achievement_fn)(void *stats, const char *name);
typedef bool(__cdecl *store_stats_fn)(void *stats);
typedef bool(__cdecl *storage_bool_fn)(void *storage);
typedef bool(__cdecl *file_write_fn)(void *storage, const char *name, const void *data, int size);
typedef int(__cdecl *file_read_fn)(void *storage, const char *name, void *data, int size);
typedef bool(__cdecl *file_exists_fn)(void *storage, const char *name);

/* CallbackMsg_t, packed as the SDK packs callbacks on Windows (8 bytes). */
typedef struct {
    int user;
    int callback;
    unsigned char *param;
    int param_size;
} CallbackMsg;
typedef bool(__cdecl *next_callback_fn)(int pipe, CallbackMsg *message);

#define USER_STATS_RECEIVED 1101
#define USER_STATS_STORED 1102
#define USER_ACHIEVEMENT_STORED 1103
#define CLOUD_FILE "macron-steam-api-probe.txt"

static const char *init_results[] = {"OK", "FailedGeneric", "NoSteamClient", "VersionMismatch"};
static FILE *results;

static void report(const char *format, ...)
{
    va_list args;

    va_start(args, format);
    vprintf(format, args);
    va_end(args);
    if (!results) return;
    va_start(args, format);
    vfprintf(results, format, args);
    va_end(args);
    fflush(results);
}

static void open_results(void)
{
    char path[MAX_PATH], *slash;

    GetModuleFileNameA(NULL, path, sizeof(path));
    if (!(slash = strrchr(path, '\\'))) return;
    sprintf(slash + 1, "steam-api-probe-%u.txt", (unsigned int)(sizeof(void *) * 8));
    results = fopen(path, "w");
}

static void print_registry(const char *name)
{
    char value[MAX_PATH];
    DWORD size = sizeof(value), number, number_size = sizeof(number);

    if (!RegGetValueA(HKEY_CURRENT_USER, "Software\\Valve\\Steam\\ActiveProcess", name, RRF_RT_REG_SZ, NULL, value, &size))
        report("RESULT registry %s: %s\n", name, value);
    else if (!RegGetValueA(HKEY_CURRENT_USER, "Software\\Valve\\Steam\\ActiveProcess", name, RRF_RT_REG_DWORD, NULL, &number, &number_size))
        report("RESULT registry %s: %lu\n", name, number);
    else
        report("RESULT registry %s: (not set)\n", name);
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
        report("RESULT module %s: loaded from %s\n", names[i], path);
    }
}

static void *interface_of(HMODULE steam_api, const char *accessor)
{
    accessor_fn get = (accessor_fn)GetProcAddress(steam_api, accessor);
    void *iface;

    if (!get)
    {
        report("RESULT %s: not exported\n", accessor);
        return NULL;
    }
    iface = get();
    report("RESULT %s: %s\n", accessor, iface ? "returned an interface" : "returned NULL");
    return iface;
}

static HMODULE api;

static void *api_function(const char *name)
{
    void *f = (void *)GetProcAddress(api, name);
    if (!f) report("RESULT %s: not exported\n", name);
    return f;
}

/* Runs Steam's callbacks for up to `seconds` and returns the result code of
 * the first callback numbered `wanted` (its EResult sits after the 8-byte game
 * id in UserStatsReceived_t and UserStatsStored_t), or -1 if none came. */
static int wait_callback(int wanted, double seconds)
{
    int pipe = ((pipe_fn)api_function("SteamAPI_GetHSteamPipe"))();
    run_frame_fn run_frame = (run_frame_fn)api_function("SteamAPI_ManualDispatch_RunFrame");
    next_callback_fn next = (next_callback_fn)api_function("SteamAPI_ManualDispatch_GetNextCallback");
    free_callback_fn release = (free_callback_fn)api_function("SteamAPI_ManualDispatch_FreeLastCallback");
    DWORD end = GetTickCount() + (DWORD)(seconds * 1000);
    CallbackMsg message;
    int result = -1;

    while (result < 0 && (int)(end - GetTickCount()) > 0)
    {
        run_frame(pipe);
        while (next(pipe, &message))
        {
            if (message.callback == wanted && result < 0 && message.param_size >= 12)
                result = *(int *)(message.param + 8);
            release(pipe);
        }
        Sleep(50);
    }
    return result;
}

/* --achievement set|clear|show: the app's first achievement (by index), as
 * Steam has it, then set or cleared and stored, which Steam confirms with
 * UserStatsStored (EResult 1 is OK). */
static int achievement(void *stats, const char *action)
{
    const char *name;
    bool achieved = false;
    int stored;

    report("RESULT stats received: %d\n", wait_callback(USER_STATS_RECEIVED, 20));
    report("RESULT achievements: %u\n", ((num_achievements_fn)api_function("SteamAPI_ISteamUserStats_GetNumAchievements"))(stats));
    if (!(name = ((achievement_name_fn)api_function("SteamAPI_ISteamUserStats_GetAchievementName"))(stats, 0)) || !*name)
    {
        report("RESULT achievement: the app has none\n");
        return 1;
    }
    if (!((get_achievement_fn)api_function("SteamAPI_ISteamUserStats_GetAchievement"))(stats, name, &achieved))
    {
        report("RESULT achievement %s: GetAchievement failed\n", name);
        return 1;
    }
    report("RESULT achievement %s before: %s\n", name, achieved ? "achieved" : "not achieved");
    if (!strcmp(action, "show"))
        return 0;
    if (!((achievement_fn)api_function(!strcmp(action, "set") ? "SteamAPI_ISteamUserStats_SetAchievement"
                                                                : "SteamAPI_ISteamUserStats_ClearAchievement"))(stats, name))
    {
        report("RESULT achievement %s %s: failed\n", name, action);
        return 1;
    }
    if (!((store_stats_fn)api_function("SteamAPI_ISteamUserStats_StoreStats"))(stats))
    {
        report("RESULT StoreStats: false\n");
        return 1;
    }
    stored = wait_callback(USER_STATS_STORED, 20);
    ((get_achievement_fn)api_function("SteamAPI_ISteamUserStats_GetAchievement"))(stats, name, &achieved);
    report("RESULT achievement %s %s: stored, result %d, now %s\n", name, action, stored, achieved ? "achieved" : "not achieved");
    return stored == 1 && achieved == !strcmp(action, "set") ? 0 : 1;
}

/* --cloud-write TEXT | --cloud-read: one file in the app's Steam Cloud. */
static int cloud(void *storage, const char *text)
{
    char data[256] = "";
    int size;

    report("RESULT cloud enabled for account: %s, for app: %s\n",
           ((storage_bool_fn)api_function("SteamAPI_ISteamRemoteStorage_IsCloudEnabledForAccount"))(storage) ? "yes" : "no",
           ((storage_bool_fn)api_function("SteamAPI_ISteamRemoteStorage_IsCloudEnabledForApp"))(storage) ? "yes" : "no");
    if (text)
    {
        bool written = ((file_write_fn)api_function("SteamAPI_ISteamRemoteStorage_FileWrite"))(storage, CLOUD_FILE, text, (int)strlen(text));
        report("RESULT cloud write %s: %s\n", CLOUD_FILE, written ? "OK" : "FAIL");
        if (!written) return 1;
    }
    if (!((file_exists_fn)api_function("SteamAPI_ISteamRemoteStorage_FileExists"))(storage, CLOUD_FILE))
    {
        report("RESULT cloud read %s: no such file\n", CLOUD_FILE);
        return 1;
    }
    size = ((file_read_fn)api_function("SteamAPI_ISteamRemoteStorage_FileRead"))(storage, CLOUD_FILE, data, sizeof(data) - 1);
    data[size > 0 ? size : 0] = 0;
    report("RESULT cloud read %s: %d bytes, \"%s\"\n", CLOUD_FILE, size, data);
    return text && strcmp(text, data) ? 1 : 0;
}

/* probe.exe [--achievement set|clear|show] [--cloud-write TEXT | --cloud-read] */
int main(int argc, char **argv)
{
    const char *achievement_action = NULL, *cloud_text = NULL;
    bool cloud_check = false;
    void *stats, *storage;
    int i, status = 0;
    const char *dll = sizeof(void *) == 8 ? "steam_api64.dll" : "steam_api.dll";
    char message[1024] = "";
    init_flat_fn init_flat;
    HMODULE steam_api;
    void *utils;
    BOOL ok;

    for (i = 1; i < argc; i++)
    {
        if (!strcmp(argv[i], "--achievement") && i + 1 < argc)
            achievement_action = argv[++i];
        else if (!strcmp(argv[i], "--cloud-write") && i + 1 < argc)
            cloud_text = argv[++i], cloud_check = true;
        else if (!strcmp(argv[i], "--cloud-read"))
            cloud_check = true;
        else
        {
            printf("usage: probe.exe [--achievement set|clear|show] [--cloud-write TEXT | --cloud-read]\n");
            return 2;
        }
    }
    if (achievement_action && strcmp(achievement_action, "set") && strcmp(achievement_action, "clear") && strcmp(achievement_action, "show"))
    {
        printf("--achievement takes set, clear or show\n");
        return 2;
    }

    setvbuf(stdout, NULL, _IONBF, 0);
    open_results();
    report("RESULT bits: %u\n", (unsigned int)(sizeof(void *) * 8));
    report("RESULT env SteamAppId: %s\n", getenv("SteamAppId") ? getenv("SteamAppId") : "(not set)");
    report("RESULT env SteamGameId: %s\n", getenv("SteamGameId") ? getenv("SteamGameId") : "(not set)");
    print_registry("pid");
    print_registry(sizeof(void *) == 8 ? "SteamClientDll64" : "SteamClientDll");

    if (!(steam_api = LoadLibraryA(dll)))
    {
        report("RESULT load %s: FAIL, error %lu\n", dll, GetLastError());
        return 2;
    }
    report("RESULT load %s: OK\n", dll);
    api = steam_api;

    if ((init_flat = (init_flat_fn)GetProcAddress(steam_api, "SteamAPI_InitFlat")))
    {
        int result = init_flat(message);
        report("RESULT SteamAPI_InitFlat: %d (%s), message \"%s\"\n", result,
               result >= 0 && result < 4 ? init_results[result] : "unknown", message);
        ok = result == 0;
    }
    else
    {
        ok = ((init_fn)GetProcAddress(steam_api, "SteamAPI_Init"))();
        report("RESULT SteamAPI_Init: %s\n", ok ? "true" : "false");
    }
    print_steamclient_modules();
    if (!ok)
        return 1;

    if ((utils = interface_of(steam_api, "SteamAPI_SteamUtils_v011")))
        report("RESULT app id: %u\n", ((app_id_fn)GetProcAddress(steam_api, "SteamAPI_ISteamUtils_GetAppID"))(utils));
    if (achievement_action)
        ((dispatch_init_fn)api_function("SteamAPI_ManualDispatch_Init"))();
    stats = interface_of(steam_api, "SteamAPI_SteamUserStats_v013");
    storage = interface_of(steam_api, "SteamAPI_SteamRemoteStorage_v016");
    if (achievement_action)
        status |= stats ? achievement(stats, achievement_action) : 1;
    if (cloud_check)
        status |= storage ? cloud(storage, cloud_text) : 1;
    ((shutdown_fn)GetProcAddress(steam_api, "SteamAPI_Shutdown"))();
    report("RESULT exit: %d\n", status);
    return status;
}
