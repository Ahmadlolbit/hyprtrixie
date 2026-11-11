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

    Scope {
      id: wall

      required property HyprlandMonitor modelData
      property bool again: false

      function follow() {
        if (setter.running) {
          again = true;
          return;
        }
        if (!modelData.activeWorkspace)
          return;
        setter.command = ["hyprtrixie-background", "workspace", modelData.name, String(modelData.activeWorkspace.id)];
        setter.running = true;
      }

      Connections {
        target: wall.modelData

        function onActiveWorkspaceChanged() {
          wall.follow();
        }
      }

      Process {
        id: setter

        onExited: {
          if (!wall.again)
            return;
          wall.again = false;
          wall.follow();
        }
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
