import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import qs.common

Label {
  id: root

  property var bar
  property var data: ({})
  property real now: Date.now()
  readonly property var current: data.current || null
  readonly property var daily: data.daily || null
  readonly property bool shown: current !== null && now - (data.fetched || 0) * 1000 < 3 * 3600 * 1000

  function glyph(code, day) {
    if (code === 0)
      return day ? "\u{f0599}" : "\u{f0594}";
    if (code <= 2)
      return day ? "\u{f0595}" : "\u{f0f31}";
    if (code === 3)
      return "\u{f0590}";
    if (code <= 48)
      return "\u{f0591}";
    if (code <= 57)
      return "\u{f0597}";
    if (code <= 67 || (code >= 80 && code <= 82))
      return "\u{f0596}";
    if (code <= 86)
      return "\u{f0598}";
    return "\u{f0593}";
  }

  function words(code) {
    if (code === 0)
      return "Clear";
    if (code === 1)
      return "Mostly clear";
    if (code === 2)
      return "Partly cloudy";
    if (code === 3)
      return "Cloudy";
    if (code <= 48)
      return "Fog";
    if (code <= 57)
      return "Drizzle";
    if (code <= 67)
      return "Rain";
    if (code <= 77)
      return "Snow";
    if (code <= 82)
      return "Showers";
    if (code <= 86)
      return "Snow showers";
    return "Thunderstorms";
  }

  text: current ? glyph(current.weather_code, current.is_day) + " " + Math.round(current.temperature_2m) + "°" : ""

  FileView {
    path: (Quickshell.env("XDG_CACHE_HOME") || Quickshell.env("HOME") + "/.cache") + "/hyprtrixie/weather.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: {
      try {
        root.data = JSON.parse(text());
      } catch (error) {
        root.data = {};
      }
    }
    onLoadFailed: root.data = {}
  }

  Timer {
    interval: 60000
    running: true
    repeat: true
    onTriggered: root.now = Date.now()
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: popup.toggle()
  }

  BarPopup {
    id: popup

    name: "weather"
    bar: root.bar
    owner: root

    RowLayout {
      Layout.preferredWidth: 300
      spacing: 14

      Label {
        text: root.current ? root.glyph(root.current.weather_code, root.current.is_day) : ""
        color: Theme.accent
        font.pixelSize: 34
      }

      ColumnLayout {
        Layout.fillWidth: true
        spacing: 2

        Label {
          text: root.current ? Math.round(root.current.temperature_2m) + root.data.current_units.temperature_2m + "  " + root.words(root.current.weather_code) : ""
          font.bold: true
        }

        Label {
          Layout.fillWidth: true
          wrapMode: Text.WordWrap
          text: root.current ? "Feels like " + Math.round(root.current.apparent_temperature) + "° · Humidity " + root.current.relative_humidity_2m + "% · Wind " + Math.round(root.current.wind_speed_10m) + " " + root.data.current_units.wind_speed_10m : ""
          color: Theme.subtext
        }
      }
    }

    GridLayout {
      Layout.fillWidth: true
      columns: 5
      columnSpacing: 14
      rowSpacing: 6

      Repeater {
        model: root.daily ? root.daily.time.length : 0

        ColumnLayout {
          required property int index

          Layout.alignment: Qt.AlignHCenter
          spacing: 2

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: index === 0 ? "Today" : Qt.formatDate(new Date(root.daily.time[index] + "T12:00"), "ddd")
            color: Theme.subtext
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: root.glyph(root.daily.weather_code[index], 1)
            font.pixelSize: 20
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: Math.round(root.daily.temperature_2m_max[index]) + "° " + Math.round(root.daily.temperature_2m_min[index]) + "°"
          }

          Label {
            Layout.alignment: Qt.AlignHCenter
            text: root.daily.precipitation_probability_max[index] + "%"
            color: Theme.subtext
          }
        }
      }
    }

    Label {
      text: "Weather from Open-Meteo"
      color: Theme.subtext
    }
  }
}
