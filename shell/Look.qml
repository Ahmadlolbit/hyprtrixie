import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Services.UPower

Scope {
  function followPower() {
    Quickshell.execDetached(["hyprtrixie-look", "power-apply", UPower.onBattery ? "battery" : "ac"]);
  }

  Component.onCompleted: followPower()

  Connections {
    target: UPower

    function onOnBatteryChanged() {
      followPower();
    }
  }

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
