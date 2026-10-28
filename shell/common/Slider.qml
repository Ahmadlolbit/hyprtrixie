import QtQuick

Item {
  id: root

  property real value: 0
  readonly property real shown: Math.max(0, Math.min(1, value))

  signal moved(real level)

  implicitWidth: 200
  implicitHeight: 20

  Rectangle {
    id: track

    anchors.left: parent.left
    anchors.right: parent.right
    anchors.verticalCenter: parent.verticalCenter
    height: 6
    radius: Theme.radius ? height / 2 : 0
    color: Theme.surface

    Rectangle {
      width: track.width * root.shown
      height: parent.height
      radius: parent.radius
      color: Theme.accent
    }
  }

  Rectangle {
    x: (root.width - width) * root.shown
    anchors.verticalCenter: parent.verticalCenter
    width: 14
    height: 14
    radius: Theme.radius ? width / 2 : 0
    color: Theme.text
  }

  MouseArea {
    function place(x) {
      root.moved(Math.max(0, Math.min(1, x / width)));
    }

    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onPressed: mouse => place(mouse.x)
    onPositionChanged: mouse => place(mouse.x)
    onWheel: wheel => root.moved(Math.max(0, Math.min(1, root.value + (wheel.angleDelta.y > 0 ? 0.05 : -0.05))))
  }
}
