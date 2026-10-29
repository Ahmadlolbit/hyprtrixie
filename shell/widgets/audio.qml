import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.Pipewire
import qs.common

Label {
  id: root

  property var bar
  readonly property PwNode sink: Pipewire.defaultAudioSink
  readonly property PwNode source: Pipewire.defaultAudioSource
  readonly property bool muted: sink && sink.audio ? sink.audio.muted : false
  readonly property real volume: sink && sink.audio ? sink.audio.volume : 0
  readonly property bool micMuted: source && source.audio ? source.audio.muted : false
  readonly property real micVolume: source && source.audio ? source.audio.volume : 0

  function icon() {
    if (muted || volume === 0)
      return "\u{f0581}";
    if (volume < 0.34)
      return "\u{f057f}";
    if (volume < 0.67)
      return "\u{f0580}";
    return "\u{f057e}";
  }

  function devices(sinks) {
    return Pipewire.nodes.values.filter(node => node.audio && !node.isStream && node.isSink === sinks);
  }

  function title(node) {
    return node.description || node.nickname || node.name;
  }

  function streams() {
    return Pipewire.nodes.values.filter(node => node.audio && node.isStream && node.isSink);
  }

  function appName(node) {
    return node.properties["application.name"] || node.description || node.name;
  }

  text: icon() + " " + Math.round(volume * 100) + "%"
  color: muted ? Theme.subtext : Theme.text

  PwObjectTracker {
    objects: [root.sink, root.source]
  }

  PwObjectTracker {
    objects: root.streams()
  }

  MouseArea {
    anchors.fill: parent
    acceptedButtons: Qt.LeftButton | Qt.RightButton
    cursorShape: Qt.PointingHandCursor
    onClicked: mouse => {
      if (mouse.button === Qt.RightButton && root.sink)
        root.sink.audio.muted = !root.muted;
      else
        popup.toggle();
    }
    onWheel: wheel => {
      if (root.sink)
        root.sink.audio.volume = Math.max(0, Math.min(1, root.volume + (wheel.angleDelta.y > 0 ? 0.05 : -0.05)));
    }
  }

  BarPopup {
    id: popup

    name: "audio"
    bar: root.bar
    owner: root

    Label {
      Layout.preferredWidth: 300
      text: "Output"
      font.bold: true
    }

    RowLayout {
      Layout.fillWidth: true
      spacing: 10

      Label {
        text: root.icon()
        color: root.muted ? Theme.subtext : Theme.text

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: {
            if (root.sink)
              root.sink.audio.muted = !root.muted;
          }
        }
      }

      Slider {
        Layout.fillWidth: true
        value: root.volume
        onMoved: level => {
          if (root.sink)
            root.sink.audio.volume = level;
        }
      }

      Label {
        Layout.preferredWidth: 36
        horizontalAlignment: Text.AlignRight
        text: Math.round(root.volume * 100) + "%"
      }
    }

    Repeater {
      model: root.devices(true)

      Choice {
        required property PwNode modelData

        Layout.fillWidth: true
        icon: "\u{f04c3}"
        label: root.title(modelData)
        selected: modelData === root.sink
        onClicked: Pipewire.preferredDefaultAudioSink = modelData
      }
    }

    Label {
      visible: root.streams().length > 0
      text: "Apps"
      font.bold: true
    }

    Repeater {
      model: root.streams()

      RowLayout {
        id: app

        required property PwNode modelData
        readonly property bool muted: modelData.audio ? modelData.audio.muted : false
        readonly property real volume: modelData.audio ? modelData.audio.volume : 0

        Layout.fillWidth: true
        spacing: 10

        Label {
          text: app.muted ? "\u{f075b}" : "\u{f075a}"
          color: app.muted ? Theme.subtext : Theme.text

          MouseArea {
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            onClicked: {
              if (app.modelData.audio)
                app.modelData.audio.muted = !app.muted;
            }
          }
        }

        Label {
          Layout.preferredWidth: 90
          text: root.appName(app.modelData)
          elide: Text.ElideRight
        }

        Slider {
          Layout.fillWidth: true
          value: app.volume
          onMoved: level => {
            if (app.modelData.audio)
              app.modelData.audio.volume = level;
          }
        }

        Label {
          Layout.preferredWidth: 36
          horizontalAlignment: Text.AlignRight
          text: Math.round(app.volume * 100) + "%"
        }
      }
    }

    Label {
      text: "Input"
      font.bold: true
    }

    RowLayout {
      Layout.fillWidth: true
      spacing: 10

      Label {
        text: root.micMuted ? "\u{f036d}" : "\u{f036c}"
        color: root.micMuted ? Theme.critical : Theme.text

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: {
            if (root.source)
              root.source.audio.muted = !root.micMuted;
          }
        }
      }

      Slider {
        Layout.fillWidth: true
        value: root.micVolume
        onMoved: level => {
          if (root.source)
            root.source.audio.volume = level;
        }
      }

      Label {
        Layout.preferredWidth: 36
        horizontalAlignment: Text.AlignRight
        text: Math.round(root.micVolume * 100) + "%"
      }
    }

    Repeater {
      model: root.devices(false)

      Choice {
        required property PwNode modelData

        Layout.fillWidth: true
        icon: "\u{f036c}"
        label: root.title(modelData)
        selected: modelData === root.source
        onClicked: Pipewire.preferredDefaultAudioSource = modelData
      }
    }

    Label {
      text: "More settings"
      color: Theme.accent

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
          Popups.close(popup);
          Quickshell.execDetached(["pavucontrol"]);
        }
      }
    }
  }
}
