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
  readonly property string font: "JetBrainsMono Nerd Font"
  readonly property int fontSize: 13
  readonly property int barHeight: 30
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

  FileView {
    id: file

    path: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/omarchy/current/theme/colors.toml"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  IpcHandler {
    target: "theme"

    function reload(): void {
      file.reload();
    }
  }
}
