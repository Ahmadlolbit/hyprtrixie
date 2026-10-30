import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property var history: []
  property bool quiet: false
  property int seen: 0
  property int cleared: 0
  readonly property var shownItems: history.filter(item => item.id > cleared)
  readonly property int newest: history.length ? Math.max(...history.map(item => item.id)) : 0
  readonly property int unread: history.filter(item => item.id > Math.max(seen, cleared)).length

  function parse(text) {
    try {
      return JSON.parse(text).data[0].map(item => ({
            id: item.id.data,
            app: item["app-name"].data,
            summary: item.summary.data,
            body: item.body.data.replace(/<[^>]*>/g, ""),
            critical: item.urgency.data === 2
          })).reverse();
    } catch (error) {
      return [];
    }
  }

  function refresh() {
    lister.running = true;
    modes.running = true;
  }

  text: (quiet ? "\u{f009b}" : "\u{f009a}") + (unread > 0 ? " " + unread : "")
  color: unread > 0 && !quiet ? Theme.accent : Theme.subtext

  Process {
    id: lister

    command: ["busctl", "--user", "-j", "call", "org.freedesktop.Notifications", "/fr/emersion/Mako", "fr.emersion.Mako", "ListHistory"]
    stdout: StdioCollector {
      onStreamFinished: root.history = root.parse(text)
    }
  }

  Process {
    id: modes

    command: ["makoctl", "mode"]
    stdout: StdioCollector {
      onStreamFinished: root.quiet = text.split("\n").includes("do-not-disturb")
    }
  }

  Process {
    id: switcher

    command: ["makoctl", "mode", "-t", "do-not-disturb"]
    onExited: modes.running = true
  }

  Timer {
    interval: 10000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: root.refresh()
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "notifications"
    bar: root.bar
    owner: root
    onVisibleChanged: {
      if (visible) {
        root.refresh();
        root.seen = root.newest;
      }
    }

    RowLayout {
      Layout.fillWidth: true
      Layout.preferredWidth: 340
      spacing: 10

      Label {
        Layout.fillWidth: true
        text: "Notifications"
        font.bold: true
      }

      Label {
        text: "Do not disturb"
        color: Theme.subtext
      }

      Toggle {
        checked: root.quiet
        onToggled: switcher.running = true
      }
    }

    Label {
      visible: root.shownItems.length === 0
      text: "Nothing yet"
      color: Theme.subtext
    }

    ListView {
      id: list

      visible: count > 0
      Layout.fillWidth: true
      Layout.preferredHeight: Math.min(contentHeight, 360)
      clip: true
      spacing: 8
      model: root.shownItems

      delegate: ColumnLayout {
        required property var modelData

        width: list.width
        spacing: 2

        Label {
          Layout.fillWidth: true
          text: modelData.app
          color: modelData.critical ? Theme.critical : Theme.subtext
          elide: Text.ElideRight
        }

        Label {
          Layout.fillWidth: true
          text: modelData.summary
          font.bold: true
          elide: Text.ElideRight
        }

        Label {
          Layout.fillWidth: true
          visible: text !== ""
          text: modelData.body
          wrapMode: Text.WordWrap
          maximumLineCount: 3
          elide: Text.ElideRight
        }
      }
    }

    RowLayout {
      spacing: 16

      Label {
        visible: root.shownItems.length > 0
        text: "Clear"
        color: Theme.accent

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: root.cleared = root.newest
        }
      }

      Label {
        visible: root.history.length > 0
        text: "Show the last one"
        color: Theme.accent

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: {
            Popups.close(popup);
            Quickshell.execDetached(["makoctl", "restore"]);
          }
        }
      }
    }
  }
}
