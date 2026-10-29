import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Wayland
import qs.common

Scope {
  id: root

  property bool open: false
  property bool loaded: false
  property var binds: []
  property string filter: ""
  readonly property var shown: binds.filter(item => (item.keys + " " + item.text).toLowerCase().includes(filter.toLowerCase()))
  readonly property var screen: Quickshell.screens.find(item => Hyprland.focusedMonitor && item.name === Hyprland.focusedMonitor.name) || Quickshell.screens[0]
  readonly property var names: ({
      "return": "Enter",
      "escape": "Esc",
      "space": "Space",
      "tab": "Tab",
      "print": "Print",
      "mouse_down": "Wheel down",
      "mouse_up": "Wheel up",
      "xf86audioraisevolume": "Volume up",
      "xf86audiolowervolume": "Volume down",
      "xf86audiomute": "Mute",
      "xf86audiomicmute": "Mic mute",
      "xf86monbrightnessup": "Brightness up",
      "xf86monbrightnessdown": "Brightness down",
      "xf86audioplay": "Play",
      "xf86audiopause": "Pause",
      "xf86audionext": "Next track",
      "xf86audioprev": "Previous track",
      "left": "Left",
      "right": "Right",
      "up": "Up",
      "down": "Down"
    })

  function show() {
    filter = "";
    loaded = false;
    open = true;
    reader.running = true;
    content.forceActiveFocus();
  }

  function close() {
    open = false;
  }

  function keyName(bind) {
    const digits = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"];
    let key = bind.key || "";
    if (!key && bind.keycode >= 10 && bind.keycode <= 19)
      return digits[bind.keycode - 10];
    if (!key && bind.keycode)
      return "Key " + bind.keycode;
    if (key.startsWith("code:")) {
      const code = parseInt(key.slice(5));
      return code >= 10 && code <= 19 ? digits[code - 10] : "Key " + code;
    }
    return names[key.toLowerCase()] || (key.length === 1 ? key.toUpperCase() : key);
  }

  function keysOf(bind) {
    const parts = [];
    if (bind.modmask & 64)
      parts.push("Super");
    if (bind.modmask & 4)
      parts.push("Ctrl");
    if (bind.modmask & 8)
      parts.push("Alt");
    if (bind.modmask & 1)
      parts.push("Shift");
    parts.push(keyName(bind));
    return parts.join(" + ");
  }

  function parse(text) {
    let list = [];
    try {
      list = JSON.parse(text);
    } catch (error) {
      return [];
    }
    return list.filter(bind => bind.has_description && bind.description && !bind.mouse && !bind.submap).map(bind => ({
          keys: keysOf(bind),
          text: bind.description
        }));
  }

  Process {
    id: reader

    command: ["hyprctl", "binds", "-j"]
    stdout: StdioCollector {
      onStreamFinished: {
        root.binds = root.parse(text);
        root.loaded = true;
      }
    }
  }

  IpcHandler {
    target: "keys"

    function open(): void {
      root.show();
    }

    function close(): void {
      root.close();
    }

    function toggle(): void {
      if (root.open)
        root.close();
      else
        root.show();
    }
  }

  PanelWindow {
    screen: root.screen
    visible: root.open || content.opacity > 0
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "hyprtrixie-keys"
    WlrLayershell.keyboardFocus: root.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    anchors {
      top: true
      bottom: true
      left: true
      right: true
    }

    Rectangle {
      id: content

      anchors.fill: parent
      color: Qt.alpha(Theme.base, 0.85)
      opacity: root.open ? 1 : 0
      focus: true
      Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape)
          root.close();
        else if (event.key === Qt.Key_Backspace)
          root.filter = root.filter.slice(0, -1);
        else if (/^[\w .+-]$/.test(event.text))
          root.filter += event.text;
        else
          return;
        event.accepted = true;
      }

      Behavior on opacity {
        NumberAnimation {
          duration: 200
          easing.type: Easing.OutCubic
        }
      }

      MouseArea {
        anchors.fill: parent
        onClicked: root.close()
      }

      Rectangle {
        id: panel

        anchors.centerIn: parent
        width: Math.min(parent.width - 80, 1180)
        height: Math.min(parent.height - 120, header.implicitHeight + list.contentHeight + 72)
        color: Theme.base
        border.color: Theme.accent
        border.width: 2
        radius: Theme.radius

        MouseArea {
          anchors.fill: parent
        }

        ColumnLayout {
          anchors.fill: parent
          anchors.margins: 24
          spacing: 18

          RowLayout {
            id: header

            Layout.fillWidth: true

            Label {
              text: "Keys"
              font.pixelSize: 20
              font.bold: true
            }

            Item {
              Layout.fillWidth: true
            }

            Label {
              text: root.filter ? "Filter: " + root.filter : "Type to filter · Esc to close"
              color: Theme.subtext
            }
          }

          Flickable {
            id: list

            Layout.fillWidth: true
            Layout.fillHeight: true
            contentHeight: Math.max(grid.implicitHeight, empty.implicitHeight)
            clip: true
            boundsBehavior: Flickable.StopAtBounds

            GridLayout {
              id: grid

              width: list.width
              columns: width > 900 ? 2 : 1
              columnSpacing: 32
              rowSpacing: 7

              Repeater {
                model: root.shown

                RowLayout {
                  required property var modelData

                  Layout.fillWidth: true
                  Layout.preferredWidth: grid.width / grid.columns
                  spacing: 14

                  Label {
                    Layout.preferredWidth: 230
                    text: modelData.keys
                    color: Theme.accent
                    font.bold: true
                    elide: Text.ElideRight
                  }

                  Label {
                    Layout.fillWidth: true
                    text: modelData.text
                    elide: Text.ElideRight
                  }
                }
              }
            }

            Label {
              id: empty

              visible: root.shown.length === 0
              text: !root.loaded ? "Reading the keys…" : root.filter ? "No keys match " + root.filter : "No keys with a description"
              color: Theme.subtext
            }
          }
        }
      }
    }
  }
}
