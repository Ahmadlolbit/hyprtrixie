pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  readonly property int breakMinutes: 5
  readonly property string chime: "/usr/share/sounds/freedesktop/stereo/alarm-clock-elapsed.oga"
  readonly property string statePath: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/hyprtrixie/focus"
  property string phase: ""
  property int length: 25
  property real until: 0
  property bool pending: false
  property real now: Date.now()
  readonly property int secondsLeft: phase ? Math.max(0, Math.round((until - now) / 1000)) : 0
  readonly property string left: Math.floor(secondsLeft / 60) + ":" + String(secondsLeft % 60).padStart(2, "0")

  function start(minutes) {
    if (!(minutes > 0))
      return;
    now = Date.now();
    length = minutes;
    phase = "focus";
    until = now + minutes * 60000;
    Quickshell.execDetached(["makoctl", "mode", "-a", "focus"]);
    save();
  }

  function stop() {
    Quickshell.execDetached(["makoctl", "mode", "-r", "focus"]);
    phase = "";
    until = 0;
    save();
  }

  function announce(title, body, loud) {
    Quickshell.execDetached(["sh", "-c", (loud ? "makoctl mode -r focus >/dev/null 2>&1; " : "") + "notify-send -a Focus \"$1\" \"$2\"; pw-play \"$3\"", "focus", title, body, chime]);
  }

  function tick() {
    now = Date.now();
    if (!phase || now < until)
      return;
    if (phase === "focus") {
      announce("Focus done", "Take a " + breakMinutes + "-minute break.", true);
      phase = "break";
      until = now + breakMinutes * 60000;
    } else {
      announce("Break's over", "Start another focus session from the menu.", false);
      phase = "";
      until = 0;
    }
    save();
  }

  function restore(text) {
    if (phase)
      return;
    try {
      const state = JSON.parse(text);
      phase = state.phase || "";
      until = state.until || 0;
      length = state.length || length;
    } catch (error) {
      return;
    }
    now = Date.now();
    if (phase && now >= until) {
      Quickshell.execDetached(["makoctl", "mode", "-r", "focus"]);
      phase = "";
      until = 0;
      save();
    } else if (phase === "focus") {
      Quickshell.execDetached(["makoctl", "mode", "-a", "focus"]);
    }
  }

  function save() {
    if (write.running) {
      pending = true;
      return;
    }
    write.command = ["sh", "-c", "mkdir -p \"${1%/*}\" && printf '%s\\n' \"$2\" >\"$1\"", "focus", statePath, JSON.stringify({
        phase: phase,
        until: until,
        length: length
      })];
    write.running = true;
  }

  Process {
    id: write

    onExited: {
      if (!root.pending)
        return;
      root.pending = false;
      root.save();
    }
  }

  FileView {
    path: root.statePath
    printErrors: false
    onLoaded: root.restore(text())
  }

  Timer {
    interval: 1000
    running: root.phase !== ""
    repeat: true
    onTriggered: root.tick()
  }

  IpcHandler {
    target: "focus"

    function start(minutes: int): void {
      root.start(minutes);
    }

    function stop(): void {
      root.stop();
    }
  }
}
