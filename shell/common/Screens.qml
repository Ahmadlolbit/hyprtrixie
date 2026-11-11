pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
  id: root

  property bool dark: false

  IpcHandler {
    target: "screens"

    function off(): void {
      root.dark = true;
    }

    function on(): void {
      root.dark = false;
    }
  }
}
