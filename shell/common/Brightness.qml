pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property string device: ""
  property int current: 0
  property int maximum: 0
  property int target: 0
  property string keyboard: ""
  property int keyboardCurrent: 0
  property int keyboardMaximum: 0
  property int keyboardTarget: -1
  readonly property bool available: device !== "" && maximum > 0
  readonly property real level: available ? current / maximum : 0
  readonly property bool keyboardAvailable: keyboard !== "" && keyboardMaximum > 0
  readonly property real keyboardLevel: keyboardAvailable ? keyboardCurrent / keyboardMaximum : 0

  function adjust(step) {
    Quickshell.execDetached(["brightnessctl", "-e4", "-n2", "set", step]);
  }

  function setLevel(level) {
    target = Math.round(level * maximum);
    if (!apply.running)
      apply.start();
  }

  function setKeyboardLevel(level) {
    const value = Math.round(level * keyboardMaximum);
    if (value === keyboardTarget)
      return;
    keyboardTarget = value;
    Quickshell.execDetached(["brightnessctl", "-d", keyboard, "set", String(value)]);
  }

  Timer {
    id: apply

    interval: 50
    onTriggered: Quickshell.execDetached(["brightnessctl", "-n2", "set", String(root.target)])
  }

  Process {
    running: true
    command: ["sh", "-c", "ls /sys/class/backlight | head -n 1"]
    stdout: StdioCollector {
      onStreamFinished: root.device = text.trim()
    }
  }

  Process {
    running: true
    command: ["sh", "-c", "ls /sys/class/leds | grep -m 1 kbd_backlight"]
    stdout: StdioCollector {
      onStreamFinished: root.keyboard = text.trim()
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

  FileView {
    path: root.keyboard ? "/sys/class/leds/" + root.keyboard + "/max_brightness" : ""
    printErrors: false
    onLoaded: root.keyboardMaximum = parseInt(text())
  }

  FileView {
    id: keys

    path: root.keyboard ? "/sys/class/leds/" + root.keyboard + "/brightness" : ""
    printErrors: false
    onLoaded: root.keyboardCurrent = parseInt(text())
  }

  Timer {
    interval: 500
    running: root.device !== "" || root.keyboard !== ""
    repeat: true
    onTriggered: {
      if (root.device)
        now.reload();
      if (root.keyboard)
        keys.reload();
    }
  }
}
