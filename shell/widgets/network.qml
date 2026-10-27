import QtQuick
import Quickshell
import Quickshell.Networking
import qs.common

Label {
  id: root

  property var bar
  readonly property var device: {
    const devices = Networking.devices.values;
    return devices.find(item => item.connected && item.type === DeviceType.Wired) || devices.find(item => item.connected && item.type === DeviceType.Wifi) || null;
  }
  readonly property var network: device && device.type === DeviceType.Wifi ? device.networks.values.find(item => item.connected) || null : null

  function icon() {
    if (!device)
      return "\u{f092e}";
    if (device.type === DeviceType.Wired)
      return "\u{f0200}";
    const strength = network ? (network.signalStrength > 1 ? network.signalStrength / 100 : network.signalStrength) : 0;
    if (strength > 0.75)
      return "\u{f0928}";
    if (strength > 0.5)
      return "\u{f0925}";
    if (strength > 0.25)
      return "\u{f0922}";
    return "\u{f091f}";
  }

  text: network ? icon() + " " + network.name : icon()
  color: device ? Theme.text : Theme.subtext

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: Quickshell.execDetached(["kitty", "--class", "nmtui", "-e", "nmtui-connect"])
  }
}
