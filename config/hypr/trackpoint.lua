local look = require("hyprtrixie.hypr.look")

local function trackpoints()
  local names = {}
  local file = io.open(os.getenv("HYPRTRIXIE_INPUT_DEVICES") or "/proc/bus/input/devices", "r")
  if not file then
    return names
  end
  for line in file:lines() do
    local name = line:match('^N: Name="(.*)"$')
    if name and name:lower():find("trackpoint", 1, true) then
      table.insert(names, (name:lower():gsub(" ", "-")))
    end
  end
  file:close()
  return names
end

local speed = tonumber(look.get("trackpoint_speed") or "") or 0
local accel = look.get("trackpoint_accel") == "flat" and "flat" or "adaptive"
local scroll = look.get("trackpoint_scroll") == "no" and "no_scroll" or "on_button_down"

for _, name in ipairs(trackpoints()) do
  hl.device({ name = name, sensitivity = speed, accel_profile = accel, scroll_method = scroll })
end
