pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  readonly property var defaults: ({
      position: "top",
      clock: "24h",
      style: "sharp",
      left: ["workspaces", "window"],
      center: ["clock"],
      right: ["indicators", "tray", "cpu", "memory", "network", "bluetooth", "audio", "brightness", "power-profile", "battery"],
      preset: "full"
    })
  readonly property var presets: ({
      minimal: {
        left: ["workspaces"],
        right: ["indicators", "tray", "network", "bluetooth", "audio", "battery"]
      },
      vertical: {
        left: ["workspaces"],
        right: ["indicators", "tray", "network", "bluetooth", "audio", "brightness", "battery"]
      }
    })
  readonly property var system: parse(systemFile.text())
  readonly property var user: parse(userFile.text())
  readonly property string position: value("position")
  readonly property string clock: value("clock")
  readonly property string style: value("style")
  readonly property string preset: value("preset")
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
    const layout = key === "preset" ? null : presets[value("preset")];
    if (layout && layout[key] !== undefined)
      return layout[key];
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
