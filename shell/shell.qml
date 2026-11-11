import QtQuick
import Quickshell
import qs.common

ShellRoot {
  Component.onCompleted: [Focus.phase, Session.awake, Screens.dark]

  Variants {
    model: Quickshell.screens

    Bar {}
  }

  Osd {}

  Picker {}

  KeySheet {}

  Menu {}

  Welcome {}

  Look {}

  Corners {}

  Loader {
    active: Theme.video !== ""
    source: "VideoWallpaper.qml"
  }
}
