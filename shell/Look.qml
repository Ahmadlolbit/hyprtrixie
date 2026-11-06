import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Services.UPower
import qs.common

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

  Variants {
    model: Theme.looks.workspaces === "yes" ? Hyprland.monitors.values : []

    Connections {
      required property HyprlandMonitor modelData

      target: modelData

      function onActiveWorkspaceChanged() {
        if (modelData.activeWorkspace)
          Quickshell.execDetached(["hyprtrixie-background", "workspace", modelData.name, String(modelData.activeWorkspace.id)]);
      }
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
