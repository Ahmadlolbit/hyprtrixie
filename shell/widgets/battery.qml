import QtQuick
import Quickshell.Services.UPower
import qs.common

Label {
  id: root

  property var bar
  readonly property var device: UPower.displayDevice
  readonly property bool shown: device !== null && device.isLaptopBattery && device.isPresent
  readonly property real level: shown ? (device.percentage > 1 ? device.percentage / 100 : device.percentage) : 0
  readonly property bool charging: shown && (device.state === UPowerDeviceState.Charging || device.state === UPowerDeviceState.FullyCharged)

  function icon() {
    if (charging)
      return "\u{f0084}";
    const icons = ["\u{f0083}", "\u{f007a}", "\u{f007b}", "\u{f007c}", "\u{f007d}", "\u{f007e}", "\u{f007f}", "\u{f0080}", "\u{f0081}", "\u{f0082}", "\u{f0079}"];
    return icons[Math.round(level * 10)];
  }

  text: icon() + " " + Math.round(level * 100) + "%"
  color: charging ? Theme.good : level < 0.15 ? Theme.critical : level < 0.3 ? Theme.warning : Theme.text
}
