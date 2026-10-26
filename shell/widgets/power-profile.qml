import QtQuick
import Quickshell
import Quickshell.Services.UPower
import qs.common

Label {
  property var bar

  text: PowerProfiles.profile === PowerProfile.PowerSaver ? "\u{f032a}" : PowerProfiles.profile === PowerProfile.Performance ? "\u{f04c5}" : "\u{f05d1}"
  color: PowerProfiles.profile === PowerProfile.Performance ? Theme.warning : Theme.text

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: {
      const next = PowerProfiles.profile === PowerProfile.PowerSaver ? "balanced" : PowerProfiles.profile === PowerProfile.Balanced && PowerProfiles.hasPerformanceProfile ? "performance" : "power-saver";
      Quickshell.execDetached(["powerprofilesctl", "set", next]);
    }
  }
}
