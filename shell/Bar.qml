import QtQuick
import QtQuick.Layouts
import Quickshell
import qs.common

PanelWindow {
  id: bar

  required property var modelData
  readonly property bool vertical: Config.preset === "vertical"
  readonly property bool floating: Config.preset === "floating"
  readonly property bool bottom: Config.position === "bottom"
  readonly property int gap: floating ? 8 : 0

  screen: modelData
  implicitHeight: vertical ? 0 : Theme.barHeight
  implicitWidth: vertical ? Theme.barWidth : 0
  color: "transparent"
  anchors {
    top: vertical || !bottom
    bottom: vertical || bottom
    left: true
    right: !vertical
  }
  margins {
    top: bar.floating && !bar.bottom ? bar.gap : 0
    bottom: bar.floating && bar.bottom ? bar.gap : 0
    left: bar.gap
    right: bar.gap
  }

  component Section: GridLayout {
    id: section

    property var names: []
    property var host

    flow: bar.vertical ? GridLayout.TopToBottom : GridLayout.LeftToRight
    rowSpacing: Theme.spacing
    columnSpacing: Theme.spacing

    Repeater {
      model: section.names

      Loader {
        required property string modelData

        Layout.alignment: Qt.AlignCenter
        visible: item ? item.shown !== false : false
        source: Qt.resolvedUrl("widgets/" + modelData + ".qml")
        onLoaded: item.bar = section.host
      }
    }
  }

  Rectangle {
    anchors.fill: parent
    color: Theme.panel
    radius: bar.floating ? 12 : 0
  }

  Section {
    anchors.left: bar.vertical ? undefined : parent.left
    anchors.leftMargin: 10
    anchors.verticalCenter: bar.vertical ? undefined : parent.verticalCenter
    anchors.top: bar.vertical ? parent.top : undefined
    anchors.topMargin: 10
    anchors.horizontalCenter: bar.vertical ? parent.horizontalCenter : undefined
    host: bar
    names: Config.left
  }

  Section {
    anchors.centerIn: parent
    host: bar
    names: Config.center
  }

  Section {
    anchors.right: bar.vertical ? undefined : parent.right
    anchors.rightMargin: 10
    anchors.verticalCenter: bar.vertical ? undefined : parent.verticalCenter
    anchors.bottom: bar.vertical ? parent.bottom : undefined
    anchors.bottomMargin: 10
    anchors.horizontalCenter: bar.vertical ? parent.horizontalCenter : undefined
    host: bar
    names: Config.right
  }
}
