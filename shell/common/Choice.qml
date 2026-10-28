import QtQuick
import QtQuick.Layouts

Rectangle {
  id: root

  property string icon: ""
  property string label: ""
  property string detail: ""
  property string action: ""
  property bool selected: false

  signal clicked
  signal actionClicked

  implicitWidth: row.implicitWidth + 16
  implicitHeight: 30
  radius: Theme.radius
  color: selected ? Theme.overlay : area.containsMouse ? Theme.surface : "transparent"

  MouseArea {
    id: area

    anchors.fill: parent
    hoverEnabled: true
    cursorShape: Qt.PointingHandCursor
    onClicked: root.clicked()
  }

  RowLayout {
    id: row

    anchors.fill: parent
    anchors.leftMargin: 8
    anchors.rightMargin: 8
    spacing: 10

    Label {
      visible: root.icon !== ""
      text: root.icon
      color: root.selected ? Theme.accent : Theme.text
    }

    Label {
      Layout.fillWidth: true
      text: root.label
      elide: Text.ElideRight
    }

    Label {
      visible: root.detail !== "" && !(root.action !== "" && area.containsMouse)
      text: root.detail
      color: Theme.subtext
    }

    Label {
      visible: root.action !== "" && area.containsMouse
      text: root.action
      color: Theme.accent

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.actionClicked()
      }
    }
  }
}
