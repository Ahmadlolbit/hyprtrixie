import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.UPower
import qs.common

Label {
  id: root

  property var bar
  readonly property var device: UPower.displayDevice
  readonly property bool shown: device !== null && device.isLaptopBattery && device.isPresent
  readonly property real level: shown ? (device.percentage > 1 ? device.percentage / 100 : device.percentage) : 0
  readonly property bool charging: shown && (device.state === UPowerDeviceState.Charging || device.state === UPowerDeviceState.FullyCharged)
  readonly property var profiles: [
    {
      label: "Power saver",
      value: "power-saver",
      profile: PowerProfile.PowerSaver,
      icon: "\u{f032a}"
    },
    {
      label: "Balanced",
      value: "balanced",
      profile: PowerProfile.Balanced,
      icon: "\u{f05d1}"
    },
    {
      label: "Performance",
      value: "performance",
      profile: PowerProfile.Performance,
      icon: "\u{f04c5}"
    }
  ]

  function icon() {
    if (charging)
      return "\u{f0084}";
    const icons = ["\u{f0083}", "\u{f007a}", "\u{f007b}", "\u{f007c}", "\u{f007d}", "\u{f007e}", "\u{f007f}", "\u{f0080}", "\u{f0081}", "\u{f0082}", "\u{f0079}"];
    return icons[Math.round(level * 10)];
  }

  function duration(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.round(seconds % 3600 / 60);
    return hours > 0 ? hours + " h " + minutes + " min" : minutes + " min";
  }

  function status() {
    if (!shown)
      return "";
    if (device.state === UPowerDeviceState.FullyCharged)
      return "Fully charged";
    if (device.state === UPowerDeviceState.Charging)
      return device.timeToFull > 0 ? duration(device.timeToFull) + " until full" : "Charging";
    if (device.state === UPowerDeviceState.Discharging)
      return device.timeToEmpty > 0 ? duration(device.timeToEmpty) + " left" : "On battery";
    return "Plugged in";
  }

  text: icon() + " " + Math.round(level * 100) + "%"
  color: charging ? Theme.good : level < 0.15 ? Theme.critical : level < 0.3 ? Theme.warning : Theme.text

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "battery"
    bar: root.bar
    owner: root

    RowLayout {
      Layout.preferredWidth: 260
      spacing: 12

      Label {
        text: root.icon()
        color: root.color
        font.pixelSize: 26
      }

      ColumnLayout {
        Layout.fillWidth: true
        spacing: 2

        Label {
          Layout.fillWidth: true
          text: Math.round(root.level * 100) + "%"
          font.bold: true
        }

        Label {
          Layout.fillWidth: true
          text: root.status()
          color: Theme.subtext
        }
      }
    }

    Label {
      visible: root.shown && Math.abs(root.device.changeRate) > 0.1
      text: (root.charging ? "Charging at " : "Using ") + Math.abs(root.shown ? root.device.changeRate : 0).toFixed(1) + " W"
      color: Theme.subtext
    }

    Label {
      visible: root.shown && root.device.healthSupported
      text: "Battery health " + Math.round(root.shown ? root.device.healthPercentage : 0) + "%"
      color: Theme.subtext
    }

    Label {
      text: "Power mode"
      font.bold: true
    }

    Repeater {
      model: root.profiles.filter(item => item.profile !== PowerProfile.Performance || PowerProfiles.hasPerformanceProfile)

      Choice {
        required property var modelData

        Layout.fillWidth: true
        icon: modelData.icon
        label: modelData.label
        selected: PowerProfiles.profile === modelData.profile
        onClicked: Quickshell.execDetached(["powerprofilesctl", "set", modelData.value])
      }
    }
  }
}
