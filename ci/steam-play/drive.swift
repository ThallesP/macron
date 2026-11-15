import AppKit
import CoreGraphics
import Foundation
import ImageIO
import Vision

// Drives a Mac desktop where no person works (a sandbox or a CI runner): lists windows, captures
// one, clicks, presses keys, and types a value read from a file so a secret never appears in argv.

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data((message + "\n").utf8))
    exit(2)
}

struct Window {
    let id: CGWindowID
    let owner: String
    let title: String
    let bounds: CGRect
    let layer: Int
}

func listWindows() -> [Window] {
    let info = CGWindowListCopyWindowInfo([.optionOnScreenOnly], kCGNullWindowID) as? [[String: Any]] ?? []
    return info.compactMap { entry in
        guard let id = entry[kCGWindowNumber as String] as? CGWindowID,
              let boundsDict = entry[kCGWindowBounds as String] as? NSDictionary,
              let bounds = CGRect(dictionaryRepresentation: boundsDict)
        else { return nil }
        return Window(
            id: id,
            owner: entry[kCGWindowOwnerName as String] as? String ?? "",
            title: entry[kCGWindowName as String] as? String ?? "",
            bounds: bounds,
            layer: entry[kCGWindowLayer as String] as? Int ?? 0
        )
    }
}

func findWindow(owner: String, title: String) -> Window? {
    listWindows().first { $0.owner == owner && $0.title == title }
}

func post(_ event: CGEvent?) {
    guard let event else { fail("could not create an input event") }
    event.post(tap: .cghidEventTap)
    usleep(40_000)
}

// Mouse events carry no modifier keys, whatever an earlier key press left in the session's state:
// a click with Command still set is a different action.
func postMouse(_ event: CGEvent?) {
    event?.flags = []
    post(event)
}

func click(at point: CGPoint, count: Int = 1) {
    postMouse(CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left))
    usleep(150_000)
    for index in 1...count {
        let down = CGEvent(mouseEventSource: nil, mouseType: .leftMouseDown, mouseCursorPosition: point, mouseButton: .left)
        down?.setIntegerValueField(.mouseEventClickState, value: Int64(index))
        postMouse(down)
        let up = CGEvent(mouseEventSource: nil, mouseType: .leftMouseUp, mouseCursorPosition: point, mouseButton: .left)
        up?.setIntegerValueField(.mouseEventClickState, value: Int64(index))
        postMouse(up)
    }
}

func rightClick(at point: CGPoint) {
    postMouse(CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .right))
    usleep(150_000)
    postMouse(CGEvent(mouseEventSource: nil, mouseType: .rightMouseDown, mouseCursorPosition: point, mouseButton: .right))
    postMouse(CGEvent(mouseEventSource: nil, mouseType: .rightMouseUp, mouseCursorPosition: point, mouseButton: .right))
}

func scroll(at point: CGPoint, lines: Int32) {
    postMouse(CGEvent(mouseEventSource: nil, mouseType: .mouseMoved, mouseCursorPosition: point, mouseButton: .left))
    usleep(150_000)
    let step: Int32 = lines < 0 ? -1 : 1
    for _ in 0..<abs(lines) {
        postMouse(CGEvent(scrollWheelEvent2Source: nil, units: .line, wheelCount: 1, wheel1: step, wheel2: 0, wheel3: 0))
    }
}

let keyCodes: [String: CGKeyCode] = [
    "return": 36, "tab": 48, "space": 49, "delete": 51, "escape": 53,
    "left": 123, "right": 124, "down": 125, "up": 126, "pagedown": 121, "pageup": 116, "home": 115, "end": 119,
    "a": 0, "c": 8, "v": 9, "w": 13, "q": 12, "f": 3, "l": 37, "r": 15,
]

func press(_ spec: String) {
    var flags: CGEventFlags = []
    var name = spec
    while let plus = name.firstIndex(of: "+") {
        switch name[..<plus] {
        case "cmd": flags.insert(.maskCommand)
        case "shift": flags.insert(.maskShift)
        case "alt": flags.insert(.maskAlternate)
        case "ctrl": flags.insert(.maskControl)
        default: fail("unknown modifier in \(spec)")
        }
        name = String(name[name.index(after: plus)...])
    }
    guard let code = keyCodes[name] else { fail("unknown key \(name)") }
    let down = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: true)
    down?.flags = flags
    post(down)
    let up = CGEvent(keyboardEventSource: nil, virtualKey: code, keyDown: false)
    up?.flags = flags
    post(up)
}

func type(_ text: String) {
    for character in text {
        let units = Array(String(character).utf16)
        let down = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: true)
        down?.flags = []
        down?.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
        post(down)
        let up = CGEvent(keyboardEventSource: nil, virtualKey: 0, keyDown: false)
        up?.flags = []
        up?.keyboardSetUnicodeString(stringLength: units.count, unicodeString: units)
        post(up)
    }
}

// The value of KEY in a shell-style file of KEY=value lines, quotes removed.
func value(of key: String, in path: String) -> String {
    guard let text = try? String(contentsOfFile: path, encoding: .utf8) else { fail("cannot read \(path)") }
    for line in text.split(separator: "\n") {
        var row = line.trimmingCharacters(in: .whitespaces)
        if row.hasPrefix("export ") { row = String(row.dropFirst(7)) }
        guard row.hasPrefix(key + "=") else { continue }
        var found = String(row.dropFirst(key.count + 1))
        for quote in ["'", "\""] where found.count >= 2 && found.hasPrefix(quote) && found.hasSuffix(quote) {
            found = String(found.dropFirst().dropLast())
        }
        return found
    }
    fail("\(path) has no \(key)")
}

// Paints black rectangles ("x,y,w,h" in pixels from the top-left) over a PNG, so a capture can be
// kept without what a field shows.
func mask(_ path: String, rectangles: [String]) {
    let url = URL(fileURLWithPath: path)
    guard let source = CGImageSourceCreateWithURL(url as CFURL, nil),
          let image = CGImageSourceCreateImageAtIndex(source, 0, nil),
          let context = CGContext(
              data: nil, width: image.width, height: image.height, bitsPerComponent: 8, bytesPerRow: 0,
              space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
    else { fail("cannot read \(path)") }
    context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
    context.setFillColor(CGColor(red: 0, green: 0, blue: 0, alpha: 1))
    for rectangle in rectangles {
        let numbers = rectangle.split(separator: ",").compactMap { Double($0) }
        guard numbers.count == 4 else { fail("want x,y,w,h, got \(rectangle)") }
        context.fill(CGRect(x: numbers[0], y: Double(image.height) - numbers[1] - numbers[3], width: numbers[2], height: numbers[3]))
    }
    guard let masked = context.makeImage(),
          let destination = CGImageDestinationCreateWithURL(url as CFURL, "public.png" as CFString, 1, nil)
    else { fail("cannot write \(path)") }
    CGImageDestinationAddImage(destination, masked, nil)
    guard CGImageDestinationFinalize(destination) else { fail("cannot write \(path)") }
}

func capture(_ window: Window, to path: String) {
    let capture = Process()
    capture.executableURL = URL(fileURLWithPath: "/usr/sbin/screencapture")
    capture.arguments = ["-x", "-o", "-l", String(window.id), path]
    try? capture.run()
    capture.waitUntilExit()
    guard capture.terminationStatus == 0 else { fail("could not capture window \(window.owner) / \(window.title)") }
}

struct Label {
    let text: String
    let centre: CGPoint
}

// Where macOS reads `text` in a window: the screen point at the centre of each place, in reading
// order. Lines that say exactly `text` win over lines that contain it as whole words, so "PLAY"
// is the button and not the "PLAY TIME" label. Never prints what else the window says: a Steam
// window shows the account's name.
func find(_ text: String, owner: String, title: String) -> [CGPoint] {
    guard let window = findWindow(owner: owner, title: title) else { return [] }
    let path = NSTemporaryDirectory() + "drive-\(getpid())-\(window.id).png"
    capture(window, to: path)
    defer { try? FileManager.default.removeItem(atPath: path) }
    guard let source = CGImageSourceCreateWithURL(URL(fileURLWithPath: path) as CFURL, nil),
          let image = CGImageSourceCreateImageAtIndex(source, 0, nil)
    else { fail("cannot read the capture of \(owner) / \(title)") }

    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = false
    do {
        try VNImageRequestHandler(cgImage: image).perform([request])
    } catch {
        fail("could not read text in \(owner) / \(title): \(error)")
    }

    var places: [CGPoint] = []
    var exactPlaces: [CGPoint] = []
    for observation in request.results ?? [] {
        guard let candidate = observation.topCandidates(1).first else { continue }
        let line = candidate.string
        if line.trimmingCharacters(in: .whitespaces) == text {
            let box = observation.boundingBox
            exactPlaces.append(CGPoint(
                x: window.bounds.minX + box.midX * window.bounds.width,
                y: window.bounds.minY + (1 - box.midY) * window.bounds.height))
            continue
        }
        var from = line.startIndex
        while let range = line.range(of: text, range: from..<line.endIndex) {
            from = range.upperBound
            let isWordStart = range.lowerBound == line.startIndex || !line[line.index(before: range.lowerBound)].isLetter
            let isWordEnd = range.upperBound == line.endIndex || !line[range.upperBound].isLetter
            guard isWordStart, isWordEnd, let box = try? candidate.boundingBox(for: range)?.boundingBox else { continue }
            places.append(CGPoint(
                x: window.bounds.minX + box.midX * window.bounds.width,
                y: window.bounds.minY + (1 - box.midY) * window.bounds.height))
        }
    }
    return inReadingOrder(exactPlaces.isEmpty ? places : exactPlaces)
}

// Whether macOS reads `needle` anywhere in a PNG, ignoring case and spaces.
func reads(_ needle: String, inImageAt path: String) -> Bool {
    guard let source = CGImageSourceCreateWithURL(URL(fileURLWithPath: path) as CFURL, nil),
          let image = CGImageSourceCreateImageAtIndex(source, 0, nil)
    else { fail("cannot read \(path)") }
    let request = VNRecognizeTextRequest()
    request.recognitionLevel = .accurate
    request.usesLanguageCorrection = false
    do {
        try VNImageRequestHandler(cgImage: image).perform([request])
    } catch {
        fail("could not read text in \(path): \(error)")
    }
    let squeezed = { (text: String) in text.lowercased().filter { !$0.isWhitespace } }
    let wanted = squeezed(needle)
    return (request.results ?? []).contains { observation in
        observation.topCandidates(3).contains { squeezed($0.string).contains(wanted) }
    }
}

// Top to bottom, and left to right among places on one row: a button's label and a caption beside
// it sit a few points apart in height.
func inReadingOrder(_ places: [CGPoint]) -> [CGPoint] {
    var rows: [[CGPoint]] = []
    for place in places.sorted(by: { $0.y < $1.y }) {
        if let top = rows.last?.first, place.y - top.y < 25 {
            rows[rows.count - 1].append(place)
            continue
        }
        rows.append([place])
    }
    return rows.flatMap { $0.sorted { $0.x < $1.x } }
}

func point(_ arguments: ArraySlice<String>) -> CGPoint {
    let numbers = arguments.compactMap { Double($0) }
    guard numbers.count == 2 else { fail("want <x> <y>") }
    return CGPoint(x: numbers[0], y: numbers[1])
}

let arguments = CommandLine.arguments.dropFirst()
guard let command = arguments.first else {
    fail("usage: drive windows | find <owner> <title> <text> [first|last] | click-text <owner> <title> <text> [first|last] | shot <owner> <title> <png> [x,y,w,h ...] | quit <app> | shows-value <png> <file> <KEY> | activate <app> | click <x> <y> | doubleclick <x> <y> | rightclick <x> <y> | scroll <x> <y> <lines> | key <spec> | type <text> | type-value <file> <KEY> | trusted")
}
let rest = arguments.dropFirst()

switch command {
case "windows":
    for window in listWindows() {
        let b = window.bounds
        print("\(window.id)\t\(window.layer)\t\(Int(b.minX)),\(Int(b.minY)) \(Int(b.width))x\(Int(b.height))\t\(window.owner)\t\(window.title)")
    }
case "shot":
    let parts = Array(rest)
    guard parts.count >= 3 else { fail("want <owner> <title> <png> [x,y,w,h ...]") }
    guard let window = findWindow(owner: parts[0], title: parts[1]) else { fail("no window \(parts[0]) / \(parts[1])") }
    capture(window, to: parts[2])
    if parts.count > 3 { mask(parts[2], rectangles: Array(parts[3...])) }
    let b = window.bounds
    print("\(Int(b.minX)),\(Int(b.minY)) \(Int(b.width))x\(Int(b.height))")
case "find", "click-text":
    let parts = Array(rest)
    guard parts.count == 3 || parts.count == 4 else { fail("want <owner> <title> <text> [first|last|all]") }
    let found = find(parts[2], owner: parts[0], title: parts[1])
    if parts.count == 4 && parts[3] == "all" {
        for place in found { print("\(Int(place.x)) \(Int(place.y))") }
        exit(found.isEmpty ? 1 : 0)
    }
    let isLast = parts.count == 4 && parts[3] == "last"
    guard let place = isLast ? found.last : found.first else { exit(1) }
    if command == "click-text" { click(at: place) }
    print("\(Int(place.x)) \(Int(place.y))")
case "shows-value":
    let parts = Array(rest)
    guard parts.count == 3 else { fail("want <png> <file> <KEY>") }
    let secret = value(of: parts[2], in: parts[1])
    let names = [secret, String(secret.split(separator: "@").first ?? "")].filter { $0.count >= 4 }
    exit(names.contains { reads($0, inImageAt: parts[0]) } ? 0 : 1)
case "quit":
    guard let name = rest.first else { fail("want <app name>") }
    let apps = NSWorkspace.shared.runningApplications.filter { $0.localizedName == name && $0.activationPolicy == .regular }
    guard !apps.isEmpty else { fail("no running app named \(name)") }
    for app in apps { app.terminate() }
case "activate":
    guard let name = rest.first else { fail("want <app name>") }
    let apps = NSWorkspace.shared.runningApplications.filter { $0.localizedName == name }
    guard !apps.isEmpty else { fail("no running app named \(name)") }
    for app in apps { app.activate(options: [.activateAllWindows]) }
case "click": click(at: point(rest))
case "doubleclick": click(at: point(rest), count: 2)
case "rightclick": rightClick(at: point(rest))
case "scroll":
    let parts = Array(rest)
    guard parts.count == 3, let lines = Int32(parts[2]) else { fail("want <x> <y> <lines>, negative scrolls down") }
    scroll(at: point(rest.prefix(2)), lines: lines)
case "key":
    for spec in rest { press(spec) }
case "type":
    type(rest.joined(separator: " "))
case "type-value":
    let parts = Array(rest)
    guard parts.count == 2 else { fail("want <file> <KEY>") }
    type(value(of: parts[1], in: parts[0]))
case "trusted":
    print(AXIsProcessTrusted() ? "trusted" : "not trusted")
default:
    fail("unknown command \(command)")
}
