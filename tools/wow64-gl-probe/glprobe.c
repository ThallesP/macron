/* A 32-bit OpenGL program that sets up contexts the way wined3d does
 * (a hidden window, a legacy context to find wglCreateContextAttribsARB,
 * then a 3.2 core context, glGetString, glGetStringi and glGetIntegerv),
 * to reproduce the 32-bit OpenGL crashes on a Mac without Direct3D.
 * runtime/wine/patches/README.md (0005) has what it found.
 *
 *   glprobe.exe [--rounds N] [--gets N] [--draw] [--threads N] [--syscalls N]
 *               [--syscall-thread] [--shaders N] [--churn]
 *
 * --syscalls N makes N cheap system calls (NtYieldExecution) after the
 * rounds, to tell Wine's 32-to-64 transition apart from OpenGL.
 * --syscall-thread makes such calls on a thread of its own, without any
 * OpenGL, while the other threads run their rounds.
 * --shaders N draws with N new shader programs per round (implies --draw),
 * so Apple's software renderer generates new machine code each time.
 * --churn starts and joins empty threads on a thread of its own until the
 * other threads are done. On macOS every 32-bit Wine thread gets an LDT
 * entry for %fs (i386_set_ldt, which Rosetta emulates), so this changes the
 * LDT while the other threads switch between 32-bit and 64-bit code.
 *
 * After each OpenGL call it checks that the CPU is still in 32-bit mode:
 * "xor eax,eax; .byte 0x40; nop" leaves eax at 1 in 32-bit mode (0x40 is
 * inc eax) and at 0 in 64-bit mode (0x40 is a REX prefix to the nop). In
 * 64-bit mode it stops with ud2 and the call's number in edx. The 64-bit
 * build has no check.
 */
#include <windows.h>
#include <GL/gl.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifdef _WIN64
#define CHECK_MODE(id) do { } while (0)
#else
#define CHECK_MODE(id)                                                                                                 \
    __asm__ volatile("xorl %%eax,%%eax\n\t"                                                                            \
                     ".byte 0x40\n\t"                                                                                  \
                     "nop\n\t"                                                                                         \
                     "testl %%eax,%%eax\n\t"                                                                           \
                     "jnz 1f\n\t"                                                                                      \
                     "movl %0,%%edx\n\t"                                                                               \
                     "ud2\n"                                                                                           \
                     "1:" ::"i"(id)                                                                                    \
                     : "eax", "edx", "cc", "memory")
#endif

#define WGL_CONTEXT_MAJOR_VERSION_ARB 0x2091
#define WGL_CONTEXT_MINOR_VERSION_ARB 0x2092
#define WGL_CONTEXT_FLAGS_ARB 0x2094
#define WGL_CONTEXT_PROFILE_MASK_ARB 0x9126
#define WGL_CONTEXT_CORE_PROFILE_BIT_ARB 0x00000001
#define WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB 0x0002
#define WGL_DRAW_TO_WINDOW_ARB 0x2001
#define WGL_SUPPORT_OPENGL_ARB 0x2010
#define WGL_DOUBLE_BUFFER_ARB 0x2011
#define WGL_PIXEL_TYPE_ARB 0x2013
#define WGL_COLOR_BITS_ARB 0x2014
#define WGL_DEPTH_BITS_ARB 0x2022
#define WGL_STENCIL_BITS_ARB 0x2023
#define WGL_TYPE_RGBA_ARB 0x202B
#define GL_NUM_EXTENSIONS 0x821D
#define GL_MAJOR_VERSION 0x821B
#define GL_MINOR_VERSION 0x821C
#define GL_SHADING_LANGUAGE_VERSION 0x8B8C
#define GL_MAX_TEXTURE_IMAGE_UNITS 0x8872
#define GL_MAX_VERTEX_ATTRIBS 0x8869
#define GL_MAX_DRAW_BUFFERS 0x8824
#define GL_FRAGMENT_SHADER 0x8B30
#define GL_VERTEX_SHADER 0x8B31
#define GL_ARRAY_BUFFER 0x8892
#define GL_STATIC_DRAW 0x88E4
#define GL_COMPILE_STATUS 0x8B81
#define GL_LINK_STATUS 0x8B82

typedef HGLRC(WINAPI *CreateContextAttribs)(HDC, HGLRC, const int *);
typedef BOOL(WINAPI *ChoosePixelFormatARB)(HDC, const int *, const FLOAT *, UINT, int *, UINT *);
typedef const GLubyte *(WINAPI *GetStringi)(GLenum, GLuint);
typedef GLuint(WINAPI *CreateShader)(GLenum);
typedef void(WINAPI *ShaderSource)(GLuint, GLsizei, const char *const *, const GLint *);
typedef void(WINAPI *CompileShader)(GLuint);
typedef void(WINAPI *GetShaderiv)(GLuint, GLenum, GLint *);
typedef GLuint(WINAPI *CreateProgram)(void);
typedef void(WINAPI *AttachShader)(GLuint, GLuint);
typedef void(WINAPI *LinkProgram)(GLuint);
typedef void(WINAPI *UseProgram)(GLuint);
typedef void(WINAPI *DeleteProgram)(GLuint);
typedef void(WINAPI *DeleteShader)(GLuint);
typedef void(WINAPI *GenVertexArrays)(GLsizei, GLuint *);
typedef void(WINAPI *BindVertexArray)(GLuint);
typedef void(WINAPI *DeleteVertexArrays)(GLsizei, const GLuint *);
typedef void(WINAPI *GenBuffers)(GLsizei, GLuint *);
typedef void(WINAPI *BindBuffer)(GLenum, GLuint);
typedef void(WINAPI *BufferData)(GLenum, ptrdiff_t, const void *, GLenum);
typedef void(WINAPI *DeleteBuffers)(GLsizei, const GLuint *);
typedef void(WINAPI *VertexAttribPointer)(GLuint, GLint, GLenum, GLboolean, GLsizei, const void *);
typedef void(WINAPI *EnableVertexAttribArray)(GLuint);
typedef void(WINAPI *Uniform1f)(GLint, GLfloat);
typedef GLint(WINAPI *GetUniformLocation)(GLuint, const char *);

typedef struct {
    int rounds;
    int gets;
    BOOL draw;
    int threads;
    int syscalls;
    int syscall_thread;
    int shaders;
    int churn;
} Options;

static Options options = {20, 200, FALSE, 1, 0, 0, 1, 0};
static volatile LONG gl_done;

static void say(const char *format, ...)
{
    char line[1024];
    DWORD written;
    va_list args;
    int length;

    va_start(args, format);
    length = vsnprintf(line, sizeof(line), format, args);
    va_end(args);
    if (length < 0)
        return;
    if (length >= (int)sizeof(line))
        length = sizeof(line) - 1;
    WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), line, length, &written, NULL);
}

static void *proc(const char *name)
{
    void *p = (void *)wglGetProcAddress(name);
    CHECK_MODE(1);
    return p;
}

static HWND hidden_window(void)
{
    static LONG registered;
    WNDCLASSA wc;

    if (!InterlockedExchange(&registered, 1)) {
        memset(&wc, 0, sizeof(wc));
        wc.style = CS_OWNDC;
        wc.lpfnWndProc = DefWindowProcA;
        wc.hInstance = GetModuleHandleA(NULL);
        wc.lpszClassName = "MacronWow64GLProbe";
        RegisterClassA(&wc);
    }
    return CreateWindowA("MacronWow64GLProbe", "Macron WoW64 GL probe", WS_OVERLAPPEDWINDOW, 0, 0, 64, 64, NULL, NULL,
                         GetModuleHandleA(NULL), NULL);
}

static BOOL set_legacy_format(HDC dc)
{
    PIXELFORMATDESCRIPTOR pfd;
    int format;

    memset(&pfd, 0, sizeof(pfd));
    pfd.nSize = sizeof(pfd);
    pfd.nVersion = 1;
    pfd.dwFlags = PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER;
    pfd.iPixelType = PFD_TYPE_RGBA;
    pfd.cColorBits = 32;
    pfd.cDepthBits = 24;
    pfd.cStencilBits = 8;
    format = ChoosePixelFormat(dc, &pfd);
    CHECK_MODE(2);
    if (!format)
        return FALSE;
    DescribePixelFormat(dc, format, sizeof(pfd), &pfd);
    CHECK_MODE(3);
    return SetPixelFormat(dc, format, &pfd);
}

static int draw(void)
{
    static const char *vs = "#version 150\nin vec2 p;\nuniform float s;\nvoid main() { gl_Position = vec4(p * s, 0.0, 1.0); }\n";
    static LONG programs;
    char fs_text[160];
    const char *fs = fs_text;
    static const float triangle[] = {-1.0f, -1.0f, 1.0f, -1.0f, 0.0f, 1.0f};
    CreateShader create_shader = proc("glCreateShader");
    ShaderSource shader_source = proc("glShaderSource");
    CompileShader compile_shader = proc("glCompileShader");
    GetShaderiv get_shader = proc("glGetShaderiv");
    CreateProgram create_program = proc("glCreateProgram");
    AttachShader attach_shader = proc("glAttachShader");
    LinkProgram link_program = proc("glLinkProgram");
    UseProgram use_program = proc("glUseProgram");
    DeleteProgram delete_program = proc("glDeleteProgram");
    DeleteShader delete_shader = proc("glDeleteShader");
    GenVertexArrays gen_vertex_arrays = proc("glGenVertexArrays");
    BindVertexArray bind_vertex_array = proc("glBindVertexArray");
    DeleteVertexArrays delete_vertex_arrays = proc("glDeleteVertexArrays");
    GenBuffers gen_buffers = proc("glGenBuffers");
    BindBuffer bind_buffer = proc("glBindBuffer");
    BufferData buffer_data = proc("glBufferData");
    DeleteBuffers delete_buffers = proc("glDeleteBuffers");
    VertexAttribPointer vertex_attrib_pointer = proc("glVertexAttribPointer");
    EnableVertexAttribArray enable_vertex_attrib_array = proc("glEnableVertexAttribArray");
    Uniform1f uniform1f = proc("glUniform1f");
    GetUniformLocation get_uniform_location = proc("glGetUniformLocation");
    GLuint shaders[2], program, vao, vbo;
    GLint ok = 0;
    BYTE pixel[4];
    int i;

    if (!create_shader || !uniform1f || !gen_vertex_arrays)
        return 0;
    /* a new program each time, so the driver compiles it instead of reusing one */
    snprintf(fs_text, sizeof(fs_text),
             "#version 150\nout vec4 c;\nuniform float s;\nvoid main() { c = vec4(s, %ld.0 / 1000000.0, 0.25, 1.0); }\n",
             InterlockedIncrement(&programs));
    shaders[0] = create_shader(GL_VERTEX_SHADER);
    CHECK_MODE(40);
    shaders[1] = create_shader(GL_FRAGMENT_SHADER);
    CHECK_MODE(41);
    shader_source(shaders[0], 1, &vs, NULL);
    CHECK_MODE(42);
    shader_source(shaders[1], 1, &fs, NULL);
    CHECK_MODE(43);
    for (i = 0; i < 2; i++) {
        compile_shader(shaders[i]);
        CHECK_MODE(44);
        get_shader(shaders[i], GL_COMPILE_STATUS, &ok);
        CHECK_MODE(45);
        if (!ok)
            return 0;
    }
    program = create_program();
    CHECK_MODE(46);
    attach_shader(program, shaders[0]);
    CHECK_MODE(47);
    attach_shader(program, shaders[1]);
    CHECK_MODE(48);
    link_program(program);
    CHECK_MODE(49);
    use_program(program);
    CHECK_MODE(50);
    gen_vertex_arrays(1, &vao);
    CHECK_MODE(51);
    bind_vertex_array(vao);
    CHECK_MODE(52);
    gen_buffers(1, &vbo);
    CHECK_MODE(53);
    bind_buffer(GL_ARRAY_BUFFER, vbo);
    CHECK_MODE(54);
    buffer_data(GL_ARRAY_BUFFER, sizeof(triangle), triangle, GL_STATIC_DRAW);
    CHECK_MODE(55);
    vertex_attrib_pointer(0, 2, GL_FLOAT, GL_FALSE, 0, NULL);
    CHECK_MODE(56);
    enable_vertex_attrib_array(0);
    CHECK_MODE(57);
    uniform1f(get_uniform_location(program, "s"), 0.5f);
    CHECK_MODE(58);
    glClearColor(0.0f, 0.0f, 1.0f, 1.0f);
    CHECK_MODE(59);
    glClear(GL_COLOR_BUFFER_BIT);
    CHECK_MODE(60);
    glDrawArrays(GL_TRIANGLES, 0, 3);
    CHECK_MODE(61);
    glFinish();
    CHECK_MODE(62);
    glReadPixels(32, 32, 1, 1, GL_RGBA, GL_UNSIGNED_BYTE, pixel);
    CHECK_MODE(63);
    delete_buffers(1, &vbo);
    CHECK_MODE(64);
    delete_vertex_arrays(1, &vao);
    CHECK_MODE(65);
    delete_program(program);
    CHECK_MODE(66);
    delete_shader(shaders[0]);
    CHECK_MODE(67);
    delete_shader(shaders[1]);
    CHECK_MODE(68);
    /* The pixel only proves the draw ran; a hidden window may have no back buffer to read. */
    return 1;
}

/* One round: what wined3d does to create an adapter and then a device. */
static const char *round_trip(int round, int thread)
{
    static const int format_attribs[] = {
        WGL_DRAW_TO_WINDOW_ARB, 1, WGL_SUPPORT_OPENGL_ARB, 1, WGL_DOUBLE_BUFFER_ARB, 1, WGL_PIXEL_TYPE_ARB, WGL_TYPE_RGBA_ARB, WGL_COLOR_BITS_ARB, 32, WGL_DEPTH_BITS_ARB, 24, WGL_STENCIL_BITS_ARB, 8, 0,
    };
    static const int context_attribs[] = {
        WGL_CONTEXT_MAJOR_VERSION_ARB, 3, WGL_CONTEXT_MINOR_VERSION_ARB, 2, WGL_CONTEXT_FLAGS_ARB,
        WGL_CONTEXT_FORWARD_COMPATIBLE_BIT_ARB, WGL_CONTEXT_PROFILE_MASK_ARB, WGL_CONTEXT_CORE_PROFILE_BIT_ARB, 0,
    };
    static const GLenum queries[] = {GL_MAX_TEXTURE_SIZE, GL_MAX_VIEWPORT_DIMS, GL_MAX_TEXTURE_IMAGE_UNITS,
                                     GL_MAX_VERTEX_ATTRIBS, GL_MAX_DRAW_BUFFERS, GL_MAJOR_VERSION, GL_MINOR_VERSION};
    CreateContextAttribs create_context_attribs;
    ChoosePixelFormatARB choose_pixel_format;
    GetStringi get_stringi;
    HGLRC legacy, core;
    HWND window;
    HDC dc;
    GLint values[4], extensions = 0;
    UINT count;
    int format, i;
    const char *failed = NULL;

    if (!(window = hidden_window()))
        return "CreateWindow";
    dc = GetDC(window);
    if (!set_legacy_format(dc)) {
        failed = "SetPixelFormat";
        goto out_window;
    }
    legacy = wglCreateContext(dc);
    CHECK_MODE(4);
    if (!legacy) {
        failed = "wglCreateContext";
        goto out_window;
    }
    if (!wglMakeCurrent(dc, legacy)) {
        failed = "wglMakeCurrent legacy";
        goto out_legacy;
    }
    CHECK_MODE(5);
    for (i = 0; i < 4; i++) {
        glGetString(GL_VENDOR + i);
        CHECK_MODE(6);
    }
    create_context_attribs = proc("wglCreateContextAttribsARB");
    choose_pixel_format = proc("wglChoosePixelFormatARB");
    if (!create_context_attribs || !choose_pixel_format) {
        failed = "wglGetProcAddress";
        goto out_current;
    }
    if (!choose_pixel_format(dc, format_attribs, NULL, 1, &format, &count) || !count) {
        failed = "wglChoosePixelFormatARB";
        goto out_current;
    }
    CHECK_MODE(7);
    core = create_context_attribs(dc, NULL, context_attribs);
    CHECK_MODE(8);
    if (!core) {
        failed = "wglCreateContextAttribsARB";
        goto out_current;
    }
    if (!wglMakeCurrent(dc, core)) {
        failed = "wglMakeCurrent core";
        goto out_core;
    }
    CHECK_MODE(9);
    if (round == 0 && thread == 0)
        say("RESULT renderer %s / %s / %s\n", glGetString(GL_VENDOR), glGetString(GL_RENDERER), glGetString(GL_VERSION));
    CHECK_MODE(10);
    get_stringi = proc("glGetStringi");
    glGetIntegerv(GL_NUM_EXTENSIONS, &extensions);
    CHECK_MODE(11);
    for (i = 0; get_stringi && i < extensions; i++) {
        get_stringi(GL_EXTENSIONS, i);
        CHECK_MODE(12);
    }
    for (i = 0; i < options.gets; i++) {
        glGetIntegerv(queries[i % (sizeof(queries) / sizeof(queries[0]))], values);
        CHECK_MODE(13);
        glGetError();
        CHECK_MODE(14);
    }
    for (i = 0; options.draw && i < options.shaders && !failed; i++) {
        if (!draw())
            failed = "draw";
    }
    SwapBuffers(dc);
    CHECK_MODE(15);
    wglMakeCurrent(NULL, NULL);
    CHECK_MODE(16);
out_core:
    wglDeleteContext(core);
    CHECK_MODE(17);
out_current:
    wglMakeCurrent(NULL, NULL);
    CHECK_MODE(18);
out_legacy:
    wglDeleteContext(legacy);
    CHECK_MODE(19);
out_window:
    ReleaseDC(window, dc);
    DestroyWindow(window);
    return failed;
}

static DWORD WINAPI syscaller(void *arg)
{
    LONG calls = 0;

    while (!gl_done) {
        SwitchToThread();
        CHECK_MODE(91);
        calls++;
    }
    say("RESULT syscall thread: %ld calls\n", calls);
    return 0;
}

static DWORD WINAPI idle(void *arg)
{
    return 0;
}

static DWORD WINAPI churner(void *arg)
{
    LONG started = 0;
    HANDLE thread;

    while (!gl_done) {
        if (!(thread = CreateThread(NULL, 0, idle, NULL, 0, NULL)))
            break;
        WaitForSingleObject(thread, INFINITE);
        CloseHandle(thread);
        CHECK_MODE(92);
        started++;
    }
    say("RESULT churn thread: %ld threads\n", started);
    return 0;
}

static DWORD WINAPI worker(void *arg)
{
    int thread = (int)(INT_PTR)arg, round;
    const char *failed;

    for (round = 0; round < options.rounds; round++) {
        if ((failed = round_trip(round, thread))) {
            say("RESULT round %d thread %d: FAILED (%s)\n", round, thread, failed);
            return 1;
        }
    }
    for (round = 0; round < options.syscalls; round++) {
        SwitchToThread();
        CHECK_MODE(90);
    }
    return 0;
}

int main(int argc, char **argv)
{
    HANDLE threads[18];
    DWORD code, failed = 0;
    int i;

    for (i = 1; i < argc; i++) {
        if (!strcmp(argv[i], "--draw"))
            options.draw = TRUE;
        else if (!strcmp(argv[i], "--rounds") && i + 1 < argc)
            options.rounds = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--gets") && i + 1 < argc)
            options.gets = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--threads") && i + 1 < argc)
            options.threads = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--shaders") && i + 1 < argc) {
            options.draw = TRUE;
            options.shaders = atoi(argv[++i]);
        }
        else if (!strcmp(argv[i], "--syscall-thread"))
            options.syscall_thread = 1;
        else if (!strcmp(argv[i], "--syscalls") && i + 1 < argc)
            options.syscalls = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--churn"))
            options.churn = 1;
        else {
            say("usage: glprobe.exe [--rounds N] [--gets N] [--draw] [--threads N] [--syscalls N] [--syscall-thread] [--shaders N] "
                "[--churn]\n");
            return 2;
        }
    }
    if (options.threads < 1 || options.threads > 16)
        return 2;
    say("RESULT config rounds %d gets %d draw %s shaders %d threads %d syscalls %d churn %d bits %d\n", options.rounds,
        options.gets, options.draw ? "yes" : "no", options.draw ? options.shaders : 0, options.threads, options.syscalls,
        options.churn, (int)(sizeof(void *) * 8));
    if (options.syscall_thread)
        threads[options.threads] = CreateThread(NULL, 0, syscaller, NULL, 0, NULL);
    if (options.churn)
        threads[options.threads + 1] = CreateThread(NULL, 0, churner, NULL, 0, NULL);
    for (i = 0; i < options.threads; i++)
        threads[i] = CreateThread(NULL, 0, worker, (void *)(INT_PTR)i, 0, NULL);
    WaitForMultipleObjects(options.threads, threads, TRUE, INFINITE);
    gl_done = 1;
    if (options.syscall_thread)
        WaitForSingleObject(threads[options.threads], INFINITE);
    if (options.churn)
        WaitForSingleObject(threads[options.threads + 1], INFINITE);
    for (i = 0; i < options.threads; i++) {
        GetExitCodeThread(threads[i], &code);
        failed |= code;
    }
    say("RESULT glprobe: %s\n", failed ? "FAILED" : "OK");
    return failed ? 1 : 0;
}
