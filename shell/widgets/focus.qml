import QtQuick
import QtQuick.Layouts
import qs.common

Label {
  id: root

  property var bar
  readonly property bool resting: Focus.phase === "break"
  readonly property bool shown: Focus.phase !== ""

  text: (resting ? "\u{f117b} " : "\u{f051f} ") + Focus.left
  color: resting ? Theme.subtext : Theme.accent

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "focus"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: 220
      text: root.resting ? "Break" : "Focus"
      font.bold: true
    }

    Label {
      text: Focus.left + " left" + (root.resting ? "" : ", notifications held")
      color: Theme.subtext
    }

    Choice {
      Layout.fillWidth: true
      label: root.resting ? "Focus again for " + Focus.length + (Focus.length === 1 ? " minute" : " minutes") : "Start over"
      onClicked: {
        Focus.start(Focus.length);
        Popups.close(popup);
      }
    }

    Choice {
      Layout.fillWidth: true
      label: "Stop"
      onClicked: {
        Focus.stop();
        Popups.close(popup);
      }
    }
  }
}
