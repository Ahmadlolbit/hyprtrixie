import QtQuick

Rectangle {
  id: root

  property bool checked: false

  signal toggled

  implicitWidth: 36
  implicitHeight: 20
  radius: Theme.radius ? height / 2 : 0
  color: checked ? Theme.accent : Theme.surface

  Rectangle {
    x: root.checked ? root.width - width - 3 : 3
    anchors.verticalCenter: parent.verticalCenter
    width: root.height - 6
    height: width
    radius: Theme.radius ? width / 2 : 0
    color: root.checked ? Theme.base : Theme.subtext

    Behavior on x {
      NumberAnimation {
        duration: 120
      }
    }
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: root.toggled()
  }
}
