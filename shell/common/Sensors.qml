pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property var report: ({})
  property string cpuFile: ""
  property var cpu: null
  readonly property bool detailed: Popups.open !== null && Popups.open.name === "temperature"

  function read() {
    probe.running = true;
  }

  function degrees(text) {
    const value = parseInt(text, 10);
    return isNaN(value) ? null : Math.round(value / 1000);
  }

  onDetailedChanged: {
    if (detailed)
      read();
  }

  Process {
    id: probe

    command: ["hyprtrixie-sensors"]
    stdout: StdioCollector {
      onStreamFinished: {
        try {
          root.report = JSON.parse(text);
        } catch (error) {
          root.report = {};
        }
      }
    }
  }

  Process {
    id: finder

    command: ["hyprtrixie-sensors", "cpu-file"]
    running: true
    stdout: StdioCollector {
      onStreamFinished: root.cpuFile = text.trim()
    }
  }

  FileView {
    id: cpuInput

    path: root.cpuFile
    printErrors: false
    onLoaded: root.cpu = root.degrees(text())
    onLoadFailed: {
      root.cpu = null;
      finder.running = true;
    }
  }

  Timer {
    interval: 5000
    running: true
    repeat: true
    onTriggered: {
      if (root.cpuFile !== "")
        cpuInput.reload();
      if (root.detailed)
        root.read();
    }
  }
}
