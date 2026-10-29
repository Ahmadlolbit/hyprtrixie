import QtQuick
import Quickshell
import Quickshell.Wayland
import qs.common

Variants {
  model: Quickshell.screens

  PanelWindow {
    id: overlay

    required property var modelData
    readonly property int size: 12

    screen: modelData
    visible: Theme.corners
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    mask: Region {}
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "hyprtrixie-corners"

    anchors {
      top: true
      bottom: true
      left: true
      right: true
    }

    component Corner: Canvas {
      width: overlay.size
      height: overlay.size
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

    Corner {
      anchors.top: parent.top
      anchors.left: parent.left
    }

    Corner {
      anchors.top: parent.top
      anchors.right: parent.right
      rotation: 90
    }

    Corner {
      anchors.bottom: parent.bottom
      anchors.right: parent.right
      rotation: 180
    }

    Corner {
      anchors.bottom: parent.bottom
      anchors.left: parent.left
      rotation: 270
    }
  }
}
