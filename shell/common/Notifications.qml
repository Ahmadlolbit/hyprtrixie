pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property var history: []
  property var opened: []
  property var items: []
  property bool quiet: false
  property int seen: 0
  property int cleared: 0
  readonly property var shownItems: items.filter(item => item.id > cleared)
  readonly property int newest: items.length ? Math.max(...items.map(item => item.id)) : 0
  readonly property int unread: items.filter(item => item.id > Math.max(seen, cleared)).length
  readonly property string runtime: Quickshell.env("XDG_RUNTIME_DIR") || ""

  onSeenChanged: keeper.restart()
  onClearedChanged: keeper.restart()
  onNewestChanged: startedAgain()

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

  function merged(current, past) {
    const ids = current.map(item => item.id);
    return current.concat(past.filter(item => !ids.includes(item.id))).sort((first, second) => second.id - first.id);
  }

  function startedAgain() {
    if (newest > 0 && newest < Math.max(seen, cleared)) {
      seen = 0;
      cleared = 0;
    }
  }

  function restore(text) {
    try {
      const state = JSON.parse(text);
      seen = Math.max(seen, state.seen || 0);
      cleared = Math.max(cleared, state.cleared || 0);
    } catch (error) {
      return;
    }
    startedAgain();
  }

  function refresh() {
    openLister.running = true;
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
    id: openLister

    command: ["busctl", "--user", "-j", "call", "org.freedesktop.Notifications", "/fr/emersion/Mako", "fr.emersion.Mako", "ListNotifications"]
    stdout: StdioCollector {
      onStreamFinished: {
        root.opened = root.parse(text);
        lister.running = true;
      }
    }
  }

  Process {
    id: lister

    command: ["busctl", "--user", "-j", "call", "org.freedesktop.Notifications", "/fr/emersion/Mako", "fr.emersion.Mako", "ListHistory"]
    stdout: StdioCollector {
      onStreamFinished: {
        root.history = root.parse(text);
        root.items = root.merged(root.opened, root.history);
      }
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

  FileView {
    id: kept

    path: root.runtime ? root.runtime + "/hyprtrixie-notifications" : ""
    printErrors: false
    onLoaded: root.restore(text())
  }

  Timer {
    id: keeper

    interval: 100
    onTriggered: kept.setText(JSON.stringify({
        seen: root.seen,
        cleared: root.cleared
      }))
  }

  Timer {
    interval: 10000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }
}
