import QtQuick
import Quickshell
import Quickshell.Services.Pipewire
import qs.common

Label {
  id: root

  property var bar
  readonly property PwNode sink: Pipewire.defaultAudioSink
  readonly property bool muted: sink && sink.audio ? sink.audio.muted : false
  readonly property real volume: sink && sink.audio ? sink.audio.volume : 0

  function icon() {
    if (muted || volume === 0)
      return "\u{f0581}";
    if (volume < 0.34)
      return "\u{f057f}";
    if (volume < 0.67)
      return "\u{f0580}";
    return "\u{f057e}";
  }

  text: icon() + " " + Math.round(volume * 100) + "%"
  color: muted ? Theme.subtext : Theme.text

  PwObjectTracker {
    objects: [root.sink]
  }

  MouseArea {
    anchors.fill: parent
    acceptedButtons: Qt.LeftButton | Qt.RightButton
    cursorShape: Qt.PointingHandCursor
    onClicked: mouse => {
      if (mouse.button === Qt.RightButton && root.sink)
        root.sink.audio.muted = !root.muted;
      else
        Quickshell.execDetached(["pavucontrol"]);
    }
    onWheel: wheel => {
      if (root.sink)
        root.sink.audio.volume = Math.max(0, Math.min(1, root.volume + (wheel.angleDelta.y > 0 ? 0.05 : -0.05)));
    }
  }
}
