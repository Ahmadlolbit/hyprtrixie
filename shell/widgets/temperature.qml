import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property var sensors: ({})
  readonly property var cpu: typeof sensors.cpu === "number" ? sensors.cpu : null
  readonly property bool shown: cpu !== null

  function tint(value) {
    return value >= 90 ? Theme.critical : value >= 80 ? Theme.warning : Theme.text;
  }

  text: "\u{f050f} " + cpu + "°"
  color: tint(cpu)

  Process {
    id: probe

    command: ["hyprtrixie-sensors"]
    stdout: StdioCollector {
      onStreamFinished: {
        try {
          root.sensors = JSON.parse(text);
        } catch (error) {
          root.sensors = {};
        }
      }
    }
  }

  Timer {
    interval: 5000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: probe.running = true
  }

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
      Layout.preferredWidth: 240
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
