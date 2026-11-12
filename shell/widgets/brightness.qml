import QtQuick
import QtQuick.Layouts
import qs.common

Label {
  id: root

  property var bar
  readonly property bool shown: Brightness.available

  text: "\u{f00df} " + Math.round(Brightness.level * 100) + "%"

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
    onWheel: wheel => {
      const steps = notches.steps(wheel);
      if (steps !== 0)
        Brightness.adjust(5 * Math.abs(steps) + (steps > 0 ? "%+" : "%-"));
    }

    Notches {
      id: notches
    }
  }

  BarPopup {
    id: popup

    name: "brightness"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: Theme.size(280)
      text: "Screen"
      font.bold: true
    }

    RowLayout {
      Layout.fillWidth: true
      spacing: 10

      Label {
        text: "\u{f00df}"
      }

      Slider {
        Layout.fillWidth: true
        value: Brightness.level
        onMoved: level => Brightness.setLevel(level)
      }

      Label {
        Layout.preferredWidth: Theme.size(36)
        horizontalAlignment: Text.AlignRight
        text: Math.round(Brightness.level * 100) + "%"
      }
    }

    Label {
      visible: Brightness.keyboardAvailable
      text: "Keyboard"
      font.bold: true
    }

    RowLayout {
      visible: Brightness.keyboardAvailable
      Layout.fillWidth: true
      spacing: 10

      Label {
        text: "\u{f030c}"
      }

      Slider {
        Layout.fillWidth: true
        value: Brightness.keyboardLevel
        step: Math.max(0.05, 1 / Math.max(1, Brightness.keyboardMaximum))
        onMoved: level => Brightness.setKeyboardLevel(level)
      }

      Label {
        Layout.preferredWidth: Theme.size(36)
        horizontalAlignment: Text.AlignRight
        text: Math.round(Brightness.keyboardLevel * 100) + "%"
      }
    }
  }
}
