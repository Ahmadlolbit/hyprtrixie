local root = os.getenv("HYPRTRIXIE_SYS_ROOT") or ""

local function first_line(path)
  local file = io.open(path, "r")
  if not file then
    return nil
  end
  local line = file:read("l")
  file:close()
  return line
end

local function lid_closed()
  for _, name in ipairs({ "LID", "LID0", "LID1" }) do
    local state = first_line(root .. "/proc/acpi/button/lid/" .. name .. "/state")
    if state then
      return state:find("closed", 1, true) ~= nil
    end
  end
  return false
end

local function internal(output)
  return output:match("^eDP") or output:match("^LVDS") or output:match("^DSI")
end

local function screens()
  local panels, others = {}, 0
  local listing = io.popen("ls " .. root .. "/sys/class/drm 2>/dev/null")
  if not listing then
    return panels, others
  end
  for entry in listing:lines() do
    local output = entry:match("^card%d+%-(.+)$")
    if output and first_line(root .. "/sys/class/drm/" .. entry .. "/status") == "connected" then
      if internal(output) then
        table.insert(panels, output)
      else
        others = others + 1
      end
    end
  end
  listing:close()
  return panels, others
end

local panels, others = screens()
if others > 0 and lid_closed() then
  for _, output in ipairs(panels) do
    hl.monitor({ output = output, disabled = true })
  end
end

hl.bind("switch:on:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
hl.bind("switch:off:Lid Switch", hl.dsp.exec_cmd("hyprctl reload"), { locked = true })
