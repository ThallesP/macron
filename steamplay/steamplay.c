// Macron's Steam Play switch. README.md in this folder says what it changes in Steam and why.

#include <dlfcn.h>
#include <libkern/OSCacheControl.h>
#include <limits.h>
#include <mach-o/dyld.h>
#include <mach-o/loader.h>
#include <mach/mach.h>
#include <spawn.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <time.h>
#include <unistd.h>

#include "builds.h"

#define INSERT "DYLD_INSERT_LIBRARIES"

extern char **environ;

static char own_path[PATH_MAX];

static void say(const char *format, ...) __attribute__((format(printf, 1, 2)));

static void say(const char *format, ...) {
    char path[PATH_MAX];
    const char *macron_home = getenv("MACRON_HOME");
    const char *home = getenv("HOME");
    if (macron_home && macron_home[0]) {
        snprintf(path, sizeof path, "%s", macron_home);
    } else if (home) {
        snprintf(path, sizeof path, "%s/Library/Application Support/Macron", home);
    } else {
        return;
    }
    mkdir(path, 0755);
    strlcat(path, "/logs", sizeof path);
    mkdir(path, 0755);
    strlcat(path, "/steamplay.log", sizeof path);

    FILE *log = fopen(path, "a");
    if (!log) return;

    time_t now = time(NULL);
    struct tm parts;
    localtime_r(&now, &parts);
    char stamp[32];
    strftime(stamp, sizeof stamp, "%Y-%m-%d %H:%M:%S", &parts);
    fprintf(log, "[%s] pid %d: ", stamp, getpid());

    va_list arguments;
    va_start(arguments, format);
    vfprintf(log, format, arguments);
    va_end(arguments);
    fputc('\n', log);
    fclose(log);
}

static bool has_suffix(const char *text, const char *suffix) {
    size_t text_length = strlen(text);
    size_t suffix_length = strlen(suffix);
    return text_length >= suffix_length && strcmp(text + text_length - suffix_length, suffix) == 0;
}

static bool image_uuid(const struct mach_header *header, char out[33]) {
    if (header->magic != MH_MAGIC_64) return false;

    const struct load_command *command = (const void *)((const struct mach_header_64 *)header + 1);
    for (uint32_t i = 0; i < header->ncmds; i++) {
        if (command->cmd == LC_UUID) {
            const uint8_t *uuid = ((const struct uuid_command *)command)->uuid;
            for (int byte = 0; byte < 16; byte++) snprintf(out + byte * 2, 3, "%02x", uuid[byte]);
            return true;
        }
        command = (const void *)((const uint8_t *)command + command->cmdsize);
    }
    return false;
}

static const struct steam_build *known_build(const char *uuid) {
    for (size_t i = 0; i < sizeof steam_builds / sizeof steam_builds[0]; i++) {
        if (strcmp(steam_builds[i].uuid, uuid) == 0) return &steam_builds[i];
    }
    return NULL;
}

static bool write_instruction(uint32_t *address, uint32_t instruction) {
    vm_address_t page = (vm_address_t)address & ~(vm_address_t)(vm_page_size - 1);
    if (vm_protect(mach_task_self(), page, vm_page_size, false, VM_PROT_READ | VM_PROT_WRITE | VM_PROT_COPY) != KERN_SUCCESS) {
        return false;
    }
    *address = instruction;
    kern_return_t restored = vm_protect(mach_task_self(), page, vm_page_size, false, VM_PROT_READ | VM_PROT_EXECUTE);
    sys_icache_invalidate(address, sizeof instruction);
    return restored == KERN_SUCCESS;
}

static void switch_on(const struct mach_header *header, intptr_t slide) {
    char uuid[33];
    if (!image_uuid(header, uuid)) {
        say("UNKNOWN_STEAM_BUILD: steamclient.dylib has no build id; Steam Play stays off and nothing was changed");
        return;
    }

    const struct steam_build *build = known_build(uuid);
    if (!build) {
        say("UNKNOWN_STEAM_BUILD: steamclient.dylib %s is not a build this Macron knows; Steam Play stays off and nothing was changed. "
            "Steam has probably updated itself: install the newest Macron",
            uuid);
        return;
    }

    uint32_t *enabled = (uint32_t *)(build->enabled.address + slide);
    uint32_t *stop = (uint32_t *)(build->stop.address + slide);
    if (*enabled != build->enabled.original || *stop != build->stop.original) {
        say("UNEXPECTED_STEAM_CODE: build %s has %08x and %08x where %08x and %08x were expected; Steam Play stays off and nothing was changed",
            build->id, *enabled, *stop, build->enabled.original, build->stop.original);
        return;
    }

    if (!write_instruction(enabled, build->enabled.patched)) {
        say("STEAM_PLAY_SWITCH_FAILED: macOS did not let Macron change Steam's code in memory; Steam Play stays off");
        return;
    }
    if (!write_instruction(stop, build->stop.patched)) {
        say("Steam Play is on (build %s), but Steam's Stop button will not stop Windows games: macOS did not allow that change", build->id);
        return;
    }
    say("Steam Play is on (build %s, steamclient.dylib %s)", build->id, uuid);
}

static void image_added(const struct mach_header *header, intptr_t slide) {
    Dl_info info;
    if (!dladdr(header, &info) || !info.dli_fname) return;
    if (!has_suffix(info.dli_fname, "/steamclient.dylib")) return;
    switch_on(header, slide);
}

static bool is_steam(const char *path) {
    if (!path) return false;
    const char *slash = strrchr(path, '/');
    return strcmp(slash ? slash + 1 : path, "steam_osx") == 0;
}

static void insert_list(const char *inserted, bool with_own, char *out, size_t size) {
    out[0] = '\0';
    if (with_own) strlcpy(out, own_path, size);
    if (!inserted) return;

    char *list = strdup(inserted);
    if (!list) return;
    char *rest = list;
    for (char *entry; (entry = strsep(&rest, ":")) != NULL;) {
        if (!entry[0] || strcmp(entry, own_path) == 0) continue;
        if (out[0]) strlcat(out, ":", size);
        strlcat(out, entry, size);
    }
    free(list);
}

static void set_own_insert(bool with_own) {
    char list[4096];
    insert_list(getenv(INSERT), with_own, list, sizeof list);
    if (list[0]) {
        setenv(INSERT, list, 1);
        return;
    }
    unsetenv(INSERT);
}

static char **environment_with_own_insert(char *const envp[]) {
    size_t count = 0;
    const char *inserted = NULL;
    for (; envp && envp[count]; count++) {
        if (strncmp(envp[count], INSERT "=", sizeof INSERT) == 0) inserted = envp[count] + sizeof INSERT;
    }

    char list[4096];
    insert_list(inserted, true, list, sizeof list);
    char *entry = NULL;
    if (asprintf(&entry, INSERT "=%s", list) < 0) return NULL;

    char **copy = calloc(count + 2, sizeof *copy);
    if (!copy) {
        free(entry);
        return NULL;
    }
    size_t kept = 0;
    for (size_t i = 0; i < count; i++) {
        if (strncmp(envp[i], INSERT "=", sizeof INSERT) != 0) copy[kept++] = envp[i];
    }
    copy[kept] = entry;
    return copy;
}

static void free_environment(char **copy) {
    if (!copy) return;
    size_t last = 0;
    while (copy[last + 1]) last++;
    free(copy[last]);
    free(copy);
}

static int steam_execv(const char *path, char *const argv[]) {
    if (!is_steam(path)) return execv(path, argv);
    set_own_insert(true);
    int result = execv(path, argv);
    set_own_insert(false);
    return result;
}

static int steam_execvp(const char *file, char *const argv[]) {
    if (!is_steam(file)) return execvp(file, argv);
    set_own_insert(true);
    int result = execvp(file, argv);
    set_own_insert(false);
    return result;
}

static int steam_execve(const char *path, char *const argv[], char *const envp[]) {
    if (!is_steam(path)) return execve(path, argv, envp);
    char **with_own = environment_with_own_insert(envp);
    int result = execve(path, argv, with_own ? with_own : envp);
    free_environment(with_own);
    return result;
}

static int steam_posix_spawn(pid_t *pid, const char *path, const posix_spawn_file_actions_t *actions,
                             const posix_spawnattr_t *attributes, char *const argv[], char *const envp[]) {
    if (!is_steam(path)) return posix_spawn(pid, path, actions, attributes, argv, envp);
    char **with_own = environment_with_own_insert(envp ? envp : environ);
    int result = posix_spawn(pid, path, actions, attributes, argv, with_own ? with_own : envp);
    free_environment(with_own);
    return result;
}

static int steam_posix_spawnp(pid_t *pid, const char *file, const posix_spawn_file_actions_t *actions,
                              const posix_spawnattr_t *attributes, char *const argv[], char *const envp[]) {
    if (!is_steam(file)) return posix_spawnp(pid, file, actions, attributes, argv, envp);
    char **with_own = environment_with_own_insert(envp ? envp : environ);
    int result = posix_spawnp(pid, file, actions, attributes, argv, with_own ? with_own : envp);
    free_environment(with_own);
    return result;
}

#define INTERPOSE(replacement, original) \
    __attribute__((used)) static const struct { \
        const void *replacement_function; \
        const void *original_function; \
    } interpose_##original __attribute__((section("__DATA,__interpose"))) = {(const void *)replacement, (const void *)original}

INTERPOSE(steam_execv, execv);
INTERPOSE(steam_execvp, execvp);
INTERPOSE(steam_execve, execve);
INTERPOSE(steam_posix_spawn, posix_spawn);
INTERPOSE(steam_posix_spawnp, posix_spawnp);

__attribute__((constructor)) static void start(void) {
    const char *program = getprogname();
    if (!program || strcmp(program, "steam_osx") != 0) return;

    Dl_info self;
    if (!dladdr(start, &self) || !self.dli_fname || strlcpy(own_path, self.dli_fname, sizeof own_path) >= sizeof own_path) {
        say("STEAM_PLAY_SWITCH_FAILED: could not find this library's own path; Steam Play stays off");
        return;
    }
    set_own_insert(false);
    _dyld_register_func_for_add_image(image_added);
}
