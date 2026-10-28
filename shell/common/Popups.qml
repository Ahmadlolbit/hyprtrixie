pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io

Singleton {
  id: root

  property var open: null
  property var popups: []

  function add(popup) {
    popups = popups.concat([popup]);
  }

  function remove(popup) {
    popups = popups.filter(item => item !== popup);
    close(popup);
  }

  function toggle(popup) {
    open = open === popup ? null : popup;
  }

  function close(popup) {
    if (open === popup)
      open = null;
  }

  IpcHandler {
    target: "popup"

    function toggle(name: string): void {
      const matches = root.popups.filter(popup => popup.name === name && popup.bar);
      const monitor = Hyprland.focusedMonitor;
      const popup = matches.find(item => Hyprland.monitorFor(item.bar.screen) === monitor) || matches[0];
      if (popup)
        popup.toggle();
    }

    function close(): void {
      root.open = null;
    }
  }
}
