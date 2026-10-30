import QtQuick
import QtQuick.Layouts
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
  readonly property var wifi: Networking.devices.values.find(item => item.type === DeviceType.Wifi) || null
  property var asking: null
  property string failure: ""

  function strength(network) {
    return network.signalStrength > 1 ? network.signalStrength / 100 : network.signalStrength;
  }

  function signalIcon(level) {
    if (level > 0.75)
      return "\u{f0928}";
    if (level > 0.5)
      return "\u{f0925}";
    if (level > 0.25)
      return "\u{f0922}";
    return "\u{f091f}";
  }

  function icon() {
    if (!device)
      return "\u{f092e}";
    if (device.type === DeviceType.Wired)
      return "\u{f0200}";
    return signalIcon(network ? strength(network) : 0);
  }

  function open(network) {
    return network.security === WifiSecurityType.Open || network.security === WifiSecurityType.Owe;
  }

  function needsPassword(network) {
    return [WifiSecurityType.Sae, WifiSecurityType.Wpa2Psk, WifiSecurityType.WpaPsk, WifiSecurityType.StaticWep].includes(network.security);
  }

  function choose(network) {
    failure = "";
    if (network.connected)
      return;
    if (network.known || open(network)) {
      asking = null;
      network.connect();
    } else if (needsPassword(network)) {
      asking = network;
      password.text = "";
      password.focusInput();
    } else {
      asking = null;
      failure = network.name + " needs more settings";
    }
  }

  function reason(code) {
    if (code === ConnectionFailReason.NoSecrets || code === ConnectionFailReason.WifiAuthTimeout)
      return "wrong password";
    if (code === ConnectionFailReason.WifiNetworkLost)
      return "out of range";
    return "could not connect";
  }

  text: network && !(bar && bar.vertical) ? icon() + " " + network.name : icon()
  color: device ? Theme.text : Theme.subtext

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "wifi"
    bar: root.bar
    owner: root
    onVisibleChanged: {
      root.asking = null;
      root.failure = "";
      if (root.wifi)
        root.wifi.scannerEnabled = visible;
    }

    RowLayout {
      Layout.fillWidth: true
      Layout.preferredWidth: 300
      spacing: 10

      Label {
        Layout.fillWidth: true
        text: "Wi-Fi"
        font.bold: true
      }

      Toggle {
        visible: root.wifi !== null
        checked: Networking.wifiEnabled
        onToggled: Networking.wifiEnabled = !Networking.wifiEnabled
      }
    }

    Label {
      visible: !list.visible
      text: !root.wifi ? "No Wi-Fi adapter" : !Networking.wifiHardwareEnabled ? "Wi-Fi is off with the hardware switch" : !Networking.wifiEnabled ? "Wi-Fi is off" : "Looking for networks..."
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
        values: root.wifi && Networking.wifiEnabled ? root.wifi.networks.values.filter(item => item.name !== "").sort((a, b) => b.connected - a.connected || root.strength(b) - root.strength(a)) : []
      }

      delegate: Choice {
        id: choice

        required property var modelData

        width: list.width
        icon: root.signalIcon(root.strength(modelData))
        label: modelData.name
        detail: modelData.connected ? "Connected" : modelData.stateChanging ? "Connecting..." : modelData.known ? "Saved" : root.open(modelData) ? "" : "\u{f033e}"
        selected: modelData.connected
        action: modelData.known ? "Forget" : ""
        onClicked: root.choose(modelData)
        onActionClicked: modelData.forget()

        Connections {
          target: choice.modelData

          function onConnectionFailed(reason) {
            root.failure = choice.modelData.name + ": " + root.reason(reason);
          }
        }
      }
    }

    ColumnLayout {
      visible: root.asking !== null
      Layout.fillWidth: true
      spacing: 6

      Label {
        text: root.asking ? "Password for " + root.asking.name : ""
        color: Theme.subtext
      }

      Field {
        id: password

        Layout.fillWidth: true
        secret: true
        placeholder: "Password"
        onAccepted: {
          if (root.asking && text !== "")
            root.asking.connectWithPsk(text);
          root.asking = null;
        }
      }
    }

    Label {
      visible: root.failure !== ""
      text: root.failure
      color: Theme.critical
    }

    Label {
      text: "More settings"
      color: Theme.accent

      MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
          Popups.close(popup);
          Quickshell.execDetached(["kitty", "--class", "nmtui", "-e", "nmtui-connect"]);
        }
      }
    }
  }
}
