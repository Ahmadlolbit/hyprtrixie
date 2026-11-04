local look = {}

local function setting(path, key)
  local file = io.open(path, "r")
  if not file then
    return nil
  end
  local text = "\n" .. file:read("a")
  file:close()
  return text:match("\n" .. key .. "=([^\n]*)")
end

function look.get(key)
  local config = os.getenv("XDG_CONFIG_HOME")
  if not config or config == "" then
    config = (os.getenv("HOME") or "") .. "/.config"
  end
  return setting(config .. "/hyprtrixie/look.conf", key) or setting("/etc/hyprtrixie/look.conf", key)
end

return look
