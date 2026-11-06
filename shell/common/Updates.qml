pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property int count: 0

  function refresh() {
    counter.running = true;
  }

  function upgrade() {
    Quickshell.execDetached(["sh", "-c", "hyprtrixie-terminal --wait hyprtrixie update; qs -p /usr/share/hyprtrixie/shell ipc call updates refresh"]);
  }

  Process {
    id: counter

    command: ["sh", "-c", "apt list --upgradable 2>/dev/null | grep -c upgradable"]
    stdout: StdioCollector {
      onStreamFinished: root.count = parseInt(text.trim()) || 0
    }
  }

  Timer {
    interval: 1800000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }

  IpcHandler {
    target: "updates"

    function refresh(): void {
      root.refresh();
    }
  }
}
