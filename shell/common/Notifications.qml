pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property var history: []
  property bool quiet: false
  property int seen: 0
  property int cleared: 0
  readonly property var shownItems: history.filter(item => item.id > cleared)
  readonly property int newest: history.length ? Math.max(...history.map(item => item.id)) : 0
  readonly property int unread: history.filter(item => item.id > Math.max(seen, cleared)).length

  function character(match, name) {
    const named = {
      amp: "&",
      lt: "<",
      gt: ">",
      quot: "\"",
      apos: "'"
    };
    if (!name.startsWith("#"))
      return named[name];
    const code = name[1] === "x" ? parseInt(name.slice(2), 16) : parseInt(name.slice(1), 10);
    return code > 0 && code <= 0x10ffff ? String.fromCodePoint(code) : match;
  }

  function plain(body) {
    return body.replace(/<[^>]*>/g, "").replace(/&(amp|lt|gt|quot|apos|#[0-9]+|#x[0-9a-fA-F]+);/g, character);
  }

  function parse(text) {
    try {
      return JSON.parse(text).data[0].map(item => ({
            id: item.id.data,
            app: item["app-name"].data,
            summary: item.summary.data,
            body: plain(item.body.data),
            critical: item.urgency.data === 2
          })).reverse();
    } catch (error) {
      return [];
    }
  }

  function refresh() {
    lister.running = true;
    modes.running = true;
  }

  function open() {
    refresh();
    seen = newest;
  }

  function clear() {
    cleared = newest;
  }

  function toggleQuiet() {
    switcher.running = true;
  }

  Process {
    id: lister

    command: ["busctl", "--user", "-j", "call", "org.freedesktop.Notifications", "/fr/emersion/Mako", "fr.emersion.Mako", "ListHistory"]
    stdout: StdioCollector {
      onStreamFinished: root.history = root.parse(text)
    }
  }

  Process {
    id: modes

    command: ["makoctl", "mode"]
    stdout: StdioCollector {
      onStreamFinished: root.quiet = text.split("\n").includes("do-not-disturb")
    }
  }

  Process {
    id: switcher

    command: ["makoctl", "mode", "-t", "do-not-disturb"]
    onExited: modes.running = true
  }

  Timer {
    interval: 10000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }
}
