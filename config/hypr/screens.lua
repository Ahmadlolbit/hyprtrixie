local screens = {}

local root = os.getenv("HYPRTRIXIE_SYS_ROOT") or ""

function screens.first_line(path)
  local file = io.open(root .. path, "r")
  if not file then
    return nil
  end
  local line = file:read("l")
  file:close()
  return line
end

local function internal(output)
  return output:match("^eDP") or output:match("^LVDS") or output:match("^DSI")
end

function screens.connected()
  local panels, others = {}, {}
  local listing = io.popen("ls " .. root .. "/sys/class/drm 2>/dev/null")
  if not listing then
    return panels, others
  end
  for entry in listing:lines() do
    local output = entry:match("^card%d+%-(.+)$")
    if output and screens.first_line("/sys/class/drm/" .. entry .. "/status") == "connected" then
      table.insert(internal(output) and panels or others, output)
    end
  end
  listing:close()
  return panels, others
end

function screens.lid_closed()
  for _, name in ipairs({ "LID", "LID0", "LID1" }) do
    local state = screens.first_line("/proc/acpi/button/lid/" .. name .. "/state")
    if state then
      return state:find("closed", 1, true) ~= nil
    end
  end
  return false
end

return screens
