local function bind(keys, description, dispatcher, options)
  local opts = options or {}
  opts.description = description
  if type(dispatcher) == "string" then
    dispatcher = hl.dsp.exec_cmd(dispatcher)
  end
  hl.bind(keys, dispatcher, opts)
end

bind("SUPER + Return", "Terminal", "kitty")
bind("SUPER + E", "Files", "xdg-open ~")
bind("SUPER + B", "Browser", [[sh -c 'gtk-launch "$(xdg-mime query default x-scheme-handler/https)" || exec x-www-browser']])
bind("SUPER + Q", "Close window", hl.dsp.window.close())
bind("SUPER + F", "Full screen", hl.dsp.window.fullscreen({ mode = "fullscreen" }))
bind("SUPER + T", "Float or tile window", hl.dsp.window.float({ action = "toggle" }))
bind("SUPER + J", "Change split direction", hl.dsp.layout("togglesplit"))
bind("SUPER + P", "Pseudo-tile window", hl.dsp.window.pseudo())
bind("SUPER + L", "Lock", "pidof hyprlock || hyprlock")
bind("SUPER + Space", "Launcher", "walker")
bind("SUPER + ALT + Space", "Menu", "qs -p /usr/share/hyprtrixie/shell ipc call menu toggle root")
bind("SUPER + Escape", "System menu", "qs -p /usr/share/hyprtrixie/shell ipc call menu toggle system")
bind("SUPER + CTRL + T", "Theme", "qs -p /usr/share/hyprtrixie/shell ipc call picker themes")
bind("SUPER + K", "Key bindings", "qs -p /usr/share/hyprtrixie/shell ipc call keys toggle")
bind("Print", "Screenshot of a region", "hyprtrixie-screenshot region")
bind("SHIFT + Print", "Screenshot of the window", "hyprtrixie-screenshot window")
bind("CTRL + Print", "Screenshot of the screen", "hyprtrixie-screenshot screen")

local arrows = {
  { "LEFT", "left", -50, 0 },
  { "RIGHT", "right", 50, 0 },
  { "UP", "up", 0, -50 },
  { "DOWN", "down", 0, 50 },
}
for _, arrow in ipairs(arrows) do
  local key, direction, x, y = table.unpack(arrow)
  bind("SUPER + " .. key, "Focus " .. direction, hl.dsp.focus({ direction = direction }))
  bind("SUPER + SHIFT + " .. key, "Move window " .. direction, hl.dsp.window.move({ direction = direction }))
  bind("SUPER + CTRL + " .. key, "Resize window " .. direction, hl.dsp.window.resize({ x = x, y = y, relative = true }), { repeating = true })
end

for workspace = 1, 10 do
  local key = "code:" .. tostring(workspace + 9)
  bind("SUPER + " .. key, "Workspace " .. workspace, hl.dsp.focus({ workspace = tostring(workspace) }))
  bind("SUPER + SHIFT + " .. key, "Move window to workspace " .. workspace, hl.dsp.window.move({ workspace = tostring(workspace) }))
end

bind("SUPER + S", "Scratchpad", hl.dsp.workspace.toggle_special("scratchpad"))
bind("SUPER + ALT + S", "Move window to scratchpad", hl.dsp.window.move({ workspace = "special:scratchpad", follow = false }))
bind("SUPER + TAB", "Next workspace", hl.dsp.focus({ workspace = "e+1" }))
bind("SUPER + SHIFT + TAB", "Previous workspace", hl.dsp.focus({ workspace = "e-1" }))
bind("SUPER + mouse_down", "Next workspace", hl.dsp.focus({ workspace = "e+1" }))
bind("SUPER + mouse_up", "Previous workspace", hl.dsp.focus({ workspace = "e-1" }))
bind("ALT + TAB", "Next window", hl.dsp.window.cycle_next())
bind("ALT + TAB", "Raise window", hl.dsp.window.bring_to_top())
bind("ALT + SHIFT + TAB", "Previous window", hl.dsp.window.cycle_next({ next = false }))
bind("ALT + SHIFT + TAB", "Raise window", hl.dsp.window.bring_to_top())
bind("SUPER + mouse:272", "Move window", hl.dsp.window.drag(), { mouse = true })
bind("SUPER + mouse:273", "Resize window", hl.dsp.window.resize(), { mouse = true })

bind("XF86AudioRaiseVolume", "Volume up", "wpctl set-volume -l 1 @DEFAULT_AUDIO_SINK@ 5%+", { locked = true, repeating = true })
bind("XF86AudioLowerVolume", "Volume down", "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-", { locked = true, repeating = true })
bind("XF86AudioMute", "Mute", "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle", { locked = true })
bind("XF86AudioMicMute", "Mute microphone", "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle", { locked = true })
bind("XF86MonBrightnessUp", "Brightness up", "brightnessctl -e4 -n2 set 5%+", { locked = true, repeating = true })
bind("XF86MonBrightnessDown", "Brightness down", "brightnessctl -e4 -n2 set 5%-", { locked = true, repeating = true })
bind("XF86AudioPlay", "Play or pause", "playerctl play-pause", { locked = true })
bind("XF86AudioPause", "Play or pause", "playerctl play-pause", { locked = true })
bind("XF86AudioNext", "Next track", "playerctl next", { locked = true })
bind("XF86AudioPrev", "Previous track", "playerctl previous", { locked = true })
