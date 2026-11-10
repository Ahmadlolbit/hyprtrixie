pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property bool awake: false
  property bool recording: false
  property bool fnLock: false
  property string fnLed: ""
  property real until: 0
  property real now: Date.now()
  readonly property int minutesLeft: awake && until > 0 ? Math.max(1, Math.ceil((until - now) / 60000)) : 0
  readonly property string runtime: Quickshell.env("XDG_RUNTIME_DIR") || ""

  function stayAwake(minutes) {
    now = Date.now();
    until = minutes > 0 ? now + minutes * 60000 : 0;
    awake = true;
  }

  function restore(text) {
    if (awake)
      return;
    try {
      const state = JSON.parse(text);
      now = Date.now();
      if (state.awake && !(state.until > 0 && state.until <= now)) {
        until = state.until || 0;
        awake = true;
      }
    } catch (error) {
      return;
    }
  }

  onAwakeChanged: {
    if (!awake)
      until = 0;
    keeper.restart();
  }
  onUntilChanged: keeper.restart()

  FileView {
    id: kept

    path: root.runtime ? root.runtime + "/hyprtrixie-awake" : ""
    printErrors: false
    onLoaded: root.restore(text())
  }

  Timer {
    id: keeper

    interval: 100
    onTriggered: kept.setText(JSON.stringify({
        awake: root.awake,
        until: root.until
      }))
  }

  Process {
    running: root.awake
    stdinEnabled: true
    command: ["systemd-inhibit", "--what=idle", "--who=Hyprtrixie", "--why=Stay awake", "cat"]
  }

  Process {
    id: recorder

    running: true
    command: ["pgrep", "-x", "wf-recorder"]
    onExited: code => root.recording = code === 0
  }

  Timer {
    interval: 3000
    running: root.recording
    repeat: true
    onTriggered: recorder.running = true
  }

  Process {
    running: true
    command: ["sh", "-c", "ls -d /sys/class/leds/*fnlock* 2>/dev/null | head -n 1"]
    stdout: StdioCollector {
      onStreamFinished: root.fnLed = text.trim()
    }
  }

  FileView {
    id: fnState

    path: root.fnLed ? root.fnLed + "/brightness" : ""
    printErrors: false
    onLoaded: root.fnLock = parseInt(text()) > 0
  }

  Timer {
    interval: 2000
    running: root.fnLed !== ""
    repeat: true
    onTriggered: fnState.reload()
  }

  Timer {
    interval: 15000
    running: root.awake && root.until > 0
    repeat: true
    onTriggered: {
      root.now = Date.now();
      if (root.now >= root.until)
        root.awake = false;
    }
  }

  IpcHandler {
    target: "recording"

    function started(): void {
      root.recording = true;
    }

    function stopped(): void {
      root.recording = false;
    }
  }

  IpcHandler {
    target: "awake"

    function on(): void {
      root.stayAwake(0);
    }

    function minutes(count: int): void {
      root.stayAwake(count);
    }

    function off(): void {
      root.awake = false;
    }

    function toggle(): void {
      root.awake = !root.awake;
    }
  }
}
