import QtQuick

Rectangle {
  id: root

  property alias text: input.text
  property string placeholder: ""
  property bool secret: false

  signal accepted

  function focusInput() {
    input.forceActiveFocus();
  }

  implicitWidth: 240
  implicitHeight: 30
  radius: Theme.radius
  color: Theme.surface
  border.color: input.activeFocus ? Theme.accent : Theme.overlay
  border.width: 1

  TextInput {
    id: input

    anchors.fill: parent
    anchors.leftMargin: 8
    anchors.rightMargin: 8
    verticalAlignment: TextInput.AlignVCenter
    clip: true
    color: Theme.text
    selectionColor: Theme.accent
    selectedTextColor: Theme.base
    font.family: Theme.font
    font.pixelSize: Theme.fontSize
    echoMode: root.secret ? TextInput.Password : TextInput.Normal
    onAccepted: root.accepted()
  }

  Label {
    anchors.fill: input
    visible: input.text === ""
    text: root.placeholder
    color: Theme.subtext
  }
}
