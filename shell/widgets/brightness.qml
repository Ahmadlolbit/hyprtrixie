import QtQuick
import qs.common

Label {
  property var bar
  readonly property bool shown: Brightness.available

  text: "\u{f00df} " + Math.round(Brightness.level * 100) + "%"

  MouseArea {
    anchors.fill: parent
    onWheel: wheel => Brightness.adjust(wheel.angleDelta.y > 0 ? "5%+" : "5%-")
  }
}
