pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property real newest: 0
  property real now: Date.now() / 1000
  property bool timers: true
  property bool known: false
  readonly property real days: newest ? (now - newest) / 86400 : 0
  readonly property bool stale: !timers || days > 2

  Process {
    id: check

    command: ["sh", "-c", "command -v snapper >/dev/null || exit 0; ! grep -qx 'SNAPSHOTS=.no.' /etc/hyprtrixie/hyprtrixie.conf 2>/dev/null || exit 0; snapper --csvout --no-headers -c root list --columns date 2>/dev/null | tail -n 1; systemctl is-active --quiet snapper-timeline.timer && echo timers-on || echo timers-off"]
    stdout: StdioCollector {
      onStreamFinished: {
        const lines = text.trim().split("\n").filter(line => line !== "");
        const status = lines.pop() || "";
        const date = lines.length ? Date.parse(lines[lines.length - 1].replace(" ", "T")) : NaN;
        root.known = status !== "" && !isNaN(date);
        root.timers = status === "timers-on";
        root.newest = isNaN(date) ? 0 : date / 1000;
      }
    }
  }

  Timer {
    interval: 1800000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: {
      root.now = Date.now() / 1000;
      check.running = true;
    }
  }
}
