local function keyboard()
  local values = {}
  local file = io.open("/etc/default/keyboard", "r")
  if not file then
    return values
  end
  for line in file:lines() do
    local key, value = line:match('^%s*([%w_]+)%s*=%s*"?([^"]*)"?%s*$')
    if key then
      values[key] = value
    end
  end
  file:close()
  return values
end

local system = keyboard()
local layout = system.XKBLAYOUT or ""
local variant = system.XKBVARIANT or ""
local options = system.XKBOPTIONS or ""
local non_latin = " am ara bd bg by et ge gr il in iq ir kg kh kz la lk mk mm mn mv np rs ru sy th tj ua "

if layout == "" then
  layout = "us"
elseif non_latin:find(" " .. layout:match("^[^,]*") .. " ", 1, true) then
  layout = "us," .. layout
  variant = "," .. variant
  if not options:find("grp:", 1, true) then
    options = options == "" and "grp:alt_shift_toggle" or options .. ",grp:alt_shift_toggle"
  end
end

hl.config({
  input = {
    kb_layout = layout,
    kb_variant = variant,
    kb_model = system.XKBMODEL or "",
    kb_options = options,
    follow_mouse = 1,
    sensitivity = 0,
    repeat_rate = 40,
    repeat_delay = 250,
    numlock_by_default = true,
    touchpad = {
      natural_scroll = false,
      tap_to_click = true,
      disable_while_typing = true,
      clickfinger_behavior = true,
      scroll_factor = 0.4,
    },
  },
  misc = {
    key_press_enables_dpms = true,
    mouse_move_enables_dpms = true,
  },
})

hl.gesture({ fingers = 3, direction = "horizontal", action = "workspace" })
