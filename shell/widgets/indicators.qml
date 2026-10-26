import QtQuick
import QtQuick.Layouts
import Quickshell.Services.Pipewire
import Quickshell.Wayland
import qs.common

RowLayout {
  id: root

  property var bar
  readonly property PwNode source: Pipewire.defaultAudioSource

  spacing: 8

  PwObjectTracker {
    objects: [root.source]
  }

  IdleInhibitor {
    window: root.bar
    enabled: Session.awake
  }

  Label {
    text: Session.awake ? "\u{f0176}" : "\u{f06ca}"
    color: Session.awake ? Theme.warning : Theme.subtext

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: Session.awake = !Session.awake
    }
  }

  Label {
    visible: root.source && root.source.audio ? root.source.audio.muted : false
    text: "\u{f036d}"
    color: Theme.critical
  }
}
