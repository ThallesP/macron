// loadprobe: a native Mac program that draws the load a Direct3D 9 game's
// frame puts on the renderer, with no Wine, no Direct3D translation and no
// Rosetta: N textured, alpha-blended quads per frame into a render target the
// size of the game's back buffer, that target drawn to the window, presented
// without vsync. Each quad's vertices are uploaded before its draw (as wined3d
// does for DrawPrimitiveUP), and the texture changes between draws. It runs on
// Apple's OpenGL (its software renderer, or the hardware one if there is one)
// or on Metal, so the two can be put side by side on the same machine.
//
//   loadprobe api=gl-sw|gl|metal size=1024x768 draws=N quad=WxH tex=WxH
//             textures=K rt=1 upload=BYTES uniforms=0|1 warm=S seconds=S
//
// vb= (OpenGL only) how each draw's vertices reach the buffer:
//   ring    (default) at the next offset of one large vertex buffer, with its
//           6 indices likewise in an index buffer, drawn with
//           glDrawElementsInstancedBaseVertex: what wined3d does for
//           Ultimate Custom Night's draws (+opengl trace)
//   sub0    glBufferSubData at offset 0 of a small buffer every draw
//   orphan  glBufferData(NULL) first, so the driver can give a fresh buffer
// uniforms=1 also sets two 4x4 matrices and a vec3 before every draw, as
// wined3d does for each Direct3D 9 draw (Metal: one more setVertexBytes).
// quad=full makes every quad cover the whole target. upload=BYTES rewrites
// that many bytes of one texture per frame (glTexSubImage2D / replaceRegion).
// Prints one RESULT line: fps, frame-time percentiles, and the CPU the
// process used (user + system, in cores) while measuring.
// Build: clang -O2 -fobjc-arc -framework Cocoa -framework OpenGL -framework Metal -framework QuartzCore -o loadprobe loadprobe.m
#define GL_SILENCE_DEPRECATION
#import <Cocoa/Cocoa.h>
#import <Metal/Metal.h>
#import <OpenGL/gl3.h>
#import <OpenGL/OpenGL.h>
#import <QuartzCore/QuartzCore.h>
#include <mach/mach_time.h>
#include <sys/resource.h>

static int W = 1024, H = 768, DRAWS = 40, QW = 256, QH = 256, TW = 256, TH = 256, TEXTURES = 16, RT = 1, UPLOAD = 0, UNIFORMS = 0;
#if defined(__x86_64__)
static const char *ARCH = "x86_64";
#else
static const char *ARCH = "arm64";
#endif
static double WARM = 5, SECONDS = 10;
static NSString *API = @"gl-sw", *VB = @"ring";

static void size_arg(NSString *v, int *w, int *h) {
    NSArray *p = [v componentsSeparatedByString:@"x"];
    *w = [p[0] intValue]; *h = [p[1] intValue];
}

static uint32_t *texture_pixels(int k) {
    uint32_t *px = malloc((size_t)TW * TH * 4);
    for (int y = 0; y < TH; y++)
        for (int x = 0; x < TW; x++) {
            int dx = x - TW / 2, dy = y - TH / 2, a = 255 - (dx * dx + dy * dy) * 255 / (TW * TW / 4 + TH * TH / 4 + 1);
            a = a < 0 ? 0 : a;
            px[y * TW + x] = ((uint32_t)a << 24) | ((uint32_t)((x * 3 + k * 40) & 0xff) << 16) | ((uint32_t)((y * 5 + k * 17) & 0xff) << 8) | (uint32_t)(k * 23 & 0xff);
        }
    return px;
}

static void quad_rect(int i, float r[4]) {
    if (QW >= W && QH >= H) { r[0] = 0; r[1] = 0; r[2] = W; r[3] = H; return; }
    r[0] = (float)((i * 97) % (W - QW + 1)); r[1] = (float)((i * 61) % (H - QH + 1)); r[2] = QW; r[3] = QH;
}

// Vertex layout shared by both APIs: x, y in pixels, u, v.
static void quad_vertices(const float r[4], float v[16]) {
    float x0 = r[0], y0 = r[1], x1 = r[0] + r[2], y1 = r[1] + r[3];
    float q[16] = { x0, y0, 0, 0, x1, y0, 1, 0, x0, y1, 0, 1, x1, y1, 1, 1 };
    memcpy(v, q, sizeof q);
}

static double now_s(void) {
    static mach_timebase_info_data_t tb;
    if (!tb.denom) mach_timebase_info(&tb);
    return (double)mach_absolute_time() * tb.numer / tb.denom / 1e9;
}
static double cpu_s(void) {
    struct rusage u; getrusage(RUSAGE_SELF, &u);
    return u.ru_utime.tv_sec + u.ru_utime.tv_usec / 1e6 + u.ru_stime.tv_sec + u.ru_stime.tv_usec / 1e6;
}
static int cmp(const void *a, const void *b) { double x = *(const double *)a, y = *(const double *)b; return x < y ? -1 : x > y; }

// Frame loop bookkeeping shared by both APIs; returns 1 when done.
static double *frame_ms; static size_t frames, cap = 1 << 16;
static double t_start, t_prev, t_meas, c_meas;
static int frame_done(void) {
    double t = now_s();
    if (!t_meas) {
        if (t - t_start >= WARM) { t_meas = t; c_meas = cpu_s(); }
        t_prev = t;
        return 0;
    }
    frame_ms[frames++ % cap] = (t - t_prev) * 1000; t_prev = t;
    if (t - t_meas < SECONDS) return 0;
    double secs = t - t_meas, cores = (cpu_s() - c_meas) / secs;
    size_t n = frames < cap ? frames : cap;
    qsort(frame_ms, n, sizeof(double), cmp);
    printf("RESULT api=%s vb=%s arch=%s size=%dx%d draws=%d quad=%dx%d tex=%dx%d textures=%d rt=%d upload=%d uniforms=%d fps %.2f p50_ms %.2f p95_ms %.2f frames %zu cpu_cores %.2f\n",
           API.UTF8String, VB.UTF8String, ARCH, W, H, DRAWS, QW, QH, TW, TH, TEXTURES, RT, UPLOAD, UNIFORMS, frames / secs, frame_ms[n / 2], frame_ms[(size_t)(n * 0.95)], frames, cores);
    fflush(stdout);
    return 1;
}

static NSWindow *make_window(void) {
    NSWindow *w = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, W, H) styleMask:NSWindowStyleMaskBorderless backing:NSBackingStoreBuffered defer:NO];
    w.title = @"Macron load probe";
    [w setFrameOrigin:NSMakePoint(0, NSScreen.mainScreen.frame.size.height - H)];
    [w makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
    return w;
}
static void pump(void) {
    for (NSEvent *e; (e = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:nil inMode:NSDefaultRunLoopMode dequeue:YES]);) [NSApp sendEvent:e];
}

static GLuint shader(GLenum kind, const char *src) {
    GLuint s = glCreateShader(kind); glShaderSource(s, 1, &src, NULL); glCompileShader(s);
    GLint ok; glGetShaderiv(s, GL_COMPILE_STATUS, &ok);
    if (!ok) { char log[1024]; glGetShaderInfoLog(s, sizeof log, NULL, log); fprintf(stderr, "shader: %s\n", log); exit(2); }
    return s;
}

static int run_gl(BOOL software) {
    NSOpenGLPixelFormatAttribute attrs[16] = { NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion4_1Core, NSOpenGLPFADoubleBuffer,
        NSOpenGLPFAColorSize, 24, NSOpenGLPFAAlphaSize, 8 };
    int n = 7;
    if (software) { attrs[n++] = NSOpenGLPFARendererID; attrs[n++] = kCGLRendererGenericFloatID; }
    else attrs[n++] = NSOpenGLPFAAccelerated;
    attrs[n] = 0;
    NSOpenGLPixelFormat *pf = [[NSOpenGLPixelFormat alloc] initWithAttributes:attrs];
    if (!pf) { printf("RESULT api=%s no pixel format\n", API.UTF8String); return 3; }
    NSWindow *window = make_window();
    NSOpenGLView *view = [[NSOpenGLView alloc] initWithFrame:NSMakeRect(0, 0, W, H) pixelFormat:pf];
    view.wantsBestResolutionOpenGLSurface = NO;
    window.contentView = view;
    NSOpenGLContext *ctx = view.openGLContext;
    [ctx makeCurrentContext];
    GLint zero = 0; [ctx setValues:&zero forParameter:NSOpenGLContextParameterSwapInterval];
    printf("RESULT adapter %s, OpenGL %s\n", glGetString(GL_RENDERER), glGetString(GL_VERSION));

    GLuint prog = glCreateProgram();
    glAttachShader(prog, shader(GL_VERTEX_SHADER, "#version 410\nlayout(location=0) in vec4 p; uniform vec2 s; uniform mat4 m0; uniform mat4 m1; uniform vec3 k; out vec2 uv;"
        "void main(){ uv=p.zw; gl_Position=m1*(m0*vec4(p.x/s.x*2.0-1.0, 1.0-p.y/s.y*2.0, k.z, 1.0)); }"));
    glAttachShader(prog, shader(GL_FRAGMENT_SHADER, "#version 410\nin vec2 uv; uniform sampler2D t; out vec4 c; void main(){ c=texture(t, uv); }"));
    glLinkProgram(prog); glUseProgram(prog);
    glUniform2f(glGetUniformLocation(prog, "s"), W, H);
    GLint m0 = glGetUniformLocation(prog, "m0"), m1 = glGetUniformLocation(prog, "m1"), k = glGetUniformLocation(prog, "k");
    static const float identity[16] = { 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1 };
    glUniformMatrix4fv(m0, 1, GL_FALSE, identity); glUniformMatrix4fv(m1, 1, GL_FALSE, identity); glUniform3f(k, 0, 0, 0);

    enum { RING_QUADS = 16384 };
    BOOL ring = [VB isEqual:@"ring"], orphan = [VB isEqual:@"orphan"];
    GLsizeiptr vbsize = ring ? RING_QUADS * 64 : 64;
    GLuint vao, vbo, ibo; glGenVertexArrays(1, &vao); glBindVertexArray(vao);
    glGenBuffers(1, &vbo); glBindBuffer(GL_ARRAY_BUFFER, vbo);
    glBufferData(GL_ARRAY_BUFFER, vbsize, NULL, GL_STREAM_DRAW);
    glGenBuffers(1, &ibo); glBindBuffer(GL_ELEMENT_ARRAY_BUFFER, ibo);
    glBufferData(GL_ELEMENT_ARRAY_BUFFER, RING_QUADS * 12, NULL, GL_STREAM_DRAW);
    glEnableVertexAttribArray(0); glVertexAttribPointer(0, 4, GL_FLOAT, GL_FALSE, 0, 0);
    static const GLushort quad_indices[6] = { 0, 1, 2, 2, 1, 3 };
    int slot = 0;
    // put(v), for vb=sub0 and vb=orphan: the quad's 4 vertices at offset 0.
    void (^put)(const float *) = ^(const float *q) {
        if (orphan) glBufferData(GL_ARRAY_BUFFER, 64, NULL, GL_STREAM_DRAW);
        glBufferSubData(GL_ARRAY_BUFFER, 0, 64, q);
    };

    GLuint *tex = calloc(TEXTURES, sizeof(GLuint)); glGenTextures(TEXTURES, tex);
    for (int k = 0; k < TEXTURES; k++) {
        uint32_t *px = texture_pixels(k);
        glBindTexture(GL_TEXTURE_2D, tex[k]);
        glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, TW, TH, 0, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, px);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_LINEAR); glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_LINEAR);
        free(px);
    }
    uint32_t *upload = UPLOAD ? calloc(1, UPLOAD) : NULL;
    GLuint fbo = 0, target = 0;
    if (RT) {
        glGenTextures(1, &target); glBindTexture(GL_TEXTURE_2D, target);
        glTexImage2D(GL_TEXTURE_2D, 0, GL_RGBA8, W, H, 0, GL_BGRA, GL_UNSIGNED_BYTE, NULL);
        glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MIN_FILTER, GL_NEAREST); glTexParameteri(GL_TEXTURE_2D, GL_TEXTURE_MAG_FILTER, GL_NEAREST);
        glGenFramebuffers(1, &fbo); glBindFramebuffer(GL_FRAMEBUFFER, fbo);
        glFramebufferTexture2D(GL_FRAMEBUFFER, GL_COLOR_ATTACHMENT0, GL_TEXTURE_2D, target, 0);
    }
    float v[16], r[4];
    t_start = now_s();
    for (int f = 0;; f++) {
        pump();
        if (upload) {
            for (int i = 0; i < UPLOAD / 4; i += 64) upload[i] = f;
            int rows = UPLOAD / (TW * 4); rows = rows > TH ? TH : rows < 1 ? 1 : rows;
            glBindTexture(GL_TEXTURE_2D, tex[0]);
            glTexSubImage2D(GL_TEXTURE_2D, 0, 0, 0, TW, rows, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, upload);
        }
        glBindFramebuffer(GL_FRAMEBUFFER, fbo);
        glViewport(0, 0, W, H);
        glClearColor(0, 0, 0, 1); glClear(GL_COLOR_BUFFER_BIT);
        glEnable(GL_BLEND); glBlendFunc(GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA);
        for (int i = 0; i < DRAWS; i++) {
            quad_rect(i, r); quad_vertices(r, v);
            if (ring) {
                if (slot == RING_QUADS) {
                    glBufferData(GL_ARRAY_BUFFER, vbsize, NULL, GL_STREAM_DRAW);
                    glBufferData(GL_ELEMENT_ARRAY_BUFFER, RING_QUADS * 12, NULL, GL_STREAM_DRAW);
                    slot = 0;
                }
                glBufferSubData(GL_ARRAY_BUFFER, slot * 64, 64, v);
                glBufferSubData(GL_ELEMENT_ARRAY_BUFFER, slot * 12, 12, quad_indices);
            } else put(v);
            if (UNIFORMS) {
                glUniformMatrix4fv(m0, 1, GL_FALSE, identity); glUniformMatrix4fv(m1, 1, GL_FALSE, identity); glUniform3f(k, 0, 0, 0);
            }
            if (i % TEXTURES == 0 || TEXTURES > 1) glBindTexture(GL_TEXTURE_2D, tex[i % TEXTURES]);
            if (ring) {
                glDrawElementsInstancedBaseVertex(GL_TRIANGLES, 6, GL_UNSIGNED_SHORT, (void *)(intptr_t)(slot * 12), 1, slot * 4);
                slot++;
            } else glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
        }
        if (RT) {
            glBindFramebuffer(GL_FRAMEBUFFER, 0);
            glDisable(GL_BLEND);
            float full[4] = { 0, 0, W, H }; quad_vertices(full, v);
            for (int k = 0; k < 4; k++) v[k * 4 + 3] = 1 - v[k * 4 + 3];
            glBindTexture(GL_TEXTURE_2D, target);
            if (ring) {
                if (slot == RING_QUADS) { glBufferData(GL_ARRAY_BUFFER, vbsize, NULL, GL_STREAM_DRAW); slot = 0; }
                glBufferSubData(GL_ARRAY_BUFFER, slot * 64, 64, v);
                glDrawArrays(GL_TRIANGLE_STRIP, slot * 4, 4);
                slot++;
            } else {
                put(v);
                glDrawArrays(GL_TRIANGLE_STRIP, 0, 4);
            }
        }
        [ctx flushBuffer];
        if (frame_done()) return 0;
    }
}

static NSString *const METAL_SRC = @"#include <metal_stdlib>\nusing namespace metal;\n"
    "struct V { float4 p [[position]]; float2 uv; };\n"
    "vertex V vs(uint i [[vertex_id]], constant float4 *q [[buffer(0)]], constant float2 &s [[buffer(1)]]) {"
    " V o; float4 a = q[i]; o.p = float4(a.x/s.x*2.0-1.0, 1.0-a.y/s.y*2.0, 0, 1); o.uv = a.zw; return o; }\n"
    "fragment float4 fs(V v [[stage_in]], texture2d<float> t [[texture(0)]]) {"
    " constexpr sampler smp(filter::linear); return t.sample(smp, v.uv); }\n";

static int run_metal(void) {
    id<MTLDevice> dev = MTLCreateSystemDefaultDevice();
    if (!dev) { printf("RESULT api=metal no device\n"); return 3; }
    printf("RESULT adapter %s, Metal\n", dev.name.UTF8String);
    NSWindow *window = make_window();
    NSView *view = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, W, H)];
    view.wantsLayer = YES;
    CAMetalLayer *layer = [CAMetalLayer layer];
    layer.device = dev; layer.pixelFormat = MTLPixelFormatBGRA8Unorm; layer.displaySyncEnabled = NO;
    layer.drawableSize = CGSizeMake(W, H); layer.contentsScale = 1;
    view.layer = layer; window.contentView = view;

    NSError *err = nil;
    id<MTLLibrary> lib = [dev newLibraryWithSource:METAL_SRC options:nil error:&err];
    if (!lib) { fprintf(stderr, "metal: %s\n", err.description.UTF8String); return 2; }
    MTLRenderPipelineDescriptor *pd = [MTLRenderPipelineDescriptor new];
    pd.vertexFunction = [lib newFunctionWithName:@"vs"]; pd.fragmentFunction = [lib newFunctionWithName:@"fs"];
    pd.colorAttachments[0].pixelFormat = MTLPixelFormatBGRA8Unorm;
    id<MTLRenderPipelineState> opaque = [dev newRenderPipelineStateWithDescriptor:pd error:&err];
    pd.colorAttachments[0].blendingEnabled = YES;
    pd.colorAttachments[0].sourceRGBBlendFactor = pd.colorAttachments[0].sourceAlphaBlendFactor = MTLBlendFactorSourceAlpha;
    pd.colorAttachments[0].destinationRGBBlendFactor = pd.colorAttachments[0].destinationAlphaBlendFactor = MTLBlendFactorOneMinusSourceAlpha;
    id<MTLRenderPipelineState> blend = [dev newRenderPipelineStateWithDescriptor:pd error:&err];

    NSMutableArray<id<MTLTexture>> *tex = [NSMutableArray array];
    for (int k = 0; k < TEXTURES; k++) {
        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm width:TW height:TH mipmapped:NO];
        id<MTLTexture> t = [dev newTextureWithDescriptor:td];
        uint32_t *px = texture_pixels(k);
        [t replaceRegion:MTLRegionMake2D(0, 0, TW, TH) mipmapLevel:0 withBytes:px bytesPerRow:TW * 4];
        free(px); [tex addObject:t];
    }
    MTLTextureDescriptor *rd = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatBGRA8Unorm width:W height:H mipmapped:NO];
    rd.usage = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead; rd.storageMode = MTLStorageModePrivate;
    id<MTLTexture> target = [dev newTextureWithDescriptor:rd];
    uint32_t *upload = UPLOAD ? calloc(1, UPLOAD) : NULL;
    id<MTLCommandQueue> queue = [dev newCommandQueue];
    dispatch_semaphore_t inflight = dispatch_semaphore_create(2);
    float s[2] = { W, H }, v[16], r[4];
    t_start = now_s();
    for (int f = 0;; f++) {
        @autoreleasepool {
            pump();
            dispatch_semaphore_wait(inflight, DISPATCH_TIME_FOREVER);
            if (upload) {
                for (int i = 0; i < UPLOAD / 4; i += 64) upload[i] = f;
                int rows = UPLOAD / (TW * 4); rows = rows > TH ? TH : rows < 1 ? 1 : rows;
                [tex[0] replaceRegion:MTLRegionMake2D(0, 0, TW, rows) mipmapLevel:0 withBytes:upload bytesPerRow:TW * 4];
            }
            id<CAMetalDrawable> drawable = [layer nextDrawable];
            id<MTLCommandBuffer> cb = [queue commandBuffer];
            MTLRenderPassDescriptor *rp = [MTLRenderPassDescriptor renderPassDescriptor];
            rp.colorAttachments[0].texture = RT ? target : drawable.texture;
            rp.colorAttachments[0].loadAction = MTLLoadActionClear; rp.colorAttachments[0].storeAction = MTLStoreActionStore;
            rp.colorAttachments[0].clearColor = MTLClearColorMake(0, 0, 0, 1);
            id<MTLRenderCommandEncoder> enc = [cb renderCommandEncoderWithDescriptor:rp];
            [enc setRenderPipelineState:blend];
            [enc setVertexBytes:s length:sizeof s atIndex:1];
            for (int i = 0; i < DRAWS; i++) {
                quad_rect(i, r); quad_vertices(r, v);
                [enc setVertexBytes:v length:sizeof v atIndex:0];
                if (UNIFORMS) [enc setVertexBytes:s length:sizeof s atIndex:1];
                [enc setFragmentTexture:tex[i % TEXTURES] atIndex:0];
                [enc drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
            }
            [enc endEncoding];
            if (RT) {
                MTLRenderPassDescriptor *pp = [MTLRenderPassDescriptor renderPassDescriptor];
                pp.colorAttachments[0].texture = drawable.texture;
                pp.colorAttachments[0].loadAction = MTLLoadActionDontCare; pp.colorAttachments[0].storeAction = MTLStoreActionStore;
                id<MTLRenderCommandEncoder> e2 = [cb renderCommandEncoderWithDescriptor:pp];
                [e2 setRenderPipelineState:opaque];
                [e2 setVertexBytes:s length:sizeof s atIndex:1];
                float full[4] = { 0, 0, W, H }; quad_vertices(full, v);
                [e2 setVertexBytes:v length:sizeof v atIndex:0];
                [e2 setFragmentTexture:target atIndex:0];
                [e2 drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
                [e2 endEncoding];
            }
            [cb presentDrawable:drawable];
            [cb addCompletedHandler:^(id<MTLCommandBuffer> b) { dispatch_semaphore_signal(inflight); }];
            [cb commit];
            if (frame_done()) { [cb waitUntilCompleted]; return 0; }
        }
    }
}

int main(int argc, const char **argv) {
    @autoreleasepool {
        for (int i = 1; i < argc; i++) {
            NSArray *kv = [@(argv[i]) componentsSeparatedByString:@"="];
            if (kv.count != 2) { fprintf(stderr, "usage: see the comment at the top of loadprobe.m\n"); return 2; }
            NSString *k = kv[0], *v = kv[1];
            if ([k isEqual:@"api"]) API = v;
            else if ([k isEqual:@"size"]) size_arg(v, &W, &H);
            else if ([k isEqual:@"draws"]) DRAWS = v.intValue;
            else if ([k isEqual:@"quad"]) { if ([v isEqual:@"full"]) { QW = 1 << 20; QH = 1 << 20; } else size_arg(v, &QW, &QH); }
            else if ([k isEqual:@"tex"]) size_arg(v, &TW, &TH);
            else if ([k isEqual:@"textures"]) TEXTURES = v.intValue;
            else if ([k isEqual:@"rt"]) RT = v.intValue;
            else if ([k isEqual:@"upload"]) UPLOAD = v.intValue;
            else if ([k isEqual:@"uniforms"]) UNIFORMS = v.intValue;
            else if ([k isEqual:@"vb"]) VB = v;
            else if ([k isEqual:@"warm"]) WARM = v.doubleValue;
            else if ([k isEqual:@"seconds"]) SECONDS = v.doubleValue;
            else { fprintf(stderr, "unknown argument %s\n", argv[i]); return 2; }
        }
        if (QW > W) QW = W;
        if (QH > H) QH = H;
        frame_ms = malloc(cap * sizeof(double));
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        [NSApp finishLaunching];
        if ([API isEqual:@"metal"]) return run_metal();
        if ([API isEqual:@"gl-sw"]) return run_gl(YES);
        if ([API isEqual:@"gl"]) return run_gl(NO);
        fprintf(stderr, "api must be gl-sw, gl or metal\n");
        return 2;
    }
}
