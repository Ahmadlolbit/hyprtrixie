import QtQuick
import Quickshell
import qs.common

Label {
  property var bar

  text: Qt.formatDateTime(clock.date, Config.clock === "12h" ? "ddd d MMM   h:mm AP" : "ddd d MMM   HH:mm")
  font.bold: true

  SystemClock {
    id: clock

    precision: SystemClock.Minutes
  }
}
