import QtQuick
import QtQuick.Layouts
import Quickshell.Services.SystemTray
import Quickshell.Widgets
import qs.common

GridLayout {
  id: root

  property var bar
  readonly property bool shown: SystemTray.items.values.length > 0

  flow: bar && bar.vertical ? GridLayout.TopToBottom : GridLayout.LeftToRight
  rowSpacing: 8
  columnSpacing: 8

  Repeater {
    model: SystemTray.items

    IconImage {
      id: icon

      required property SystemTrayItem modelData

      implicitSize: 16
      source: modelData.icon

      MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.MiddleButton | Qt.RightButton
        onClicked: mouse => {
          const item = icon.modelData;
          if (mouse.button === Qt.MiddleButton) {
            item.secondaryActivate();
          } else if (mouse.button === Qt.LeftButton && !item.onlyMenu) {
            item.activate();
          } else if (item.hasMenu && root.bar) {
            const point = icon.mapToItem(null, 0, icon.height);
            item.display(root.bar, point.x, point.y);
          }
        }
      }
    }
  }
}
