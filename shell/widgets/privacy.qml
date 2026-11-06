import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.Pipewire
import qs.common

GridLayout {
  id: root

  property var bar
  readonly property var held: Cameras.held
  readonly property var links: Pipewire.linkGroups.values.filter(link => link.source && link.target)
  readonly property var listening: links.filter(link => link.source.type === PwNodeType.AudioSource && link.target.isStream)
  readonly property var filming: links.filter(link => link.source.type === PwNodeType.VideoSource && !shared(link.source))
  readonly property var watching: links.filter(link => shared(link.source))
  readonly property bool mic: listening.length > 0
  readonly property bool camera: filming.length > 0 || held.length > 0
  readonly property bool screen: Pipewire.nodes.values.some(node => shared(node))
  readonly property bool shown: mic || camera || screen || Session.recording

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

  Label {
    visible: Session.recording
    text: "\u{f044a}"
    color: Theme.critical

    MouseArea {
      anchors.fill: parent
      cursorShape: Qt.PointingHandCursor
      onClicked: Quickshell.execDetached(["hyprtrixie-record", "stop"])
    }
  }

  BarPopup {
    id: popup

    name: "privacy"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: Theme.size(280)
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
