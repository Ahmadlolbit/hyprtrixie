import QtQuick
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property int count: 0
  readonly property bool shown: count > 0

  text: "\u{f06b0} " + count
  color: Theme.warning

  Process {
    id: counter

    command: ["sh", "-c", "apt list --upgradable 2>/dev/null | grep -c upgradable"]
    stdout: StdioCollector {
      onStreamFinished: root.count = parseInt(text.trim()) || 0
    }
  }

  Process {
    id: upgrade

    command: ["hyprtrixie-terminal", "--wait", "sh", "-c", "sudo apt update && sudo apt full-upgrade"]
    onExited: counter.running = true
  }

  Timer {
    interval: 1800000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: counter.running = true
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: upgrade.running = true
  }
}
