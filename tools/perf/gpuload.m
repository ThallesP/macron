// The load of gpuload.c in native Metal, with no Wine and no DXMT, so that
// the two on one Mac show what DXMT's translation costs on the GPU.
//
//   gpuload [--mode fill|draws|mix] [--size WxH] [--seconds S]
//           [--passes P] [--draws D] [--sync N] [--out FILE]
//
// The same passes, shader math, draws and per-draw constants as gpuload.c;
// the window's layer is set up the way Wine and DXMT set theirs up
// (contentsScale 1, so the drawable is WxH pixels in a WxH-point window;
// display sync off, as DXMT sets it). --sync 1 turns display sync on, so
// presents wait for the display as gpuload.c's --sync 1 does. One command buffer a frame, at most 3 frames in
// flight. Besides the frame rate it prints the GPU time a frame from the
// command buffers' GPUStartTime and GPUEndTime, a cross-check of the Metal
// HUD's figure. Runs as x86_64 (Rosetta, like Wine) or arm64.
//
// Build: clang -O2 -fobjc-arc -arch x86_64 -arch arm64 -o gpuload gpuload.m \
//          -framework Metal -framework QuartzCore -framework AppKit
#import <AppKit/AppKit.h>
#import <Metal/Metal.h>
#import <QuartzCore/CAMetalLayer.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

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
    fflush(stdout);
}

static NSString *const shaders = @
    "#include <metal_stdlib>\n"
    "using namespace metal;\n"
    "struct V { float4 pos [[position]]; float2 uv; };\n"
    "vertex V vs_full(uint id [[vertex_id]]) {\n"
    "  V v; v.uv = float2((id << 1) & 2, id & 2);\n"
    "  v.pos = float4(v.uv * float2(2, -2) + float2(-1, 1), 0, 1); return v; }\n"
    "fragment float4 ps_fill(V v [[stage_in]], texture2d<float> t [[texture(0)]], sampler s [[sampler(0)]],\n"
    "                        constant float4 &k [[buffer(0)]]) {\n"
    "  float4 c = t.sample(s, v.uv);\n"
    "  for (int i = 0; i < 16; i++) c = c * k.x + sin(c.yzwx + v.uv.xyxy) * k.y;\n"
    "  return c; }\n"
    "fragment float4 ps_show(V v [[stage_in]], texture2d<float> t [[texture(0)]], sampler s [[sampler(0)]]) {\n"
    "  return saturate(abs(t.sample(s, v.uv))); }\n"
    "vertex float4 vs_quad(uint id [[vertex_id]], constant float4 &q [[buffer(1)]]) {\n"
    "  float2 corner = float2(id & 1, id >> 1);\n"
    "  return float4(q.xy + corner * q.z, 0, 1); }\n"
    "fragment float4 ps_quad(constant float4 &q [[buffer(1)]]) { return float4(q.w, 1 - q.w, 0.5, 1); }\n";

static int compare_double(const void *a, const void *b)
{
    double x = *(const double *)a, y = *(const double *)b;
    return x < y ? -1 : x > y;
}

static id<MTLRenderPipelineState> pipeline(id<MTLDevice> device, id<MTLLibrary> library, NSString *vs, NSString *fs, MTLPixelFormat format)
{
    MTLRenderPipelineDescriptor *desc = [MTLRenderPipelineDescriptor new];
    desc.vertexFunction = [library newFunctionWithName:vs];
    desc.fragmentFunction = [library newFunctionWithName:fs];
    desc.colorAttachments[0].pixelFormat = format;
    NSError *error = nil;
    id<MTLRenderPipelineState> state = [device newRenderPipelineStateWithDescriptor:desc error:&error];
    if (!state) {
        result("RESULT pipeline %s: FAILED %s\n", fs.UTF8String, error.localizedDescription.UTF8String);
        exit(1);
    }
    return state;
}

int main(int argc, char **argv)
{
    @autoreleasepool {
        const char *mode = "mix", *out = "gpuload.txt";
        unsigned width = 1600, height = 900, passes = 8, draws = 4000, sync = 0;
        double seconds = 20;
        for (int i = 1; i + 1 < argc; i += 2) {
            if (!strcmp(argv[i], "--mode")) mode = argv[i + 1];
            else if (!strcmp(argv[i], "--size")) sscanf(argv[i + 1], "%ux%u", &width, &height);
            else if (!strcmp(argv[i], "--seconds")) seconds = atof(argv[i + 1]);
            else if (!strcmp(argv[i], "--passes")) passes = atoi(argv[i + 1]);
            else if (!strcmp(argv[i], "--draws")) draws = atoi(argv[i + 1]);
            else if (!strcmp(argv[i], "--sync")) sync = atoi(argv[i + 1]);
            else if (!strcmp(argv[i], "--out")) out = argv[i + 1];
        }
        int fill = strcmp(mode, "draws") != 0, quads = strcmp(mode, "fill") != 0;
        result_file = fopen(out, "w");
#if defined(__x86_64__)
        const char *arch = "x86_64";
#else
        const char *arch = "arm64";
#endif
        result("RESULT config api metal-%s mode %s size %ux%u passes %u draws %u sync %u\n", arch, mode, width, height, fill ? passes : 0, quads ? draws : 0, sync);

        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(40, 40, width, height)
                                                       styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                                         backing:NSBackingStoreBuffered
                                                           defer:NO];
        window.title = @"Macron gpuload (Metal)";
        id<MTLDevice> device = MTLCreateSystemDefaultDevice();
        CAMetalLayer *layer = [CAMetalLayer layer];
        layer.device = device;
        layer.pixelFormat = MTLPixelFormatRGBA8Unorm;
        layer.framebufferOnly = YES;
        layer.contentsScale = 1.0;
        layer.drawableSize = CGSizeMake(width, height);
        layer.displaySyncEnabled = sync > 0;
        window.contentView.wantsLayer = YES;
        window.contentView.layer = layer;
        [window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        [NSApp finishLaunching];

        NSError *error = nil;
        id<MTLLibrary> library = [device newLibraryWithSource:shaders options:nil error:&error];
        if (!library) {
            result("RESULT shaders: FAILED %s\n", error.localizedDescription.UTF8String);
            return 1;
        }
        id<MTLRenderPipelineState> fill_state = pipeline(device, library, @"vs_full", @"ps_fill", MTLPixelFormatRGBA16Float);
        id<MTLRenderPipelineState> show_state = pipeline(device, library, @"vs_full", @"ps_show", MTLPixelFormatRGBA8Unorm);
        id<MTLRenderPipelineState> quad_state = pipeline(device, library, @"vs_quad", @"ps_quad", MTLPixelFormatRGBA8Unorm);
        MTLTextureDescriptor *td = [MTLTextureDescriptor texture2DDescriptorWithPixelFormat:MTLPixelFormatRGBA16Float width:width height:height mipmapped:NO];
        td.usage = MTLTextureUsageRenderTarget | MTLTextureUsageShaderRead;
        td.storageMode = MTLStorageModePrivate;
        id<MTLTexture> tex[2] = {[device newTextureWithDescriptor:td], [device newTextureWithDescriptor:td]};
        MTLSamplerDescriptor *sd = [MTLSamplerDescriptor new];
        sd.minFilter = sd.magFilter = MTLSamplerMinMagFilterLinear;
        id<MTLSamplerState> sampler = [device newSamplerStateWithDescriptor:sd];
        float k[4] = {0.999f, 0.01f, 0, 0};
        id<MTLCommandQueue> queue = [device newCommandQueue];

        {
            id<MTLCommandBuffer> cb = [queue commandBuffer];
            MTLRenderPassDescriptor *rp = [MTLRenderPassDescriptor renderPassDescriptor];
            rp.colorAttachments[0].texture = tex[0];
            rp.colorAttachments[0].loadAction = MTLLoadActionClear;
            rp.colorAttachments[0].clearColor = MTLClearColorMake(0.3, 0.6, 0.9, 1);
            rp.colorAttachments[0].storeAction = MTLStoreActionStore;
            [[cb renderCommandEncoderWithDescriptor:rp] endEncoding];
            [cb commit];
        }

        dispatch_semaphore_t in_flight = dispatch_semaphore_create(3);
        size_t cap = 1 << 20;
        double *intervals = malloc(cap * sizeof(double)), *gpu = malloc(cap * sizeof(double));
        __block size_t gpu_n = 0;
        __block int gpu_measuring = 0;
        size_t n = 0;
        int measuring = 0;
        double start = CACurrentMediaTime(), prev = start, now = start, measure_start = start;
        unsigned long long frame = 0;
        while (window.isVisible) {
            @autoreleasepool {
                NSEvent *event;
                while ((event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantPast] inMode:NSDefaultRunLoopMode dequeue:YES]))
                    [NSApp sendEvent:event];
                dispatch_semaphore_wait(in_flight, DISPATCH_TIME_FOREVER);
                id<CAMetalDrawable> drawable = [layer nextDrawable];
                id<MTLCommandBuffer> cb = [queue commandBuffer];
                int cur = 0;
                if (fill) {
                    for (unsigned p = 0; p < passes; p++) {
                        MTLRenderPassDescriptor *rp = [MTLRenderPassDescriptor renderPassDescriptor];
                        rp.colorAttachments[0].texture = tex[1 - cur];
                        rp.colorAttachments[0].loadAction = MTLLoadActionDontCare;
                        rp.colorAttachments[0].storeAction = MTLStoreActionStore;
                        id<MTLRenderCommandEncoder> enc = [cb renderCommandEncoderWithDescriptor:rp];
                        [enc setRenderPipelineState:fill_state];
                        [enc setFragmentTexture:tex[cur] atIndex:0];
                        [enc setFragmentSamplerState:sampler atIndex:0];
                        [enc setFragmentBytes:k length:sizeof(k) atIndex:0];
                        [enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
                        [enc endEncoding];
                        cur = 1 - cur;
                    }
                }
                MTLRenderPassDescriptor *rp = [MTLRenderPassDescriptor renderPassDescriptor];
                rp.colorAttachments[0].texture = drawable.texture;
                rp.colorAttachments[0].loadAction = fill ? MTLLoadActionDontCare : MTLLoadActionClear;
                rp.colorAttachments[0].clearColor = MTLClearColorMake(0.1, 0.1, 0.15, 1);
                rp.colorAttachments[0].storeAction = MTLStoreActionStore;
                id<MTLRenderCommandEncoder> enc = [cb renderCommandEncoderWithDescriptor:rp];
                if (fill) {
                    [enc setRenderPipelineState:show_state];
                    [enc setFragmentTexture:tex[cur] atIndex:0];
                    [enc setFragmentSamplerState:sampler atIndex:0];
                    [enc drawPrimitives:MTLPrimitiveTypeTriangle vertexStart:0 vertexCount:3];
                }
                if (quads) {
                    [enc setRenderPipelineState:quad_state];
                    for (unsigned d = 0; d < draws; d++) {
                        float q[4] = {-1 + 2.0f * (float)((d * 37 + frame) % 100) / 100, -1 + 2.0f * (float)((d * 61) % 100) / 100, 0.02f,
                                      (float)(d % 16) / 16};
                        [enc setVertexBytes:q length:sizeof(q) atIndex:1];
                        [enc setFragmentBytes:q length:sizeof(q) atIndex:1];
                        [enc drawPrimitives:MTLPrimitiveTypeTriangleStrip vertexStart:0 vertexCount:4];
                    }
                }
                [enc endEncoding];
                [cb presentDrawable:drawable];
                [cb addCompletedHandler:^(id<MTLCommandBuffer> done) {
                    if (gpu_measuring && gpu_n < cap) gpu[gpu_n++] = 1000.0 * (done.GPUEndTime - done.GPUStartTime);
                    dispatch_semaphore_signal(in_flight);
                }];
                [cb commit];
                frame++;
                now = CACurrentMediaTime();
                if (!measuring && now - start >= 2) {
                    measuring = 1;
                    gpu_measuring = 1;
                    measure_start = now;
                } else if (measuring) {
                    if (n < cap) intervals[n++] = 1000.0 * (now - prev);
                    if (now - measure_start >= seconds) break;
                }
                prev = now;
            }
        }
        gpu_measuring = 0;
        // Wait for the frames in flight, then give the semaphore its count
        // back: libdispatch traps when one is freed below its first value.
        for (int i = 0; i < 3; i++) dispatch_semaphore_wait(in_flight, DISPATCH_TIME_FOREVER);
        for (int i = 0; i < 3; i++) dispatch_semaphore_signal(in_flight);
        double total = now - measure_start;
        qsort(intervals, n, sizeof(double), compare_double);
        size_t g = gpu_n;
        qsort(gpu, g, sizeof(double), compare_double);
        if (n)
            result("RESULT fps %.2f frames %zu interval_ms p50 %.3f p95 %.3f p99 %.3f\n", n / total, n, intervals[n / 2], intervals[n * 95 / 100],
                   intervals[n * 99 / 100]);
        if (g)
            result("RESULT gpu_ms p50 %.3f p95 %.3f frames %zu\n", gpu[g / 2], gpu[g * 95 / 100], g);
    }
    return 0;
}
