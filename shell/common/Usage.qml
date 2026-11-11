pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property real cpu: 0
  property real memory: 0
  property var last: null

  FileView {
    id: stat

    path: "/proc/stat"
    onLoaded: {
      const fields = text().split("\n")[0].trim().split(/\s+/).slice(1, 9).map(Number);
      const idle = fields[3] + fields[4];
      const total = fields.reduce((sum, value) => sum + value, 0);
      if (root.last)
        root.cpu = 100 * (1 - (idle - root.last.idle) / Math.max(1, total - root.last.total));
      root.last = {
        idle: idle,
        total: total
      };
    }
  }

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
        root.memory = 100 * (1 - values.MemAvailable / values.MemTotal);
    }
  }

  Timer {
    interval: 2000
    running: !Screens.dark && !Theme.zen
    repeat: true
    triggeredOnStart: true
    onTriggered: stat.reload()
  }

  Timer {
    interval: 5000
    running: !Screens.dark && !Theme.zen
    repeat: true
    triggeredOnStart: true
    onTriggered: meminfo.reload()
  }
}
