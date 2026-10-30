import QtQuick
import Quickshell
import Quickshell.Io

Scope {
  Process {
    id: tick

    command: ["hyprtrixie-look", "tick"]
  }

  Timer {
    interval: 60000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: tick.running = true
  }
}
