import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io

Scope {
  Process {
    id: tick

    command: ["hyprtrixie-look", "tick"]
  }

  Connections {
    target: Hyprland

    function onFocusedWorkspaceChanged() {
      Quickshell.execDetached(["hyprtrixie-background", "workspaces"]);
    }
  }

  Timer {
    interval: 60000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: tick.running = true
  }
}
