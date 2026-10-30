import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Bluetooth
import qs.common

Label {
  id: root

  property var bar
  readonly property var adapter: Bluetooth.defaultAdapter
  readonly property bool shown: adapter !== null
  readonly property var devices: adapter ? adapter.devices.values : []
  readonly property bool linked: devices.some(device => device.connected)

  function icon() {
    if (!adapter || !adapter.enabled)
      return "\u{f00b2}";
    return linked ? "\u{f00b1}" : "\u{f00af}";
  }

  function deviceIcon(device) {
    const kind = device.icon || "";
    if (kind.startsWith("audio"))
      return "\u{f02cb}";
    if (kind === "input-keyboard")
      return "\u{f030c}";
    if (kind === "input-mouse")
      return "\u{f037d}";
    if (kind === "input-gaming")
      return "\u{f0297}";
    if (kind === "phone")
      return "\u{f03f2}";
    return "\u{f00af}";
  }

  function detail(device) {
    if (device.pairing)
      return "Pairing...";
    if (device.state === BluetoothDeviceState.Connecting)
      return "Connecting...";
    if (device.connected)
      return device.batteryAvailable ? Math.round(device.battery * 100) + "%" : "Connected";
    return device.paired ? "Paired" : "";
  }

  function choose(device) {
    if (device.connected)
      device.disconnect();
    else if (device.paired)
      device.connect();
    else
      device.pair();
  }

  text: icon()
  color: adapter && adapter.enabled ? Theme.text : Theme.subtext

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "bluetooth"
    bar: root.bar
    owner: root
    onVisibleChanged: {
      if (!visible && root.adapter && root.adapter.discovering)
        root.adapter.discovering = false;
    }

    RowLayout {
      Layout.fillWidth: true
      Layout.preferredWidth: 300
      spacing: 10

      Label {
        Layout.fillWidth: true
        text: "Bluetooth"
        font.bold: true
      }

      Toggle {
        visible: root.adapter !== null
        checked: root.adapter ? root.adapter.enabled : false
        onToggled: root.adapter.enabled = !root.adapter.enabled
      }
    }

    Label {
      visible: !list.visible
      text: !root.adapter ? "No Bluetooth adapter" : !root.adapter.enabled ? "Bluetooth is off" : root.adapter.discovering ? "Looking for devices..." : "No devices yet; Scan finds new ones"
      color: Theme.subtext
    }

    ListView {
      id: list

      visible: count > 0
      Layout.fillWidth: true
      Layout.preferredHeight: Math.min(contentHeight, 300)
      clip: true
      spacing: 2
      model: ScriptModel {
        values: root.adapter && root.adapter.enabled ? root.devices.filter(device => device.paired || root.adapter.discovering).sort((a, b) => b.connected - a.connected || b.paired - a.paired || a.name.localeCompare(b.name)) : []
      }

      delegate: Choice {
        id: choice

        required property var modelData

        width: list.width
        icon: root.deviceIcon(modelData)
        label: modelData.name || modelData.address
        detail: root.detail(modelData)
        selected: modelData.connected
        action: modelData.paired ? "Forget" : ""
        onClicked: root.choose(modelData)
        onActionClicked: modelData.forget()

        Connections {
          target: choice.modelData

          function onPairedChanged() {
            if (!choice.modelData.paired)
              return;
            choice.modelData.trusted = true;
            choice.modelData.connect();
          }
        }
      }
    }

    RowLayout {
      spacing: 16

      Label {
        visible: root.adapter !== null && root.adapter.enabled
        text: root.adapter && root.adapter.discovering ? "Stop scanning" : "Scan"
        color: Theme.accent

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: root.adapter.discovering = !root.adapter.discovering
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
            Quickshell.execDetached(["blueman-manager"]);
          }
        }
      }
    }
  }
}
