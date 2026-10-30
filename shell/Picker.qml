import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Wayland
import qs.common

Scope {
  id: root

  readonly property int wide: 660
  readonly property int narrow: 96
  readonly property int gap: 12
  property string mode: ""
  property var items: []
  property string filter: ""
  property string selected: ""
  property int index: 0
  property bool busy: false
  readonly property bool open: mode !== ""
  readonly property var shown: items.filter(item => item.label.toLowerCase().includes(filter.toLowerCase()))
  readonly property var screen: Quickshell.screens.find(item => Hyprland.focusedMonitor && item.name === Hyprland.focusedMonitor.name) || Quickshell.screens[0]

  function show(kind) {
    mode = kind;
    filter = "";
    busy = false;
    items = [];
    content.forceActiveFocus();
    if (kind === "themes") {
      themeName.reload();
      selected = themeName.text().trim();
      list.command = ["hyprtrixie-theme-set", "--previews"];
      list.running = true;
    } else {
      background.running = true;
    }
  }

  function close() {
    mode = "";
  }

  function move(step) {
    if (shown.length)
      index = Math.max(0, Math.min(shown.length - 1, index + step));
  }

  function apply() {
    const item = shown[index];
    if (!item || busy)
      return;
    busy = true;
    applying.command = mode === "themes" ? ["hyprtrixie-theme-set", item.value] : ["hyprtrixie-background", item.value];
    applying.running = true;
  }

  function parse(line) {
    if (mode === "themes") {
      const fields = line.split("\t");
      return {
        value: fields[0],
        label: fields[1] || fields[0],
        image: fields[2] || ""
      };
    }
    return {
      value: line,
      label: line.split("/").pop().replace(/\.[^.]+$/, ""),
      image: line
    };
  }

  onFilterChanged: index = 0

  FileView {
    id: themeName

    path: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/omarchy/current/theme.name"
    blockLoading: true
    printErrors: false
  }

  Process {
    id: background

    command: ["hyprtrixie-background", "--current"]
    stdout: StdioCollector {
      onStreamFinished: {
        root.selected = text.trim();
        list.command = ["hyprtrixie-background", "--list"];
        list.running = true;
      }
    }
  }

  Process {
    id: list

    stdout: StdioCollector {
      onStreamFinished: {
        root.items = text.split("\n").filter(line => line !== "").map(line => root.parse(line));
        root.index = Math.max(0, root.items.findIndex(item => item.value === root.selected));
      }
    }
  }

  Process {
    id: applying

    onExited: done.restart()
  }

  Timer {
    id: done

    interval: 300
    onTriggered: root.close()
  }

  IpcHandler {
    target: "picker"

    function themes(): void {
      root.show("themes");
    }

    function backgrounds(): void {
      root.show("backgrounds");
    }

    function close(): void {
      root.close();
    }
  }

  PanelWindow {
    screen: root.screen
    visible: root.open || content.opacity > 0
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "hyprtrixie-picker"
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
        else if (event.key === Qt.Key_Left)
          root.move(-1);
        else if (event.key === Qt.Key_Right)
          root.move(1);
        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
          root.apply();
        else if (event.key === Qt.Key_Backspace)
          root.filter = root.filter.slice(0, -1);
        else if (/^[\w .-]$/.test(event.text))
          root.filter += event.text;
        else
          return;
        event.accepted = true;
      }

      Behavior on opacity {
        NumberAnimation {
          duration: 250
          easing.type: Easing.OutCubic
        }
      }

      MouseArea {
        anchors.fill: parent
        onClicked: root.close()
        onWheel: wheel => root.move(wheel.angleDelta.y > 0 ? -1 : 1)
      }

      Item {
        id: carousel

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: -30
        height: 420

        Repeater {
          model: root.shown

          Item {
            id: slice

            required property var modelData
            required property int index
            readonly property bool current: index === root.index
            readonly property int offset: index - root.index

            x: carousel.width / 2 + (current ? -root.wide / 2 : offset < 0 ? -root.wide / 2 + offset * (root.narrow + root.gap) : root.wide / 2 + root.gap + (offset - 1) * (root.narrow + root.gap))
            width: current ? root.wide : root.narrow
            height: carousel.height

            Behavior on x {
              NumberAnimation {
                duration: 180
                easing.type: Easing.OutCubic
              }
            }

            Behavior on width {
              NumberAnimation {
                duration: 180
                easing.type: Easing.OutCubic
              }
            }

            Rectangle {
              anchors.fill: parent
              radius: Theme.radius
              color: Theme.surface
              border.width: slice.current ? 2 : 0
              border.color: Theme.accent
              clip: true

              Image {
                anchors.fill: parent
                anchors.margins: slice.current ? 2 : 0
                source: slice.modelData.image ? "file://" + slice.modelData.image : ""
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                sourceSize.width: root.wide
                sourceSize.height: carousel.height
              }

              Rectangle {
                anchors.fill: parent
                color: Theme.base
                opacity: slice.current ? 0 : 0.5

                Behavior on opacity {
                  NumberAnimation {
                    duration: 180
                  }
                }
              }
            }

            MouseArea {
              anchors.fill: parent
              cursorShape: Qt.PointingHandCursor
              onClicked: {
                if (slice.current)
                  root.apply();
                else
                  root.index = slice.index;
              }
            }
          }
        }
      }

      Label {
        id: title

        anchors.top: carousel.bottom
        anchors.topMargin: 24
        anchors.horizontalCenter: parent.horizontalCenter
        text: root.shown.length ? (root.busy ? "Applying " : "") + root.shown[root.index].label : root.items.length ? "Nothing matches " + root.filter : ""
        font.pixelSize: 22
        font.bold: true
      }

      Label {
        anchors.top: title.bottom
        anchors.topMargin: 10
        anchors.horizontalCenter: parent.horizontalCenter
        text: root.filter ? "Filter: " + root.filter : "Type to filter · arrows or wheel to move · Enter to apply · Esc to close"
        color: Theme.subtext
      }
    }
  }
}
