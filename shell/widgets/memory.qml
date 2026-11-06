import QtQuick
import Quickshell
import qs.common

Label {
  id: root

  property var bar
  readonly property real usage: Usage.memory

  text: "\u{f035b} " + Math.round(usage) + "%"
  color: usage > 90 ? Theme.critical : Theme.text

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Quickshell.execDetached(["kitty", "--class", "btop", "-e", "btop"])
  }
}
