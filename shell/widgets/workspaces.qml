import QtQuick
import QtQuick.Layouts
import Quickshell.Hyprland
import qs.common

RowLayout {
  id: root

  property var bar
  readonly property var monitor: bar ? Hyprland.monitorFor(bar.screen) : null

  spacing: 2

  Repeater {
    model: Hyprland.workspaces

    Rectangle {
      id: cell

      required property HyprlandWorkspace modelData
      readonly property bool current: root.monitor ? root.monitor.activeWorkspace === modelData : modelData.focused

      visible: modelData.id > 0 && (!root.monitor || modelData.monitor === root.monitor)
      implicitWidth: Math.max(22, label.implicitWidth + 12)
      implicitHeight: Theme.barHeight - 8
      color: current ? Theme.accent : "transparent"

      Label {
        id: label

        anchors.centerIn: parent
        text: cell.modelData.name
        color: cell.current ? Theme.base : cell.modelData.urgent ? Theme.critical : Theme.text
      }

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: cell.modelData.activate()
      }
    }
  }
}
