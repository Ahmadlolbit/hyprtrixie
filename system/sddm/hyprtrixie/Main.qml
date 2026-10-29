import QtQuick
import QtQuick.Controls
import QtQuick.Effects

Rectangle {
  id: root

  readonly property color background: config.background || "#1e1e2e"
  readonly property color foreground: config.foreground || "#cdd6f4"
  readonly property color accent: config.accent || "#89b4fa"
  readonly property color fail: config.fail || "#f38ba8"
  readonly property string fontFamily: config.font || "monospace"
  readonly property string timeFormat: config.clock === "12h" ? "h:mm AP" : "HH:mm"
  property int userIndex: userModel.lastIndex >= 0 ? userModel.lastIndex : 0
  readonly property var currentUser: users.count > 0 ? users.itemAt(Math.min(userIndex, users.count - 1)) : null
  readonly property string userName: currentUser ? currentUser.login : userModel.lastUser
  readonly property int sessionIndex: sessionModel.lastIndex >= 0 ? sessionModel.lastIndex : 0
  property bool failed: false
  property date now: new Date()

  width: 1920
  height: 1080
  color: root.background

  function login() {
    failed = false;
    sddm.login(root.userName, password.text, root.sessionIndex);
  }

  Connections {
    target: sddm

    function onLoginFailed() {
      root.failed = true;
      password.text = "";
      password.forceActiveFocus();
    }
  }

  Timer {
    interval: 1000
    running: true
    repeat: true
    onTriggered: root.now = new Date()
  }

  Repeater {
    id: users

    model: userModel

    delegate: Item {
      required property string name
      required property string realName
      readonly property string login: name
      readonly property string shown: realName || name
    }
  }

  Image {
    id: wallpaper

    anchors.fill: parent
    source: config.wallpaper ? Qt.resolvedUrl(config.wallpaper) : ""
    fillMode: Image.PreserveAspectCrop
    asynchronous: true
  }

  MultiEffect {
    anchors.fill: parent
    source: wallpaper
    visible: wallpaper.status === Image.Ready
    blurEnabled: true
    blur: 1.0
    blurMax: 48
  }

  Rectangle {
    anchors.fill: parent
    visible: wallpaper.status === Image.Ready
    color: "black"
    opacity: 0.4
  }

  Column {
    anchors.centerIn: parent
    anchors.verticalCenterOffset: -40
    spacing: 14

    Text {
      anchors.horizontalCenter: parent.horizontalCenter
      text: Qt.formatTime(root.now, root.timeFormat)
      color: root.foreground
      font.family: root.fontFamily
      font.pixelSize: 128
    }

    Text {
      anchors.horizontalCenter: parent.horizontalCenter
      text: Qt.formatDate(root.now, "dddd, d MMMM")
      color: root.foreground
      font.family: root.fontFamily
      font.pixelSize: 26
    }

    Item {
      width: 1
      height: 56
    }

    Text {
      anchors.horizontalCenter: parent.horizontalCenter
      text: root.currentUser ? root.currentUser.shown : root.userName
      color: root.foreground
      opacity: 0.8
      font.family: root.fontFamily
      font.pixelSize: 18

      MouseArea {
        anchors.fill: parent
        enabled: users.count > 1
        cursorShape: Qt.PointingHandCursor
        onClicked: root.userIndex = (root.userIndex + 1) % users.count
      }
    }

    Rectangle {
      anchors.horizontalCenter: parent.horizontalCenter
      width: 320
      height: 56
      color: Qt.alpha(root.background, 0.8)
      border.color: root.failed ? root.fail : root.accent
      border.width: 2

      TextField {
        id: password

        anchors.fill: parent
        anchors.margins: 2
        focus: true
        echoMode: TextInput.Password
        passwordCharacter: "•"
        horizontalAlignment: TextInput.AlignHCenter
        cursorVisible: activeFocus && text !== ""
        color: root.foreground
        font.family: root.fontFamily
        font.pixelSize: 20
        background: Item {}
        onTextEdited: root.failed = false
        onAccepted: root.login()
        Keys.onEscapePressed: text = ""
      }

      Text {
        anchors.centerIn: parent
        visible: password.text === ""
        text: "Password"
        color: root.foreground
        opacity: 0.5
        font.family: root.fontFamily
        font.pixelSize: 20
      }
    }

    Text {
      anchors.horizontalCenter: parent.horizontalCenter
      text: root.failed ? "Wrong password" : keyboard.capsLock ? "Caps Lock is on" : " "
      color: root.failed ? root.fail : root.foreground
      font.family: root.fontFamily
      font.pixelSize: 16
    }
  }

  Row {
    anchors.right: parent.right
    anchors.bottom: parent.bottom
    anchors.margins: 32
    spacing: 28

    Repeater {
      model: [["\u{f04b2}", sddm.canSuspend, "suspend"], ["\u{f0709}", sddm.canReboot, "reboot"], ["\u{f0425}", sddm.canPowerOff, "powerOff"]]

      Text {
        required property var modelData

        visible: modelData[1]
        text: modelData[0]
        color: hover.containsMouse ? root.accent : root.foreground
        font.family: root.fontFamily
        font.pixelSize: 26

        MouseArea {
          id: hover

          anchors.fill: parent
          hoverEnabled: true
          cursorShape: Qt.PointingHandCursor
          onClicked: sddm[parent.modelData[2]]()
        }
      }
    }
  }
}
