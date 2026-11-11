import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Wayland
import qs.common

Variants {
  model: Quickshell.screens

  Scope {
    id: display

    required property var modelData
    readonly property var monitor: Hyprland.monitorFor(modelData)
    readonly property bool covered: monitor && monitor.activeWorkspace ? monitor.activeWorkspace.hasFullscreen : false
    readonly property bool shown: Theme.corners && !covered

    component Corner: PanelWindow {
      id: corner

      property int turn: 0
      readonly property int size: 12

      screen: display.modelData
      visible: display.shown
      implicitWidth: size
      implicitHeight: size
      color: "transparent"
      exclusionMode: ExclusionMode.Ignore
      mask: Region {}
      WlrLayershell.layer: WlrLayer.Overlay
      WlrLayershell.namespace: "hyprtrixie-corners"

      Canvas {
        anchors.fill: parent
        rotation: corner.turn
        onPaint: {
          const context = getContext("2d");
          context.reset();
          context.fillStyle = "black";
          context.beginPath();
          context.moveTo(0, 0);
          context.lineTo(width, 0);
          context.arc(width, height, width, -Math.PI / 2, Math.PI, true);
          context.closePath();
          context.fill();
        }
      }
    }

    Corner {
      anchors {
        top: true
        left: true
      }
    }

    Corner {
      turn: 90

      anchors {
        top: true
        right: true
      }
    }

    Corner {
      turn: 180

      anchors {
        bottom: true
        right: true
      }
    }

    Corner {
      turn: 270

      anchors {
        bottom: true
        left: true
      }
    }
  }
}
