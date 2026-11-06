import QtQuick
import QtQuick.Layouts
import qs.common

Label {
  id: root

  property var bar
  readonly property var sensors: Sensors.report
  readonly property var cpu: Sensors.cpu
  readonly property bool shown: cpu !== null

  function tint(value) {
    return value >= 90 ? Theme.critical : value >= 80 ? Theme.warning : Theme.text;
  }

  text: "\u{f050f} " + cpu + "°"
  color: tint(cpu)

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "temperature"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: Theme.size(240)
      text: "Temperatures"
      font.bold: true
    }

    Repeater {
      model: root.sensors.temps || []

      RowLayout {
        required property var modelData

        Layout.fillWidth: true

        Label {
          Layout.fillWidth: true
          text: modelData.label
          color: Theme.subtext
        }

        Label {
          text: modelData.value + " °C"
          color: root.tint(modelData.value)
        }
      }
    }

    Label {
      visible: (root.sensors.fans || []).length > 0
      text: "Fans"
      font.bold: true
    }

    Repeater {
      model: root.sensors.fans || []

      RowLayout {
        required property var modelData

        Layout.fillWidth: true

        Label {
          Layout.fillWidth: true
          text: modelData.label
          color: Theme.subtext
        }

        Label {
          text: modelData.rpm + " rpm"
        }
      }
    }

    RowLayout {
      Layout.fillWidth: true
      visible: !!root.sensors.level

      Label {
        Layout.fillWidth: true
        text: "Fan level"
        color: Theme.subtext
      }

      Label {
        text: root.sensors.level || ""
      }
    }
  }
}
