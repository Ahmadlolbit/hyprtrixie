import QtQuick
import Quickshell.Hyprland
import qs.common

Label {
  property var bar
  readonly property bool shown: !(bar && bar.vertical)

  width: Math.min(implicitWidth, 420)
  text: Hyprland.activeToplevel ? Hyprland.activeToplevel.title : ""
  color: Theme.subtext
  elide: Text.ElideRight
}
