import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property real newest: 0
  property real now: Date.now() / 1000
  property bool timers: true
  property bool known: false
  readonly property bool shown: known
  readonly property real days: newest ? (now - newest) / 86400 : 0
  readonly property bool stale: !timers || days > 2

  text: stale ? "\u{f02da} " + (timers ? Math.floor(days) + "d" : "off") : "\u{f02da}"
  color: stale ? Theme.critical : Theme.subtext

  Process {
    id: check

    command: ["sh", "-c", "command -v snapper >/dev/null || exit 0; snapper --csvout --no-headers -c root list --columns date 2>/dev/null | tail -n 1; systemctl is-active --quiet snapper-timeline.timer && echo timers-on || echo timers-off"]
    stdout: StdioCollector {
      onStreamFinished: {
        const lines = text.trim().split("\n").filter(line => line !== "");
        const status = lines.pop() || "";
        const date = lines.length ? Date.parse(lines[lines.length - 1].replace(" ", "T")) : NaN;
        root.known = status !== "" && !isNaN(date);
        root.timers = status === "timers-on";
        root.newest = isNaN(date) ? 0 : date / 1000;
      }
    }
  }

  Timer {
    interval: 1800000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: {
      root.now = Date.now() / 1000;
      check.running = true;
    }
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "snapshots"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: 280
      text: "Snapshots"
      font.bold: true
    }

    Label {
      text: "Newest: " + Qt.formatDateTime(new Date(root.newest * 1000), Config.clock === "12h" ? "ddd d MMM, h:mm AP" : "ddd d MMM, HH:mm")
      color: root.stale && root.timers ? Theme.critical : Theme.subtext
    }

    Label {
      visible: !root.timers
      text: "The snapshot timers are off"
      color: Theme.critical
    }

    Label {
      visible: root.stale
      text: "Check the system"
      color: Theme.accent

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
          Popups.close(popup);
          Quickshell.execDetached(["hyprtrixie-terminal", "--wait", "sudo", "hyprtrixie", "doctor"]);
        }
      }
    }
  }
}
