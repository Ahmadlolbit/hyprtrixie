import QtQuick
import Quickshell
import Quickshell.Services.Pipewire
import Quickshell.Wayland
import qs.common

Scope {
  id: root

  readonly property PwNode sink: Pipewire.defaultAudioSink
  property string icon: ""
  property real level: 0
  property bool shown: false
  property bool armed: false

  function show(icon, level) {
    if (!armed)
      return;
    root.icon = icon;
    root.level = level;
    shown = true;
    hide.restart();
  }

  function showVolume() {
    const audio = sink.audio;
    show(audio.muted ? "\u{f0581}" : "\u{f057e}", audio.muted ? 0 : audio.volume);
  }

  PwObjectTracker {
    objects: [root.sink]
  }

  Connections {
    target: root.sink && root.sink.audio ? root.sink.audio : null

    function onVolumesChanged() {
      root.showVolume();
    }

    function onMutedChanged() {
      root.showVolume();
    }
  }

  Connections {
    target: Brightness

    function onCurrentChanged() {
      root.show("\u{f00df}", Brightness.level);
    }
  }

  Timer {
    interval: 2000
    running: true
    onTriggered: root.armed = true
  }

  Timer {
    id: hide

    interval: 1500
    onTriggered: root.shown = false
  }

  PanelWindow {
    visible: root.shown
    anchors.bottom: true
    margins.bottom: 96
    exclusiveZone: 0
    implicitWidth: 280
    implicitHeight: 52
    color: "transparent"
    mask: Region {}
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "hyprtrixie-osd"

    Rectangle {
      anchors.fill: parent
      color: Theme.base
      border.color: Theme.accent
      border.width: 2

      Label {
        id: glyph

        anchors.left: parent.left
        anchors.leftMargin: 16
        anchors.verticalCenter: parent.verticalCenter
        text: root.icon
        font.pixelSize: 20
      }

      Rectangle {
        id: track

        anchors.left: glyph.right
        anchors.leftMargin: 14
        anchors.right: value.left
        anchors.rightMargin: 14
        anchors.verticalCenter: parent.verticalCenter
        height: 6
        color: Theme.surface

        Rectangle {
          width: track.width * Math.max(0, Math.min(1, root.level))
          height: parent.height
          color: Theme.accent
        }
      }

      Label {
        id: value

        anchors.right: parent.right
        anchors.rightMargin: 16
        anchors.verticalCenter: parent.verticalCenter
        width: 36
        horizontalAlignment: Text.AlignRight
        text: Math.round(root.level * 100)
      }
    }
  }
}
