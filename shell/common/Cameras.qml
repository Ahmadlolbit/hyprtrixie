pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property var held: []

  Process {
    id: watcher

    running: true
    command: ["hyprtrixie-sensors", "cameras"]
    stdout: SplitParser {
      onRead: data => root.held = data === "" ? [] : data.split("\t")
    }
    onExited: code => {
      root.held = [];
      if (code !== 2)
        restart.start();
    }
  }

  Timer {
    id: restart

    interval: 5000
    onTriggered: watcher.running = true
  }
}
