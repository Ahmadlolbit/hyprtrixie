pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property bool awake: false
  property real until: 0
  property real now: Date.now()
  readonly property int minutesLeft: awake && until > 0 ? Math.max(1, Math.ceil((until - now) / 60000)) : 0

  function stayAwake(minutes) {
    now = Date.now();
    until = minutes > 0 ? now + minutes * 60000 : 0;
    awake = true;
  }

  onAwakeChanged: {
    if (!awake)
      until = 0;
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
