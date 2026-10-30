import QtQuick
import QtQuick.Layouts
import Quickshell.Services.Pipewire
import Quickshell.Wayland
import qs.common

RowLayout {
  id: root

  property var bar
  readonly property PwNode source: Pipewire.defaultAudioSource
  readonly property var stays: [
    {
      label: "Until I turn it off",
      minutes: 0
    },
    {
      label: "30 minutes",
      minutes: 30
    },
    {
      label: "1 hour",
      minutes: 60
    },
    {
      label: "2 hours",
      minutes: 120
    }
  ]

  spacing: 8

  PwObjectTracker {
    objects: [root.source]
  }

  IdleInhibitor {
    window: root.bar
    enabled: Session.awake
  }

  Label {
    horizontalAlignment: Text.AlignHCenter
    text: Session.awake ? "\u{f0176}" + (Session.minutesLeft > 0 ? (root.bar && root.bar.vertical ? "\n" : " ") + Session.minutesLeft + "m" : "") : "\u{f06ca}"
    color: Session.awake ? Theme.warning : Theme.subtext

    MouseArea {
      anchors.fill: parent
      acceptedButtons: Qt.LeftButton | Qt.RightButton
      cursorShape: Qt.PointingHandCursor
      onClicked: mouse => {
        if (mouse.button === Qt.RightButton)
          popup.toggle();
        else
          Session.awake = !Session.awake;
      }
    }
  }

  Label {
    visible: root.source && root.source.audio ? root.source.audio.muted : false
    text: "\u{f036d}"
    color: Theme.critical
  }

  BarPopup {
    id: popup

    name: "awake"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: 220
      text: "Stay awake"
      font.bold: true
    }

    Repeater {
      model: root.stays

      Choice {
        required property var modelData

        Layout.fillWidth: true
        label: modelData.label
        selected: Session.awake && Session.until === 0 && modelData.minutes === 0
        onClicked: {
          Session.stayAwake(modelData.minutes);
          Popups.close(popup);
        }
      }
    }

    Choice {
      Layout.fillWidth: true
      visible: Session.awake
      label: "Turn it off"
      onClicked: {
        Session.awake = false;
        Popups.close(popup);
      }
    }

    Label {
      visible: Session.minutesLeft > 0
      text: Session.minutesLeft + (Session.minutesLeft === 1 ? " minute" : " minutes") + " left"
      color: Theme.subtext
    }
  }
}
