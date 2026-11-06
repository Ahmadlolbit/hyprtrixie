import QtQuick
import QtQuick.Layouts
import Quickshell
import qs.common

Label {
  id: root

  property var bar
  readonly property real newest: Snapshots.newest
  readonly property bool timers: Snapshots.timers
  readonly property bool shown: Snapshots.known
  readonly property real days: Snapshots.days
  readonly property bool stale: Snapshots.stale

  text: stale ? "\u{f02da} " + (timers ? Math.floor(days) + "d" : "off") : "\u{f02da}"
  color: stale ? Theme.critical : Theme.subtext

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "snapshots"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: Theme.size(280)
      text: "Snapshots"
      font.bold: true
    }

    Label {
      text: "Newest: " + Qt.formatDateTime(new Date(root.newest * 1000), Config.clock === "12h" ? "ddd d MMM, h:mm AP" : "ddd d MMM, HH:mm")
      color: root.stale && root.timers ? Theme.critical : Theme.subtext
    }

    Label {
      visible: !root.timers
      text: "The snapshot timers are off"
      color: Theme.critical
    }

    Label {
      visible: root.stale
      text: "Check the system"
      color: Theme.accent

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
          Popups.close(popup);
          Quickshell.execDetached(["hyprtrixie-terminal", "--wait", "sudo", "hyprtrixie", "doctor"]);
        }
      }
    }
  }
}
