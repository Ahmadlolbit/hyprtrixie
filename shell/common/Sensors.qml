pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property var report: ({})
  readonly property bool detailed: Popups.open !== null && Popups.open.name === "temperature"
  readonly property var cpu: typeof report.cpu === "number" ? report.cpu : null

  onDetailedChanged: {
    if (detailed)
      probe.running = true;
  }

  Process {
    id: probe

    command: root.detailed ? ["hyprtrixie-sensors"] : ["hyprtrixie-sensors", "cpu"]
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

  Timer {
    interval: 5000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: probe.running = true
  }
}
