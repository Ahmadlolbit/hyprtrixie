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
  readonly property var window: owner ? owner.QsWindow.window : null
  readonly property bool below: Config.position !== "bottom"

  function toggle() {
    Popups.toggle(root);
  }

  anchor {
    id: placement

    window: root.window
    edges: Edges.Top | Edges.Left
    gravity: Edges.Bottom | Edges.Right
    adjustment: PopupAdjustment.Slide
    rect.width: 1
    rect.height: 1
    onAnchoring: {
      if (!root.owner || !root.window)
        return;
      const point = root.window.contentItem.mapFromItem(root.owner, root.owner.width / 2 - root.implicitWidth / 2, 0);
      placement.rect.x = Math.round(Math.max(6, Math.min(point.x, root.window.width - root.implicitWidth - 6)));
      placement.rect.y = root.below ? root.window.height + 6 : -root.implicitHeight - 6;
    }
  }
  visible: Popups.open === root
  color: "transparent"
  implicitWidth: frame.implicitWidth
  implicitHeight: frame.implicitHeight

  Component.onCompleted: Popups.add(root)
  Component.onDestruction: Popups.remove(root)

  HyprlandFocusGrab {
    windows: root.window ? [root, root.window] : [root]
    active: root.visible
    onCleared: Popups.close(root)
  }

  Rectangle {
    id: frame

    anchors.fill: parent
    implicitWidth: body.implicitWidth + 28
    implicitHeight: body.implicitHeight + 28
    color: Theme.panel
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
