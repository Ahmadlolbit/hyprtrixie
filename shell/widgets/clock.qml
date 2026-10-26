import QtQuick
import Quickshell
import qs.common

Label {
  property var bar

  text: Qt.formatDateTime(clock.date, "ddd d MMM   HH:mm")
  font.bold: true

  SystemClock {
    id: clock

    precision: SystemClock.Minutes
  }
}
