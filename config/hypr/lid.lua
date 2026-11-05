local screens = require("hyprtrixie.hypr.screens")

local panels, others = screens.connected()
if #others > 0 and screens.lid_closed() then
  for _, output in ipairs(panels) do
    hl.monitor({ output = output, disabled = true })
  end
end

hl.bind("switch:on:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
hl.bind("switch:off:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
