Name = "hyprtrixie-backgrounds"
NamePretty = "Backgrounds"
Icon = "preferences-desktop-wallpaper"
Parent = "hyprtrixie-style"
Action = "hyprtrixie-background '%VALUE%'"

local function lines(command)
  local found = {}
  local handle = io.popen(command)
  if handle then
    for line in handle:lines() do
      table.insert(found, line)
    end
    handle:close()
  end
  return found
end

function GetEntries()
  local current = lines("hyprtrixie-background --current 2>/dev/null")[1] or ""
  local entries = {}
  for _, path in ipairs(lines("hyprtrixie-background --list 2>/dev/null")) do
    table.insert(entries, {
      Text = path:match("([^/]+)%.%w+$") or path,
      Value = path,
      Icon = path,
      Preview = path,
      PreviewType = "file",
      State = path == current and { "current" } or {},
    })
  end
  return entries
end
