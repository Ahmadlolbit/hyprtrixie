import QtQuick
import QtQuick.Layouts
import Quickshell
import qs.common

PanelWindow {
  id: bar

  required property var modelData

  screen: modelData
  implicitHeight: Theme.barHeight
  color: Theme.panel
  anchors {
    top: Config.position !== "bottom"
    bottom: Config.position === "bottom"
    left: true
    right: true
  }

  component Section: RowLayout {
    id: section

    property var names: []
    property var host

    spacing: Theme.spacing

    Repeater {
      model: section.names

      Loader {
        required property string modelData

        Layout.alignment: Qt.AlignVCenter
        visible: item ? item.shown !== false : false
        source: Qt.resolvedUrl("widgets/" + modelData + ".qml")
        onLoaded: item.bar = section.host
      }
    }
  }

  Section {
    anchors.left: parent.left
    anchors.leftMargin: 10
    anchors.verticalCenter: parent.verticalCenter
    host: bar
    names: Config.left
  }

  Section {
    anchors.centerIn: parent
    host: bar
    names: Config.center
  }

  Section {
    anchors.right: parent.right
    anchors.rightMargin: 10
    anchors.verticalCenter: parent.verticalCenter
    host: bar
    names: Config.right
  }
}
