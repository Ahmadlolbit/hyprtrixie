Name = "hyprtrixie-themes"
NamePretty = "Themes"
Icon = "preferences-desktop-theme"
Parent = "hyprtrixie-style"
Action = "omarchy-theme-set %VALUE%"

local function first_line(path)
  local file = io.open(path, "r")
  if not file then
    return nil
  end
  local line = file:read("*l")
  file:close()
  return line
end

local function exists(path)
  local file = io.open(path, "r")
  if not file then
    return false
  end
  file:close()
  return true
end

local function pretty(name)
  return (name:gsub("-", " "):gsub("(%a)(%w*)", function(head, tail)
    return head:upper() .. tail
  end))
end

function GetEntries()
  local home = os.getenv("HOME") or ""
  local state = (os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")) .. "/omarchy/current/theme.name"
  local roots = {
    (os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")) .. "/omarchy/themes",
    "/usr/share/omarchy/themes",
  }
  local current = first_line(state) or ""
  local names, seen = {}, {}
  for _, root in ipairs(roots) do
    local handle = io.popen("find '" .. root .. "' -mindepth 1 -maxdepth 1 -printf '%f\\n' 2>/dev/null")
    if handle then
      for name in handle:lines() do
        if not seen[name] and not name:find("^%.") then
          seen[name] = true
          table.insert(names, name)
        end
      end
      handle:close()
    end
  end
  table.sort(names)
  local entries = {}
  for _, name in ipairs(names) do
    local preview = ""
    for _, root in ipairs(roots) do
      if preview == "" and exists(root .. "/" .. name .. "/preview.png") then
        preview = root .. "/" .. name .. "/preview.png"
      end
    end
    table.insert(entries, {
      Text = pretty(name),
      Value = name,
      Icon = preview,
      Preview = preview,
      PreviewType = "file",
      State = name == current and { "current" } or {},
    })
  end
  return entries
end
