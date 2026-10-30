import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import Quickshell.Services.Pipewire
import qs.common

GridLayout {
  id: root

  property var bar
  property var held: []
  readonly property var links: Pipewire.linkGroups.values.filter(link => link.source && link.target)
  readonly property var listening: links.filter(link => link.source.type === PwNodeType.AudioSource && link.target.isStream)
  readonly property var filming: links.filter(link => link.source.type === PwNodeType.VideoSource && !shared(link.source))
  readonly property var watching: links.filter(link => shared(link.source))
  readonly property bool mic: listening.length > 0
  readonly property bool camera: filming.length > 0 || held.length > 0
  readonly property bool screen: Pipewire.nodes.values.some(node => shared(node))
  readonly property bool shown: mic || camera || screen

  function shared(node) {
    return node.type === PwNodeType.VideoSource && (node.name.startsWith("xdg-desktop-portal") || node.name.startsWith("xdph-streaming"));
  }

  function apps(list) {
    const names = list.map(link => (link.target.properties || {})["application.name"] || link.target.description || link.target.name);
    return names.filter((name, index) => name && names.indexOf(name) === index);
  }

  flow: bar && bar.vertical ? GridLayout.TopToBottom : GridLayout.LeftToRight
  rowSpacing: 8
  columnSpacing: 8

  PwObjectTracker {
    objects: root.listening.concat(root.filming, root.watching).map(link => link.target)
  }

  Process {
    id: devices

    command: ["sh", "-c", "ls /dev/video* >/dev/null 2>&1 || exit 0; find /proc/[0-9]*/fd -lname '/dev/video*' -printf '%h\\n' 2>/dev/null | sort -u | while read -r fd; do cat \"${fd%/fd}/comm\"; done 2>/dev/null | sort -u"]
    stdout: StdioCollector {
      onStreamFinished: root.held = text.split("\n").filter(name => name !== "" && name !== "pipewire" && name !== "wireplumber")
    }
  }

  Timer {
    interval: 3000
    running: true
    repeat: true
    triggeredOnStart: true
    onTriggered: devices.running = true
  }

  Label {
    visible: root.mic
    text: "\u{f036c}"
    color: Theme.warning

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: popup.toggle()
    }
  }

  Label {
    visible: root.camera
    text: "\u{f05a0}"
    color: Theme.warning

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: popup.toggle()
    }
  }

  Label {
    visible: root.screen
    text: "\u{f1483}"
    color: Theme.warning

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: popup.toggle()
    }
  }

  BarPopup {
    id: popup

    name: "privacy"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: 280
      text: "In use"
      font.bold: true
    }

    Label {
      Layout.fillWidth: true
      visible: root.mic
      text: "\u{f036c}  Microphone: " + (root.apps(root.listening).join(", ") || "an app")
      wrapMode: Text.WordWrap
    }

    Label {
      Layout.fillWidth: true
      visible: root.camera
      text: "\u{f05a0}  Camera: " + (root.apps(root.filming).concat(root.held).join(", ") || "an app")
      wrapMode: Text.WordWrap
    }

    Label {
      Layout.fillWidth: true
      visible: root.screen
      text: "\u{f1483}  Screen: " + (root.apps(root.watching).join(", ") || "shared")
      wrapMode: Text.WordWrap
    }

    Label {
      visible: !root.shown
      text: "Nothing is recording"
      color: Theme.subtext
    }
  }
}
