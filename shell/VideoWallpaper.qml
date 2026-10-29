import QtQuick
import QtMultimedia
import Quickshell
import Quickshell.Wayland
import qs.common

Variants {
  model: Quickshell.screens

  PanelWindow {
    required property var modelData

    screen: modelData
    color: "black"
    exclusionMode: ExclusionMode.Ignore
    mask: Region {}
    WlrLayershell.layer: WlrLayer.Bottom
    WlrLayershell.namespace: "hyprtrixie-video"

    anchors {
      top: true
      bottom: true
      left: true
      right: true
    }

    Video {
      anchors.fill: parent
      source: "file://" + Theme.video.split("/").map(encodeURIComponent).join("/")
      loops: MediaPlayer.Infinite
      muted: true
      fillMode: VideoOutput.PreserveAspectCrop
      autoPlay: true
    }
  }
}
