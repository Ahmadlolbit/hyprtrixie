import QtQuick
import qs.common

Label {
  id: root

  property var bar
  readonly property int count: Updates.count
  readonly property bool shown: count > 0

  text: "\u{f06b0} " + count
  color: Theme.warning

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Updates.upgrade()
  }
}
