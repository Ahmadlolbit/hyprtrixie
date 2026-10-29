import QtQuick
import Quickshell

ShellRoot {
  Variants {
    model: Quickshell.screens

    Bar {}
  }

  Osd {}

  Picker {}

  KeySheet {}
}
