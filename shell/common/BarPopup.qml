import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Wayland

PanelWindow {
  id: root

  required property string name
  required property var bar
  required property Item owner
  default property alias content: body.data
  readonly property var window: owner ? owner.QsWindow.window : null
  readonly property bool below: Config.position !== "bottom"

  function toggle() {
    Popups.toggle(root);
  }

  function place() {
    if (!owner || !window)
      return;
    const point = window.contentItem.mapFromItem(owner, owner.width / 2, 0);
    frame.x = Math.round(Math.max(6, Math.min(point.x - frame.width / 2, width - frame.width - 6)));
    frame.y = below ? window.height + 6 : height - window.height - frame.height - 6;
  }

  screen: bar ? bar.screen : null
  visible: Popups.open === root
  color: "transparent"
  exclusionMode: ExclusionMode.Ignore
  WlrLayershell.layer: WlrLayer.Overlay
  WlrLayershell.namespace: "hyprtrixie-popup"
  WlrLayershell.keyboardFocus: visible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
  onVisibleChanged: {
    if (!visible)
      return;
    place();
    frame.forceActiveFocus();
  }
  onWidthChanged: place()
  onHeightChanged: place()

  anchors {
    top: true
    bottom: true
    left: true
    right: true
  }

  Component.onCompleted: Popups.add(root)
  Component.onDestruction: Popups.remove(root)

  MouseArea {
    anchors.fill: parent
    onClicked: Popups.close(root)
  }

  Rectangle {
    id: frame

    width: body.implicitWidth + 28
    height: body.implicitHeight + 28
    color: Theme.panel
    border.color: Theme.accent
    border.width: 2
    radius: Theme.radius
    focus: true
    Keys.onEscapePressed: Popups.close(root)
    onWidthChanged: root.place()
    onHeightChanged: root.place()

    MouseArea {
      anchors.fill: parent
    }

    ColumnLayout {
      id: body

      anchors.fill: parent
      anchors.margins: 14
      spacing: 10
    }
  }
}
