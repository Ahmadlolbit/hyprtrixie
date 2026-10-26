import QtQuick
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property real usage: 0
  property var last: null

  text: "\u{f0ee0} " + Math.round(usage) + "%"
  color: usage > 85 ? Theme.critical : Theme.text

  FileView {
    id: stat

    path: "/proc/stat"
    onLoaded: {
      const fields = text().split("\n")[0].trim().split(/\s+/).slice(1).map(Number);
      const idle = fields[3] + fields[4];
      const total = fields.reduce((sum, value) => sum + value, 0);
      if (root.last)
        root.usage = 100 * (1 - (idle - root.last.idle) / Math.max(1, total - root.last.total));
      root.last = {
        idle: idle,
        total: total
      };
    }
  }

  Timer {
    interval: 2000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: stat.reload()
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Quickshell.execDetached(["kitty", "--class", "btop", "-e", "btop"])
  }
}
