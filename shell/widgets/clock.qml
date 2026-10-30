import QtQuick
import QtQuick.Layouts
import Quickshell
import qs.common

Label {
  id: root

  property var bar
  property var month: new Date(clock.date.getFullYear(), clock.date.getMonth(), 1)

  function days() {
    const offset = (month.getDay() - Qt.locale().firstDayOfWeek + 7) % 7;
    return Array.from({
      length: 42
    }, (_, index) => new Date(month.getFullYear(), month.getMonth(), 1 - offset + index));
  }

  function shift(months) {
    month = new Date(month.getFullYear(), month.getMonth() + months, 1);
  }

  function reset() {
    month = new Date(clock.date.getFullYear(), clock.date.getMonth(), 1);
  }

  text: Qt.formatDateTime(clock.date, root.bar && root.bar.vertical ? (Config.clock === "12h" ? "h\nmm" : "HH\nmm") : Config.clock === "12h" ? "ddd d MMM   h:mm AP" : "ddd d MMM   HH:mm")
  horizontalAlignment: Text.AlignHCenter
  font.bold: true

  SystemClock {
    id: clock

    precision: SystemClock.Minutes
  }

  MouseArea {
    anchors.fill: parent
    cursorShape: Qt.PointingHandCursor
    onClicked: {
      root.reset();
      popup.toggle();
    }
  }

  BarPopup {
    id: popup

    name: "calendar"
    bar: root.bar
    owner: root

    RowLayout {
      Layout.fillWidth: true
      spacing: 10

      Label {
        text: "\u{f0141}"

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: root.shift(-1)
        }
      }

      Label {
        Layout.fillWidth: true
        horizontalAlignment: Text.AlignHCenter
        text: Qt.locale().standaloneMonthName(root.month.getMonth()) + " " + root.month.getFullYear()
        font.bold: true

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: root.reset()
        }
      }

      Label {
        text: "\u{f0142}"

        MouseArea {
          anchors.fill: parent
          cursorShape: Qt.PointingHandCursor
          onClicked: root.shift(1)
        }
      }
    }

    GridLayout {
      columns: 7
      columnSpacing: 2
      rowSpacing: 2

      Repeater {
        model: 7

        Label {
          required property int index

          Layout.preferredWidth: 34
          horizontalAlignment: Text.AlignHCenter
          text: Qt.locale().dayName((Qt.locale().firstDayOfWeek + index) % 7, Locale.ShortFormat)
          color: Theme.subtext
        }
      }

      Repeater {
        model: root.days()

        Rectangle {
          id: cell

          required property var modelData
          readonly property bool today: modelData.toDateString() === clock.date.toDateString()

          Layout.preferredWidth: 34
          Layout.preferredHeight: 28
          radius: Theme.radius
          color: today ? Theme.accent : "transparent"

          Label {
            anchors.centerIn: parent
            text: cell.modelData.getDate()
            color: cell.today ? Theme.base : cell.modelData.getMonth() === root.month.getMonth() ? Theme.text : Theme.subtext
          }
        }
      }
    }
  }
}
