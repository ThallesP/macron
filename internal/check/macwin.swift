import AppKit
import CoreGraphics
import Foundation
import Vision

struct WindowInfo: Codable {
    let id: Int
    let pid: Int
    let owner: String
    let title: String
    let x: Double
    let y: Double
    let width: Double
    let height: Double
    let layer: Int
}

struct TextLine: Codable {
    let text: String
    let confidence: Float
}

func printJSON<T: Encodable>(_ value: T) {
    let data = try! JSONEncoder().encode(value)
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write("\n".data(using: .utf8)!)
}

func windows() {
    let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
    let list = CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] ?? []
    let found = list.map { entry -> WindowInfo in
        let bounds = entry[kCGWindowBounds as String] as? [String: Double] ?? [:]
        return WindowInfo(
            id: entry[kCGWindowNumber as String] as? Int ?? 0,
            pid: entry[kCGWindowOwnerPID as String] as? Int ?? 0,
            owner: entry[kCGWindowOwnerName as String] as? String ?? "",
            title: entry[kCGWindowName as String] as? String ?? "",
            x: bounds["X"] ?? 0,
            y: bounds["Y"] ?? 0,
            width: bounds["Width"] ?? 0,
            height: bounds["Height"] ?? 0,
            layer: entry[kCGWindowLayer as String] as? Int ?? 0
        )
    }
    printJSON(found)
}

func ocr(_ path: String) {
    guard let image = NSImage(contentsOfFile: path),
          let cgImage = image.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
        FileHandle.standardError.write("cannot read \(path)\n".data(using: .utf8)!)
        exit(1)
    }
    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = false
    try! VNImageRequestHandler(cgImage: cgImage).perform([request])
    let lines = (request.results ?? []).compactMap { observation -> TextLine? in
        guard let best = observation.topCandidates(1).first else { return nil }
        return TextLine(text: best.string, confidence: best.confidence)
    }
    printJSON(lines)
}

let args = CommandLine.arguments
switch args.count > 1 ? args[1] : "" {
case "windows":
    windows()
case "park-cursor":
    CGWarpMouseCursorPosition(CGPoint(x: 0, y: 0))
case "ocr" where args.count == 3:
    ocr(args[2])
default:
    FileHandle.standardError.write("usage: macwin windows | macwin park-cursor | macwin ocr <image>\n".data(using: .utf8)!)
    exit(2)
}
