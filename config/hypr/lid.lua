local screens = require("hyprtrixie.hypr.screens")

local function lid_closed()
  for _, name in ipairs({ "LID", "LID0", "LID1" }) do
    local state = screens.first_line("/proc/acpi/button/lid/" .. name .. "/state")
    if state then
      return state:find("closed", 1, true) ~= nil
    end
  end
  return false
end

local panels, others = screens.connected()
if #others > 0 and lid_closed() then
  for _, output in ipairs(panels) do
    hl.monitor({ output = output, disabled = true })
  end
end

hl.bind("switch:on:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
hl.bind("switch:off:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
