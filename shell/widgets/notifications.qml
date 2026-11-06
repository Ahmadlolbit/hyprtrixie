import QtQuick
import QtQuick.Layouts
import Quickshell
import qs.common

Label {
  id: root

  property var bar
  readonly property var history: Notifications.history
  readonly property bool quiet: Notifications.quiet
  readonly property var shownItems: Notifications.shownItems
  readonly property int unread: Notifications.unread

  text: (quiet ? "\u{f009b}" : "\u{f009a}") + (unread > 0 ? " " + unread : "")
  color: unread > 0 && !quiet ? Theme.accent : Theme.subtext

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
      if (visible)
        Notifications.open();
    }

    RowLayout {
      Layout.fillWidth: true
      Layout.preferredWidth: Theme.size(340)
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
        onToggled: Notifications.toggleQuiet()
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
          onClicked: Notifications.clear()
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
