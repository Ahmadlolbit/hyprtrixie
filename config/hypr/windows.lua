local look = require("hyprtrixie.hypr.look")

local scale = ({ large = 1.25, larger = 1.5 })[look.get("text")] or 1
local bar_height = math.floor(30 * scale + 0.5)
local bar_width = math.floor(60 * scale + 0.5)
local preset = look.bar("preset") or "full"
local dropdown_move, dropdown_size = "0 " .. bar_height, "monitor_w monitor_h*0.45"
if preset == "vertical" then
  dropdown_move, dropdown_size = bar_width .. " 0", "(monitor_w-" .. bar_width .. ") (monitor_h*0.45)"
elseif look.bar("position") == "bottom" then
  dropdown_move = "0 0"
elseif preset == "floating" then
  dropdown_move = "0 " .. (bar_height + 8)
end

hl.window_rule({
  name = "suppress-maximize",
  match = { class = ".*" },
  suppress_event = "maximize",
})

hl.window_rule({
  name = "xwayland-drag-fix",
  match = { class = "^$", title = "^$", xwayland = true, float = true, fullscreen = false, pin = false },
  no_focus = true,
})

hl.window_rule({
  name = "float-system-tools",
  match = { class = "^(org\\.pulseaudio\\.pavucontrol|blueman-manager|nm-connection-editor|xdg-desktop-portal-gtk)$" },
  float = true,
  center = true,
  size = { 900, 600 },
})

hl.window_rule({
  name = "float-file-dialogs",
  match = { title = "^(Open.*Files?|Open [Ff]older.*|Save.*Files?|Save.*As|Save|All Files|[Cc]hoose.*)$" },
  float = true,
  center = true,
  size = { 900, 600 },
})

hl.window_rule({
  name = "picture-in-picture",
  match = { title = "^(Picture.?in.?[Pp]icture)$" },
  float = true,
  pin = true,
  size = { 600, 338 },
  keep_aspect_ratio = true,
  move = { "(monitor_w-window_w-40)", "(monitor_h*0.04)" },
})

hl.window_rule({
  name = "screensaver",
  match = { class = "^hyprtrixie\\.screensaver$" },
  fullscreen = true,
})

hl.window_rule({
  name = "floating-terminal",
  match = { class = "^hyprtrixie\\.floating$" },
  float = true,
  center = true,
  size = { "(monitor_w*0.66)", "(monitor_h*0.66)" },
})

hl.window_rule({
  name = "dropdown-terminal",
  match = { class = "^hyprtrixie-dropdown$" },
  float = true,
  size = dropdown_size,
  move = dropdown_move,
  workspace = "special:dropdown silent",
})
