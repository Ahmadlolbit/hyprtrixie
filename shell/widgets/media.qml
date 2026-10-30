import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.Mpris
import qs.common

Label {
  id: root

  property var bar
  readonly property var players: Mpris.players.values
  readonly property var player: players.find(item => item.isPlaying) || players.find(item => item.canTogglePlaying) || null
  readonly property bool shown: player !== null && player.trackTitle !== ""
  readonly property bool vertical: bar ? bar.vertical === true : false

  function icon() {
    return player && player.isPlaying ? "\u{f075a}" : "\u{f03e4}";
  }

  function line() {
    if (!player)
      return "";
    return player.trackArtist ? player.trackTitle + " · " + player.trackArtist : player.trackTitle;
  }

  width: Math.min(implicitWidth, 300)
  text: vertical ? icon() : icon() + " " + line()
  color: player && player.isPlaying ? Theme.text : Theme.subtext
  elide: Text.ElideRight

  MouseArea {
    anchors.fill: parent
    acceptedButtons: Qt.LeftButton | Qt.MiddleButton
    cursorShape: Qt.PointingHandCursor
    onClicked: mouse => {
      if (mouse.button === Qt.MiddleButton && root.player)
        root.player.togglePlaying();
      else
        popup.toggle();
    }
    onWheel: wheel => {
      if (!root.player)
        return;
      if (wheel.angleDelta.y < 0 && root.player.canGoNext)
        root.player.next();
      else if (wheel.angleDelta.y > 0 && root.player.canGoPrevious)
        root.player.previous();
    }
  }

  component Button: Label {
    id: button

    signal pressed

    font.pixelSize: 22
    color: enabled ? Theme.text : Theme.subtext

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: button.pressed()
    }
  }

  BarPopup {
    id: popup

    name: "media"
    bar: root.bar
    owner: root

    RowLayout {
      Layout.preferredWidth: 320
      spacing: 14

      Rectangle {
        Layout.preferredWidth: 88
        Layout.preferredHeight: 88
        color: Theme.surface
        radius: Theme.radius

        Image {
          id: cover

          anchors.fill: parent
          source: root.player ? root.player.trackArtUrl : ""
          fillMode: Image.PreserveAspectCrop
          sourceSize.width: 176
          sourceSize.height: 176
          visible: status === Image.Ready
        }

        Label {
          anchors.centerIn: parent
          visible: !cover.visible
          text: "\u{f075a}"
          color: Theme.subtext
          font.pixelSize: 32
        }
      }

      ColumnLayout {
        Layout.fillWidth: true
        spacing: 4

        Label {
          Layout.fillWidth: true
          text: root.player ? root.player.trackTitle : ""
          font.bold: true
          elide: Text.ElideRight
        }

        Label {
          Layout.fillWidth: true
          visible: text !== ""
          text: root.player ? root.player.trackArtist : ""
          elide: Text.ElideRight
        }

        Label {
          Layout.fillWidth: true
          visible: text !== ""
          text: root.player ? root.player.trackAlbum : ""
          color: Theme.subtext
          elide: Text.ElideRight
        }
      }
    }

    RowLayout {
      Layout.alignment: Qt.AlignHCenter
      spacing: 28

      Button {
        text: "\u{f04ae}"
        enabled: root.player ? root.player.canGoPrevious : false
        onPressed: root.player.previous()
      }

      Button {
        text: root.player && root.player.isPlaying ? "\u{f03e4}" : "\u{f040a}"
        enabled: root.player ? root.player.canTogglePlaying : false
        onPressed: root.player.togglePlaying()
      }

      Button {
        text: "\u{f04ad}"
        enabled: root.player ? root.player.canGoNext : false
        onPressed: root.player.next()
      }
    }

    Label {
      visible: text !== ""
      text: root.player ? root.player.identity : ""
      color: Theme.subtext
    }
  }
}
