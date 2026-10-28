import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland

PopupWindow {
  id: root

  required property string name
  required property var bar
  required property Item owner
  default property alias content: body.data
  readonly property bool below: Config.position !== "bottom"

  function toggle() {
    const point = owner.mapToItem(null, 0, 0);
    anchor.rect.x = point.x;
    anchor.rect.width = owner.width;
    Popups.toggle(root);
  }

  anchor.window: bar
  anchor.rect.y: below ? 0 : -6
  anchor.rect.height: bar ? bar.height + 6 : 0
  anchor.edges: below ? Edges.Bottom : Edges.Top
  anchor.gravity: below ? Edges.Bottom : Edges.Top
  anchor.adjustment: PopupAdjustment.Slide
  visible: Popups.open === root
  color: "transparent"
  implicitWidth: frame.implicitWidth
  implicitHeight: frame.implicitHeight

  Component.onCompleted: Popups.add(root)
  Component.onDestruction: Popups.remove(root)

  HyprlandFocusGrab {
    windows: [root, root.bar]
    active: root.visible
    onCleared: Popups.close(root)
  }

  Rectangle {
    id: frame

    anchors.fill: parent
    implicitWidth: body.implicitWidth + 28
    implicitHeight: body.implicitHeight + 28
    color: Theme.base
    border.color: Theme.accent
    border.width: 2
    radius: Theme.radius

    Shortcut {
      sequence: "Escape"
      onActivated: Popups.close(root)
    }

    ColumnLayout {
      id: body

      anchors.fill: parent
      anchors.margins: 14
      spacing: 10
    }
  }
}
