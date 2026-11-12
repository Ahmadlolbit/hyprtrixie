import QtQuick

QtObject {
  property real rest: 0

  function steps(wheel) {
    const delta = wheel.angleDelta.y;
    rest = delta * rest < 0 ? delta : rest + delta;
    const count = Math.trunc(rest / 120);
    rest -= count * 120;
    return count;
  }
}
