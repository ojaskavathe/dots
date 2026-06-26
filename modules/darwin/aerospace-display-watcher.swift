// Runs a rebalance script whenever the display configuration changes
// (monitor added/removed/rearranged). Stays fully idle between events.
//
// We drive this through NSApplication on purpose: a bare CoreGraphics process
// with no window-server connection does NOT reliably receive display
// callbacks. NSApplication establishes that connection, after which both the
// AppKit screen-parameters notification (primary) and the CoreGraphics
// reconfiguration callback (backup) fire as expected.

import AppKit
import CoreGraphics
import Foundation

let scriptPath = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : ""

func logErr(_ msg: String) {
    FileHandle.standardError.write(Data("display-watcher: \(msg)\n".utf8))
}

// Coalesce the burst of events macOS emits during one hotplug and let the new
// arrangement (and AeroSpace's own reaction to it) settle before rebalancing.
var pending: DispatchWorkItem?

func scheduleRebalance(_ reason: String) {
    logErr("\(reason): scheduling rebalance")
    pending?.cancel()
    let work = DispatchWorkItem {
        guard !scriptPath.isEmpty else { return }
        let p = Process()
        p.executableURL = URL(fileURLWithPath: scriptPath)
        do {
            try p.run()
        } catch {
            logErr("exec failed: \(error)")
        }
    }
    pending = work
    DispatchQueue.main.asyncAfter(deadline: .now() + 1.0, execute: work)
}

let app = NSApplication.shared
app.setActivationPolicy(.prohibited)

// Primary signal.
NotificationCenter.default.addObserver(
    forName: NSApplication.didChangeScreenParametersNotification,
    object: nil,
    queue: .main
) { _ in scheduleRebalance("screen-params") }

// Backup signal.
let cgCallback: CGDisplayReconfigurationCallBack = { _, flags, _ in
    let interesting: CGDisplayChangeSummaryFlags = [
        .addFlag, .removeFlag, .enabledFlag, .disabledFlag,
        .movedFlag, .desktopShapeChangedFlag,
    ]
    if flags.rawValue & interesting.rawValue != 0 {
        scheduleRebalance("cg-reconfig")
    }
}
CGDisplayRegisterReconfigurationCallback(cgCallback, nil)

// Rebalance once at startup so the layout is correct on login.
scheduleRebalance("startup")

app.run()
