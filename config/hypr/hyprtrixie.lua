local state = os.getenv("XDG_STATE_HOME") or ((os.getenv("HOME") or "") .. "/.local/state")
local roots = "/etc/?.lua;/usr/share/?.lua;" .. state .. "/?.lua;"
if not package.path:find(roots, 1, true) then
  package.path = roots .. package.path
end

require("hyprtrixie.hypr.envs")
require("hyprtrixie.hypr.looknfeel")
require("hyprtrixie.hypr.input")
require("hyprtrixie.hypr.windows")
require("hyprtrixie.hypr.autostart")
require("hyprtrixie.hypr.bindings")

if package.searchpath("omarchy.current.theme.hyprland", package.path) then
  require("omarchy.current.theme.hyprland")
end

if package.searchpath("hyprtrixie.hypr.options", package.path) then
  require("hyprtrixie.hypr.options")
end
