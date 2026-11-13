// Probe: is there a Metal GPU, and can it compile a shader, draw, and read the result back?
import Foundation
import Metal

let devices = MTLCopyAllDevices()
print("metal devices: \(devices.count)")
for d in devices {
    print("  name=\(d.name) lowPower=\(d.isLowPower) headless=\(d.isHeadless) unifiedMemory=\(d.hasUnifiedMemory)")
    print("  metal3=\(d.supportsFamily(.metal3)) apple7=\(d.supportsFamily(.apple7)) apple8=\(d.supportsFamily(.apple8)) mac2=\(d.supportsFamily(.mac2)) raytracing=\(d.supportsRaytracing)")
    print("  recommendedMaxWorkingSetSize=\(d.recommendedMaxWorkingSetSize / 1_048_576) MB")
}
guard let dev = MTLCreateSystemDefaultDevice() else {
    print("RESULT metal: FAILED (no default device)")
    exit(1)
}
print("default device: \(dev.name)")

let src = """
#include <metal_stdlib>
using namespace metal;
struct VOut { float4 pos [[position]]; };
vertex VOut vmain(uint vid [[vertex_id]]) {
    float2 p[3] = { float2(-1.0, -1.0), float2(3.0, -1.0), float2(-1.0, 3.0) };
    VOut o;
    o.pos = float4(p[vid], 0.0, 1.0);
    return o;
}
fragment float4 fmain() { return float4(0.25, 0.5, 0.75, 1.0); }
"""

do {
    let lib = try dev.makeLibrary(source: src, options: nil)
    let pd = MTLRenderPipelineDescriptor()
    pd.vertexFunction = lib.makeFunction(name: "vmain")
    pd.fragmentFunction = lib.makeFunction(name: "fmain")
    pd.colorAttachments[0].pixelFormat = .rgba8Unorm
    let pso = try dev.makeRenderPipelineState(descriptor: pd)

    let size = 64
    let td = MTLTextureDescriptor.texture2DDescriptor(pixelFormat: .rgba8Unorm, width: size, height: size, mipmapped: false)
    td.usage = [.renderTarget]
    td.storageMode = .private
    guard let tex = dev.makeTexture(descriptor: td),
          let buf = dev.makeBuffer(length: size * size * 4, options: .storageModeShared),
          let queue = dev.makeCommandQueue(),
          let cb = queue.makeCommandBuffer() else {
        print("RESULT metal: FAILED (could not create resources)")
        exit(1)
    }

    let rp = MTLRenderPassDescriptor()
    rp.colorAttachments[0].texture = tex
    rp.colorAttachments[0].loadAction = .clear
    rp.colorAttachments[0].clearColor = MTLClearColor(red: 1, green: 0, blue: 0, alpha: 1)
    rp.colorAttachments[0].storeAction = .store
    let enc = cb.makeRenderCommandEncoder(descriptor: rp)!
    enc.setRenderPipelineState(pso)
    enc.drawPrimitives(type: .triangle, vertexStart: 0, vertexCount: 3)
    enc.endEncoding()

    let blit = cb.makeBlitCommandEncoder()!
    blit.copy(from: tex, sourceSlice: 0, sourceLevel: 0,
              sourceOrigin: MTLOrigin(x: 0, y: 0, z: 0),
              sourceSize: MTLSize(width: size, height: size, depth: 1),
              to: buf, destinationOffset: 0,
              destinationBytesPerRow: size * 4, destinationBytesPerImage: size * size * 4)
    blit.endEncoding()
    cb.commit()
    cb.waitUntilCompleted()
    if let e = cb.error {
        print("RESULT metal: FAILED (command buffer: \(e))")
        exit(1)
    }

    let p = buf.contents().bindMemory(to: UInt8.self, capacity: 4)
    print("pixel rgba=\(p[0]),\(p[1]),\(p[2]),\(p[3]) (expected about 64,128,191,255)")
    let ok = abs(Int(p[0]) - 64) <= 2 && abs(Int(p[1]) - 128) <= 2 && abs(Int(p[2]) - 191) <= 2
    print(ok ? "RESULT metal: OK (compiled a shader, drew, read it back)" : "RESULT metal: FAILED (wrong pixel)")
    exit(ok ? 0 : 1)
} catch {
    print("RESULT metal: FAILED (\(error))")
    exit(1)
}
