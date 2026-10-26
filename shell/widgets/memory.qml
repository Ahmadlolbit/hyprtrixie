import QtQuick
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property real usage: 0

  text: "\u{f035b} " + Math.round(usage) + "%"
  color: usage > 90 ? Theme.critical : Theme.text

  FileView {
    id: meminfo

    path: "/proc/meminfo"
    onLoaded: {
      const values = {};
      for (const line of text().split("\n")) {
        const match = line.match(/^(\w+):\s+(\d+)/);
        if (match)
          values[match[1]] = Number(match[2]);
      }
      if (values.MemTotal)
        root.usage = 100 * (1 - values.MemAvailable / values.MemTotal);
    }
  }

  Timer {
    interval: 5000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: meminfo.reload()
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Quickshell.execDetached(["kitty", "--class", "btop", "-e", "btop"])
  }
}
