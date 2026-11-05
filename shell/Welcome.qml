import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Wayland
import qs.common

Scope {
  id: root

  readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/hyprtrixie"
  readonly property var keys: [["Super + Space", "Apps"], ["Super + Alt + Space", "Menu"], ["Super + K", "All the keys"], ["Super + Return", "Terminal"], ["Super + Ctrl + T", "Themes"]]
  property bool open: false
  property string tip: ""
  readonly property var screen: Quickshell.screens.find(item => Hyprland.focusedMonitor && item.name === Hyprland.focusedMonitor.name) || Quickshell.screens[0]

  function show() {
    tip = "";
    tipper.running = true;
    open = true;
    content.forceActiveFocus();
  }

  function close() {
    open = false;
    Quickshell.execDetached(["sh", "-c", "mkdir -p \"$1\" && touch \"$1/welcome-seen\"", "sh", stateDir]);
  }

  function run(command) {
    close();
    Quickshell.execDetached(["sh", "-c", command]);
  }

  component Action: Rectangle {
    id: action

    required property string label
    required property string command

    implicitWidth: text.implicitWidth + 32
    implicitHeight: Theme.size(36)
    radius: Theme.radius
    color: hover.containsMouse ? Theme.overlay : Theme.surface
    border.color: hover.containsMouse ? Theme.accent : "transparent"
    border.width: 1

    Label {
      id: text

      anchors.centerIn: parent
      text: action.label
    }

    MouseArea {
      id: hover

      anchors.fill: parent
      hoverEnabled: true
      cursorShape: Qt.PointingHandCursor
      onClicked: action.command ? root.run(action.command) : root.close()
    }
  }

  FileView {
    id: seen

    path: root.stateDir + "/welcome-seen"
    printErrors: false
    onLoadFailed: later.start()
  }

  Process {
    id: tipper

    command: ["hyprtrixie-tip", "welcome"]
    stdout: StdioCollector {
      onStreamFinished: root.tip = text.trim()
    }
  }

  Timer {
    id: later

    interval: 2500
    onTriggered: root.show()
  }

  IpcHandler {
    target: "welcome"

    function open(): void {
      root.show();
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
    WlrLayershell.namespace: "hyprtrixie-welcome"
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
        if (event.key === Qt.Key_Escape || event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
          root.close();
          event.accepted = true;
        }
      }

      Behavior on opacity {
        NumberAnimation {
          duration: 250
          easing.type: Easing.OutCubic
        }
      }

      Rectangle {
        anchors.centerIn: parent
        width: column.implicitWidth + 64
        height: column.implicitHeight + 56
        color: Theme.panel
        border.color: Theme.accent
        border.width: 2
        radius: Theme.radius

        ColumnLayout {
          id: column

          anchors.centerIn: parent
          spacing: 18

          Item {
            id: burst

            readonly property int cell: 4
            readonly property var rows: [0x00000000, 0x00080000, 0x00080200, 0x00040200, 0x01040400, 0x00820400, 0x00820400, 0x00430808, 0x00430830, 0x002f1040, 0x00292180, 0x0011c300, 0x00083600, 0x00081900, 0x00100906, 0x00d00ff8, 0x1ff00b80, 0x60500800, 0x00480000, 0x004c0800, 0x00531800, 0x00616600, 0x00c31300, 0x01831180, 0x02031080, 0x04020840, 0x00020820, 0x00040410, 0x00040008, 0x00080000, 0x00080000, 0x00000000]

            Layout.alignment: Qt.AlignHCenter
            implicitWidth: 32 * cell
            implicitHeight: 32 * cell

            Repeater {
              model: {
                const lit = [];
                for (let y = 0; y < 32; y++)
                  for (let x = 0; x < 32; x++)
                    if ((burst.rows[y] >>> (31 - x)) & 1)
                      lit.push(Qt.point(x, y));
                return lit;
              }

              Rectangle {
                required property point modelData

                x: modelData.x * burst.cell
                y: modelData.y * burst.cell
                width: burst.cell
                height: burst.cell
                color: Theme.accent
              }
            }
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: "Welcome. These keys get you everywhere:"
            color: Theme.subtext
          }

          GridLayout {
            Layout.alignment: Qt.AlignHCenter
            columns: 2
            columnSpacing: 24
            rowSpacing: 8

            Repeater {
              model: root.keys.length * 2

              Label {
                required property int index

                Layout.alignment: index % 2 ? Qt.AlignLeft : Qt.AlignRight
                text: root.keys[Math.floor(index / 2)][index % 2]
                color: index % 2 ? Theme.text : Theme.accent
                font.bold: index % 2 === 0
              }
            }
          }

          RowLayout {
            Layout.alignment: Qt.AlignHCenter
            Layout.topMargin: 6
            spacing: 10

            Action {
              label: "Pick a theme"
              command: "qs -p /usr/share/hyprtrixie/shell ipc call picker themes"
            }

            Action {
              label: "Pick a background"
              command: "qs -p /usr/share/hyprtrixie/shell ipc call picker backgrounds"
            }

            Action {
              label: "Read the guide"
              command: "hyprtrixie-terminal --wait hyprtrixie guide"
            }

            Action {
              label: "Project page"
              command: "xdg-open https://github.com/Ahmadlolbit/hyprtrixie"
            }

            Action {
              label: "Close"
              command: ""
            }
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            Layout.maximumWidth: Theme.size(640)
            visible: root.tip !== ""
            text: "Tip: " + root.tip
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: "Menu › Learn › Welcome brings this back."
            color: Theme.subtext
          }
        }
      }
    }
  }
}
