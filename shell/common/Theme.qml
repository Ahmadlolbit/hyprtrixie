pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  readonly property var palette: parse(file.text())
  readonly property color base: palette.background || "#1e1e2e"
  readonly property color surface: palette.lighter_background || "#313244"
  readonly property color overlay: palette.selection || "#45475a"
  readonly property color text: palette.foreground || "#cdd6f4"
  readonly property color subtext: Qt.tint(text, Qt.alpha(base, 0.3))
  readonly property color accent: palette.accent || "#89b4fa"
  readonly property color good: palette.green || "#a6e3a1"
  readonly property color warning: palette.yellow || "#f9e2af"
  readonly property color critical: palette.red || "#f38ba8"
  readonly property var look: settings(lookFile.text())
  readonly property string font: look.font || "JetBrainsMono Nerd Font"
  readonly property bool glass: look.glass === "yes"
  readonly property color panel: Qt.alpha(base, glass ? 0.8 : 1)
  readonly property var looks: Object.assign({}, lines(systemLook.text()), lines(userLook.text()))
  readonly property bool corners: looks.corners === "yes"
  readonly property int fontSize: 13
  readonly property int barHeight: 30
  readonly property int barWidth: 60
  readonly property int spacing: 14
  readonly property int radius: Config.style === "soft" ? 10 : 0

  function parse(text) {
    const colors = {};
    for (const line of text.split("\n")) {
      const match = line.match(/^\s*([a-z0-9_]+)\s*=\s*"(#[0-9A-Fa-f]{6})"/);
      if (match)
        colors[match[1]] = match[2];
    }
    return colors;
  }

  function settings(text) {
    const values = {};
    for (const line of text.split("\n")) {
      const match = line.match(/^\s*([a-z0-9_]+)\s*=\s*"([^"]*)"/);
      if (match)
        values[match[1]] = match[2];
    }
    return values;
  }

  FileView {
    id: file

    path: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/omarchy/current/theme/colors.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  function lines(text) {
    const values = {};
    for (const line of text.split("\n")) {
      const at = line.indexOf("=");
      if (at > 0)
        values[line.slice(0, at).trim()] = line.slice(at + 1).trim();
    }
    return values;
  }

  FileView {
    id: systemLook

    path: "/etc/hyprtrixie/look.conf"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  FileView {
    id: userLook

    path: (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/hyprtrixie/look.conf"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  FileView {
    id: lookFile

    path: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/omarchy/current/theme/look.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  IpcHandler {
    target: "theme"

    function reload(): void {
      file.reload();
      lookFile.reload();
      systemLook.reload();
      userLook.reload();
    }
  }
}
