import QtQuick
import Quickshell
import qs.common

Label {
  id: root

  property var bar
  readonly property real usage: Usage.cpu

  text: "\u{f0ee0} " + Math.round(usage) + "%"
  color: usage > 85 ? Theme.critical : Theme.text

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Quickshell.execDetached(["kitty", "--class", "btop", "-e", "btop"])
  }
}
