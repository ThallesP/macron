// The frame-rate probe's scene drawn by a native Mac program through Apple's
// OpenGL, with no Wine and no Direct3D translation: the same backdrop, 400
// alpha-blended sprites (one draw each, a texture and blend change between
// them), the panel rewritten every frame, the vignette and the render-target
// pass, into a 1280x720 frame scaled to a 960x540 window, presented without
// vsync. It prints the probe's RESULT lines, so the same parser reads both.
// Build: clang -fobjc-arc -framework Cocoa -framework OpenGL -o glprobe glprobe.m
#define GL_SILENCE_DEPRECATION
#import <Cocoa/Cocoa.h>
#import <OpenGL/gl.h>
#include <mach/mach_time.h>
#include <stdlib.h>

enum { WIDTH = 1280, HEIGHT = 720, SPRITE_TEXTURES = 64, SPRITE_SIZE = 64, PANEL_WIDTH = 512, PANEL_HEIGHT = 256 };

static const uint32_t palette[] = { 0xffe8553a, 0xfff2b134, 0xff4fb477, 0xff3a86e8, 0xff9b5de5, 0xfff15bb5, 0xff00bbf9, 0xfffee440 };

static uint32_t argb(int a, int r, int g, int b) { return ((uint32_t)a << 24) | ((uint32_t)r << 16) | ((uint32_t)g << 8) | (uint32_t)b; }
static int clamp255(int v) { return v < 0 ? 0 : v > 255 ? 255 : v; }

static uint32_t sprite_pixel(int shape, uint32_t color, int x, int y) {
    int c = SPRITE_SIZE / 2, dx = x - c, dy = y - c, d2 = dx * dx + dy * dy, ad = abs(dx) + abs(dy), alpha = 0;
    if (shape == 0) alpha = (int)((29.0 * 29.0 - d2) * 0.25);
    if (shape == 1) alpha = d2 < 29 * 29 && d2 > 18 * 18 ? 255 : 0;
    if (shape == 2) alpha = (30 - ad) * 40;
    if (shape == 3) alpha = abs(dx) < 26 && abs(dy) < 26 ? 230 : 0;
    alpha = clamp255(alpha);
    if (shape == 3 && ((x / 8 + y / 8) & 1))
        return argb(alpha, ((color >> 16) & 0xff) / 2, ((color >> 8) & 0xff) / 2, (color & 0xff) / 2);
    return (color & 0x00ffffff) | ((uint32_t)alpha << 24);
}
static uint32_t backdrop_pixel(int x, int y) { int band = (x / 32 + y / 32) & 1; return argb(255, 20 + y / 8, 30 + y / 5 + band * 12, 70 + x / 6); }
static uint32_t overlay_pixel(int x, int y) { int dx = x - 64, dy = y - 64; return argb(clamp255((dx * dx + dy * dy) / 24), 0, 0, 0); }
static uint32_t panel_pixel(int x, int y) {
    int row = y / 32;
    if (x < 8 || y < 8 || x >= PANEL_WIDTH - 8 || y >= PANEL_HEIGHT - 8) return 0xfff4f1de;
    if (y % 32 > 6 && y % 32 < 24 && x > 24 && x < 24 + (row * 53 + 120) % 440) return argb(255, 60 + row * 20, 200 - row * 15, 120);
    return 0xe0202838;
}

static GLuint texture_from(int width, int height, uint32_t (^pixel)(int, int)) {
    uint32_t *pixels = malloc(width * height * 4);
    for (int y = 0; y < height; y++)
        for (int x = 0; x < width; x++) pixels[y * width + x] = pixel(x, y);
    GLuint texture;
    glGenTextures(1, &texture);
    glBindTexture(GL_TEXTURE_2D, texture);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_S, GL_CLAMP_TO_EDGE);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_WRAP_T, GL_CLAMP_TO_EDGE);
    glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, width, height, 0, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, pixels);
    free(pixels);
    return texture;
}

static void filter(GLenum mode) {
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, mode);
    glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, mode);
}

static void quad(float x, float y, float w, float h, uint32_t color) {
    GLfloat vertices[] = { x, y, x + w, y, x, y + h, x + w, y + h };
    GLfloat uv[] = { 0, 0, 1, 0, 0, 1, 1, 1 };
    GLubyte c[16];
    for (int i = 0; i < 4; i++) {
        c[i * 4] = (color >> 16) & 0xff; c[i * 4 + 1] = (color >> 8) & 0xff; c[i * 4 + 2] = color & 0xff; c[i * 4 + 3] = color >> 24;
    }
    glVertexPointer(2, GL_FLOAT, 0, vertices);
    glTexCoordPointer(2, GL_FLOAT, 0, uv);
    glColorPointer(4, GL_UNSIGNED_BYTE, 0, c);
    glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
}

static int compare_floats(const void *a, const void *b) { float x = *(const float *)a, y = *(const float *)b; return x < y ? -1 : x > y; }
static float percentile(const float *sorted, size_t n, double q) { size_t i = (size_t)(q * n + 0.999999); return sorted[(i ? i : 1) - 1]; }

int main(int argc, const char **argv) {
    @autoreleasepool {
        double warmup_seconds = argc > 1 ? atof(argv[1]) : 2, measure_seconds = argc > 2 ? atof(argv[2]) : 10;
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(40, 40, 960, 540) styleMask:NSWindowStyleMaskTitled
                                                         backing:NSBackingStoreBuffered defer:NO];
        window.title = @"Macron native GL probe";
        NSOpenGLPixelFormatAttribute attributes[] = { NSOpenGLPFADoubleBuffer, NSOpenGLPFAColorSize, 24, NSOpenGLPFAAlphaSize, 8, 0 };
        NSOpenGLPixelFormat *format = [[NSOpenGLPixelFormat alloc] initWithAttributes:attributes];
        NSOpenGLView *view = [[NSOpenGLView alloc] initWithFrame:NSMakeRect(0, 0, 960, 540) pixelFormat:format];
        view.wantsBestResolutionOpenGLSurface = NO;
        window.contentView = view;
        [window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        NSOpenGLContext *context = view.openGLContext;
        [context makeCurrentContext];
        GLint swapInterval = 0;
        [context setValues:&swapInterval forParameter:NSOpenGLContextParameterSwapInterval];
        printf("RESULT window: OK\nRESULT adapter %s (native OpenGL %s)\n", glGetString(GL_RENDERER), glGetString(GL_VERSION));

        GLuint sprites[SPRITE_TEXTURES];
        for (int i = 0; i < SPRITE_TEXTURES; i++)
            sprites[i] = texture_from(SPRITE_SIZE, SPRITE_SIZE, ^(int x, int y) { return sprite_pixel(i % 4, palette[(i / 4) % 8], x, y); });
        GLuint backdrop = texture_from(256, 256, ^(int x, int y) { return backdrop_pixel(x, y); });
        GLuint overlay = texture_from(128, 128, ^(int x, int y) { return overlay_pixel(x, y); });
        GLuint panel = texture_from(PANEL_WIDTH, PANEL_HEIGHT, ^(int x, int y) { return panel_pixel(x, y); });
        uint32_t *panel_pixels = malloc(PANEL_WIDTH * PANEL_HEIGHT * 4);

        GLuint frame[2], frame_texture[2];
        glGenFramebuffersEXT(2, frame);
        glGenTextures(2, frame_texture);
        for (int i = 0; i < 2; i++) {
            glBindTexture(GL_TEXTURE_2D, frame_texture[i]);
            glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, WIDTH, HEIGHT, 0, GL_BGRA, GL_UNSIGNED_BYTE, NULL);
            filter(GL_NEAREST);
            glBindFramebufferEXT(GL_FRAMEBUFFER_EXT, frame[i]);
            glFramebufferTexture2DEXT(GL_FRAMEBUFFER_EXT, GL_COLOR_ATTACHMENT0_EXT, GL_TEXTURE_2D, frame_texture[i], 0);
        }
        glEnable(GL_TEXTURE_2D);
        glEnableClientState(GL_VERTEX_ARRAY);
        glEnableClientState(GL_TEXTURE_COORD_ARRAY);
        glEnableClientState(GL_COLOR_ARRAY);
        glTexEnvi(GL_TEXTURE_ENV, GL_TEXTURE_ENV_MODE, GL_MODULATE);

        size_t capacity = 4096, frames = 0;
        float *frame_ms = malloc(capacity * sizeof(float));
        mach_timebase_info_data_t timebase;
        mach_timebase_info(&timebase);
        uint64_t started = mach_absolute_time(), previous = started, measuring_since = 0;
        size_t warmup_frames = 0;
        for (;;) {
            for (NSEvent *event; (event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:nil inMode:NSDefaultRunLoopMode dequeue:YES]);)
                [NSApp sendEvent:event];

            for (int y = 0; y < PANEL_HEIGHT; y++)
                for (int x = 0; x < PANEL_WIDTH; x++) panel_pixels[y * PANEL_WIDTH + x] = panel_pixel(x, y);
            glBindTexture(GL_TEXTURE_2D, panel);
            glTexSubImage2D(GL_TEXTURE_2D, 0, 0, 0, PANEL_WIDTH, PANEL_HEIGHT, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, panel_pixels);

            glBindFramebufferEXT(GL_FRAMEBUFFER_EXT, frame[0]);
            glViewport(0, 0, WIDTH, HEIGHT);
            glMatrixMode(GL_PROJECTION);
            glLoadIdentity();
            glOrtho(0, WIDTH, HEIGHT, 0, -1, 1);
            glClearColor(0, 0, 0, 1);
            glClear(GL_COLOR_BUFFER_BIT);
            glDisable(GL_BLEND);
            glBindTexture(GL_TEXTURE_2D, backdrop);
            filter(GL_LINEAR);
            quad(0, 0, WIDTH, HEIGHT, 0xffffffff);
            glEnable(GL_BLEND);
            for (int i = 0; i < 400; i++) {
                int column = i % 25, row = (i / 25) % 16;
                float size = 40.0f + (float)((i * 37) % 5) * 12.0f;
                float x = 26.0f + column * 50.0f - size / 2 + 12, y = 40.0f + row * 42.0f - size / 2 + 6;
                glBlendFunc(GL_SRC_ALPHA, i % 5 == 4 ? GL_ONE : GL_ONE_MINUS_SRC_ALPHA);
                glBindTexture(GL_TEXTURE_2D, sprites[(i * 7) % SPRITE_TEXTURES]);
                filter(i % 2 ? GL_LINEAR : GL_NEAREST);
                quad(x, y, size, size, i % 3 == 0 ? 0xb0ffffff : 0xffffffff);
            }
            glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
            glBindTexture(GL_TEXTURE_2D, panel);
            filter(GL_NEAREST);
            quad(740, 440, PANEL_WIDTH, PANEL_HEIGHT, 0xffffffff);
            glBindTexture(GL_TEXTURE_2D, overlay);
            filter(GL_LINEAR);
            quad(0, 0, WIDTH, HEIGHT, 0xffffffff);

            glBindFramebufferEXT(GL_FRAMEBUFFER_EXT, frame[1]);
            glDisable(GL_BLEND);
            glBindTexture(GL_TEXTURE_2D, frame_texture[0]);
            quad(0, 0, WIDTH, HEIGHT, 0xffffffff);

            glBindFramebufferEXT(GL_READ_FRAMEBUFFER_EXT, frame[1]);
            glBindFramebufferEXT(GL_DRAW_FRAMEBUFFER_EXT, 0);
            glBlitFramebufferEXT(0, 0, WIDTH, HEIGHT, 0, 540, 960, 0, GL_COLOR_BUFFER_BIT, GL_LINEAR);
            [context flushBuffer];

            uint64_t now = mach_absolute_time();
            double elapsed = (now - started) * timebase.numer / timebase.denom / 1e9;
            float ms = (float)((now - previous) * timebase.numer / timebase.denom / 1e6);
            previous = now;
            if (!measuring_since) {
                warmup_frames++;
                if (elapsed >= warmup_seconds && warmup_frames >= 60) measuring_since = now;
                continue;
            }
            if (frames == capacity) frame_ms = realloc(frame_ms, (capacity *= 2) * sizeof(float));
            frame_ms[frames++] = ms;
            double measured = (now - measuring_since) * timebase.numer / timebase.denom / 1e9;
            if (measured < measure_seconds) continue;
            qsort(frame_ms, frames, sizeof(float), compare_floats);
            printf("RESULT fps %.2f p50_ms %.3f p95_ms %.3f p99_ms %.3f frames %zu\n", frames / measured,
                   percentile(frame_ms, frames, 0.5), percentile(frame_ms, frames, 0.95), percentile(frame_ms, frames, 0.99), frames);
            return 0;
        }
    }
}
