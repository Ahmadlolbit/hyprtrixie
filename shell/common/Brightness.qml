pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property string device: ""
  property int current: 0
  property int maximum: 0
  readonly property bool available: device !== "" && maximum > 0
  readonly property real level: available ? current / maximum : 0

  function adjust(step) {
    Quickshell.execDetached(["brightnessctl", "-e4", "-n2", "set", step]);
  }

  Process {
    running: true
    command: ["sh", "-c", "ls /sys/class/backlight | head -n 1"]
    stdout: StdioCollector {
      onStreamFinished: root.device = text.trim()
    }
  }

  FileView {
    path: root.device ? "/sys/class/backlight/" + root.device + "/max_brightness" : ""
    printErrors: false
    onLoaded: root.maximum = parseInt(text())
  }

  FileView {
    id: now
    path: root.device ? "/sys/class/backlight/" + root.device + "/brightness" : ""
    printErrors: false
    onLoaded: root.current = parseInt(text())
  }

  Timer {
    interval: 500
    running: root.device !== ""
    repeat: true
    onTriggered: now.reload()
  }
}
