import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io
import Quickshell.Wayland
import qs.common

Scope {
  id: root

  readonly property int rowHeight: 40
  readonly property int visibleRows: 11
  property bool open: false
  property var stack: ["root"]
  readonly property string page: stack[stack.length - 1]
  property string filter: ""
  property int index: 0
  property int stamp: 0
  property var entries: ({})
  property var order: []
  property var listed: []
  readonly property var rows: filter ? search(filter) : childrenOf(page).map(entry => row(entry)).concat(listed)
  readonly property var screen: Quickshell.screens.find(item => Hyprland.focusedMonitor && item.name === Hyprland.focusedMonitor.name) || Quickshell.screens[0]
  readonly property var desktopApps: DesktopEntries.applications.values

  onDesktopAppsChanged: {
    const entry = entries[page];
    if (open && entry && entry.list === "apps")
      listed = apps();
  }

  function parse(text) {
    try {
      const data = JSON.parse(text);
      return data && typeof data === "object" && !Array.isArray(data) ? data : {};
    } catch (error) {
      return {};
    }
  }

  function load() {
    const merged = {};
    const keys = [];
    for (const source of [parse(systemFile.text()), parse(userFile.text())]) {
      for (const id in source) {
        const value = source[id];
        if (!value || typeof value !== "object" || Array.isArray(value))
          continue;
        if (!merged[id])
          keys.push(id);
        merged[id] = Object.assign({}, merged[id] || {}, value, {
          id: id
        });
      }
    }
    entries = merged;
    order = keys;
  }

  function parentOf(id) {
    const at = id.lastIndexOf(".");
    return at < 0 ? "root" : id.slice(0, at);
  }

  function childrenOf(id) {
    return order.filter(key => parentOf(key) === id).map(key => entries[key]);
  }

  function isPage(entry) {
    return !!entry.list || order.some(key => parentOf(key) === entry.id);
  }

  function row(entry) {
    return {
      id: entry.id,
      glyph: entry.glyph || "",
      label: entry.label || entry.id,
      action: entry.action || "",
      shell: entry.shell || "",
      state: entry.state || "",
      color: entry.color || "",
      font: "",
      page: isPage(entry),
      icon: "",
      app: null,
      path: ""
    };
  }

  function trail(id) {
    const labels = [];
    for (let at = id; at !== "root" && entries[at]; at = parentOf(at))
      labels.unshift(entries[at].label || at);
    return labels.join(" › ");
  }

  function search(text) {
    const needle = text.toLowerCase();
    const found = listed.filter(item => item.label.toLowerCase().includes(needle));
    return found.concat(order.map(key => entries[key]).filter(entry => !isPage(entry) && ((entry.label || "") + " " + (entry.keywords || []).join(" ")).toLowerCase().includes(needle)).map(entry => Object.assign(row(entry), {
          path: trail(parentOf(entry.id))
        })));
  }

  function title() {
    return stack.map(id => id === "root" ? "Menu" : entries[id] ? entries[id].label : id).join(" › ");
  }

  function show(id) {
    load();
    filter = "";
    const path = [];
    for (let at = id; at && at !== "root" && entries[at]; at = parentOf(at))
      path.unshift(at);
    stack = ["root"].concat(path);
    open = true;
    enter();
    content.forceActiveFocus();
  }

  function enter() {
    index = 0;
    listed = [];
    const entry = entries[page];
    if (entry && entry.list)
      provide(entry.list);
  }

  function provide(kind) {
    if (kind === "apps") {
      listed = apps();
      return;
    }
    lister.kind = kind;
    if (kind === "fonts" || kind === "ui-fonts")
      lister.command = ["fc-list", "--format", "%{family[0]}\n", kind === "fonts" ? ":spacing=100" : ":"];
    else
      lister.command = ["hyprtrixie", "app", "list", kind === "install" ? "available" : "installed"];
    lister.running = true;
  }

  function apps() {
    const values = desktopApps || [];
    return values.filter(entry => !entry.noDisplay).sort((a, b) => a.name.localeCompare(b.name)).map(entry => ({
          id: "app:" + entry.id,
          glyph: "\u{f003b}",
          label: entry.name,
          action: "",
          shell: "",
          state: "",
          color: "",
          font: "",
          page: false,
          icon: entry.icon || "",
          app: entry,
          path: ""
        }));
  }

  function catalogue(kind, text) {
    return text.split("\n").filter(line => line.includes("\t")).map(line => {
      const fields = line.split("\t");
      return {
        id: kind + ":" + fields[0],
        glyph: kind === "install" ? "\u{f01da}" : "\u{f0b4c}",
        label: fields[1],
        action: "hyprtrixie-terminal --wait sudo hyprtrixie app " + kind + " " + fields[0],
        shell: "",
        state: "",
        color: "",
        font: "",
        page: false,
        icon: "",
        app: null,
        path: ""
      };
    });
  }

  function quote(text) {
    return "'" + text.replace(/'/g, "'\\''") + "'";
  }

  function fonts(kind, text) {
    const setting = kind === "fonts" ? "font" : "ui-font";
    const names = [...new Set(text.split("\n").map(line => line.trim()).filter(name => /^[A-Za-z0-9][A-Za-z0-9 ._+-]*$/.test(name)))];
    return names.sort((a, b) => a.localeCompare(b)).map(name => ({
          id: kind + ":" + name,
          glyph: "\u{f06d6}",
          label: name,
          action: "hyprtrixie-look " + setting + " " + quote(name),
          shell: "",
          state: "test \"$(hyprtrixie-look " + setting + ")\" = " + quote(name),
          color: "",
          font: name,
          page: false,
          icon: "",
          app: null,
          path: ""
        }));
  }

  function activate(item) {
    if (!item)
      return;
    if (item.page) {
      stack = stack.concat([item.id]);
      filter = "";
      enter();
      return;
    }
    if (item.app) {
      item.app.execute();
      close();
      return;
    }
    if (item.shell === "awake") {
      Session.awake = !Session.awake;
      return;
    }
    if (item.shell.startsWith("awake ")) {
      Session.stayAwake(Number(item.shell.slice(6)));
      close();
      return;
    }
    if (item.shell.startsWith("focus ")) {
      if (item.shell === "focus stop")
        Focus.stop();
      else
        Focus.start(Number(item.shell.slice(6)));
      close();
      return;
    }
    if (item.action)
      Quickshell.execDetached(["sh", "-c", item.action]);
    if (item.state) {
      refresh.restart();
      return;
    }
    close();
  }

  function back() {
    if (stack.length > 1) {
      stack = stack.slice(0, -1);
      enter();
    } else {
      close();
    }
  }

  function move(step) {
    if (rows.length)
      index = (index + step + rows.length) % rows.length;
  }

  function close() {
    open = false;
  }

  onFilterChanged: index = 0

  FileView {
    id: systemFile

    path: Quickshell.shellPath("menu.json")
    blockLoading: true
    printErrors: false
  }

  FileView {
    id: userFile

    path: (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/hyprtrixie/menu.json"
    blockLoading: true
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
  }

  Process {
    id: lister

    property string kind: ""

    stdout: StdioCollector {
      onStreamFinished: {
        const entry = root.entries[root.page];
        if (entry && entry.list === lister.kind)
          root.listed = lister.kind.endsWith("fonts") ? root.fonts(lister.kind, text) : root.catalogue(lister.kind, text);
      }
    }
  }

  Timer {
    id: refresh

    interval: 600
    onTriggered: root.stamp++
  }

  IpcHandler {
    target: "menu"

    function open(page: string): void {
      root.show(page);
    }

    function toggle(page: string): void {
      if (root.open)
        root.close();
      else
        root.show(page);
    }

    function close(): void {
      root.close();
    }
  }

  PanelWindow {
    screen: root.screen
    visible: root.open || content.opacity > 0
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "hyprtrixie-menu"
    WlrLayershell.keyboardFocus: root.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    anchors {
      top: true
      bottom: true
      left: true
      right: true
    }

    Rectangle {
      id: content

      anchors.fill: parent
      color: Qt.alpha(Theme.base, 0.45)
      opacity: root.open ? 1 : 0
      focus: true
      Keys.onPressed: event => {
        if (event.key === Qt.Key_Escape)
          root.close();
        else if (event.key === Qt.Key_Down || event.key === Qt.Key_Tab)
          root.move(1);
        else if (event.key === Qt.Key_Up || event.key === Qt.Key_Backtab)
          root.move(-1);
        else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter)
          root.activate(root.rows[root.index]);
        else if (event.key === Qt.Key_Right && !root.filter && root.rows[root.index] && root.rows[root.index].page)
          root.activate(root.rows[root.index]);
        else if (event.key === Qt.Key_Backspace && root.filter)
          root.filter = root.filter.slice(0, -1);
        else if ((event.key === Qt.Key_Backspace || event.key === Qt.Key_Left) && !root.filter)
          root.back();
        else if (/^[\w .+-]$/.test(event.text))
          root.filter += event.text;
        else
          return;
        event.accepted = true;
      }

      Behavior on opacity {
        NumberAnimation {
          duration: 160
          easing.type: Easing.OutCubic
        }
      }

      MouseArea {
        anchors.fill: parent
        onClicked: root.close()
      }

      Rectangle {
        anchors.centerIn: parent
        width: 480
        height: header.height + list.height + 36
        color: Theme.panel
        border.color: Theme.accent
        border.width: 2
        radius: Theme.radius

        MouseArea {
          anchors.fill: parent
        }

        RowLayout {
          id: header

          anchors.top: parent.top
          anchors.left: parent.left
          anchors.right: parent.right
          anchors.margins: 18
          height: 28

          Label {
            Layout.fillWidth: true
            text: root.title()
            font.bold: true
            elide: Text.ElideLeft
          }

          Label {
            text: root.filter ? "\u{f0349} " + root.filter : "Type to search"
            color: root.filter ? Theme.text : Theme.subtext
          }
        }

        ListView {
          id: list

          anchors.top: header.bottom
          anchors.left: parent.left
          anchors.right: parent.right
          anchors.margins: 10
          anchors.topMargin: 8
          height: Math.max(1, Math.min(root.rows.length, root.visibleRows)) * root.rowHeight
          clip: true
          model: root.rows
          currentIndex: root.index
          boundsBehavior: Flickable.StopAtBounds
          highlightMoveDuration: 0
          onCurrentIndexChanged: positionViewAtIndex(currentIndex, ListView.Contain)

          delegate: Rectangle {
            id: item

            required property var modelData
            required property int index
            readonly property bool current: index === root.index
            property bool on: false

            width: ListView.view.width
            height: root.rowHeight
            radius: Theme.radius
            color: current ? Theme.overlay : "transparent"

            Process {
              id: probe

              command: ["sh", "-c", item.modelData.state]
              running: !!item.modelData.state && root.open
              onExited: code => item.on = code === 0
            }

            Connections {
              target: root

              function onStampChanged() {
                if (item.modelData.state)
                  probe.running = true;
              }
            }

            RowLayout {
              anchors.fill: parent
              anchors.leftMargin: 12
              anchors.rightMargin: 12
              spacing: 12

              Item {
                Layout.preferredWidth: 22
                Layout.preferredHeight: 22

                Image {
                  id: icon

                  anchors.fill: parent
                  source: item.modelData.icon ? Quickshell.iconPath(item.modelData.icon, true) : ""
                  sourceSize.width: 44
                  sourceSize.height: 44
                  visible: status === Image.Ready
                }

                Label {
                  anchors.centerIn: parent
                  visible: !icon.visible
                  text: item.modelData.glyph
                  color: Theme.palette[item.modelData.color] || Theme.accent
                  font.pixelSize: 18
                }
              }

              Label {
                Layout.fillWidth: true
                text: item.modelData.label
                font.family: item.modelData.font || Theme.font
                elide: Text.ElideRight
              }

              Label {
                visible: !!item.modelData.path
                text: item.modelData.path
                color: Theme.subtext
              }

              Label {
                visible: item.modelData.page || !!item.modelData.state || item.modelData.shell === "awake"
                text: item.modelData.page ? "\u{f0142}" : (item.modelData.shell === "awake" ? Session.awake : item.on) ? "\u{f0765}" : "\u{f0766}"
                color: item.modelData.page ? Theme.subtext : (item.modelData.shell === "awake" ? Session.awake : item.on) ? Theme.accent : Theme.subtext
                font.pixelSize: 16
              }
            }

            MouseArea {
              anchors.fill: parent
              hoverEnabled: true
              cursorShape: Qt.PointingHandCursor
              onEntered: root.index = item.index
              onClicked: root.activate(item.modelData)
            }
          }
        }

        Label {
          anchors.centerIn: list
          visible: root.rows.length === 0
          text: root.filter ? "Nothing matches " + root.filter : "Nothing here yet"
          color: Theme.subtext
        }
      }
    }
  }
}
