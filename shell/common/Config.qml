pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  readonly property var defaults: ({
      position: "top",
      left: [],
      center: [],
      right: []
    })
  readonly property var system: parse(systemFile.text())
  readonly property var user: parse(userFile.text())
  readonly property string position: value("position")
  readonly property var left: value("left")
  readonly property var center: value("center")
  readonly property var right: value("right")

  function parse(text) {
    try {
      return JSON.parse(text).bar || {};
    } catch (error) {
      return {};
    }
  }

  function value(key) {
    if (user[key] !== undefined)
      return user[key];
    if (system[key] !== undefined)
      return system[key];
    return defaults[key];
  }

  FileView {
    id: systemFile
    path: "/etc/hyprtrixie/shell.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  FileView {
    id: userFile
    path: (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/hyprtrixie/shell.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }
}
