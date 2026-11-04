local look = require("hyprtrixie.hypr.look")

hl.env("XCURSOR_THEME", "Bibata-Modern-Ice")
hl.env("XCURSOR_SIZE", "24")
hl.env("HYPRCURSOR_SIZE", "24")
hl.env("GDK_BACKEND", "wayland,x11,*")
hl.env("QT_QPA_PLATFORM", "wayland;xcb")
hl.env("QT_QPA_PLATFORMTHEME", "kde")
hl.env("KDE_SESSION_VERSION", "6")
hl.env("QT_WAYLAND_DISABLE_WINDOWDECORATION", "1")
hl.env("MOZ_ENABLE_WAYLAND", "1")
hl.env("ELECTRON_OZONE_PLATFORM_HINT", "wayland")
hl.env("OZONE_PLATFORM", "wayland")
hl.env("XDG_SESSION_TYPE", "wayland")
hl.env("XDG_CURRENT_DESKTOP", "Hyprland")
hl.env("XDG_SESSION_DESKTOP", "Hyprland")
hl.env("XDG_MENU_PREFIX", "hyprtrixie-")
if look.get("reader") == "yes" then
  hl.env("QT_ACCESSIBILITY", "1")
  hl.env("QT_LINUX_ACCESSIBILITY_ALWAYS_ON", "1")
end
local data_dirs, seen = {}, {}
local function data_dir(dir)
  if dir ~= "" and not seen[dir] then
    seen[dir] = true
    table.insert(data_dirs, dir)
  end
end
data_dir((os.getenv("HOME") or "") .. "/.local/share/flatpak/exports/share")
data_dir("/var/lib/flatpak/exports/share")
for dir in string.gmatch(os.getenv("XDG_DATA_DIRS") or "/usr/local/share:/usr/share", "[^:]+") do
  data_dir(dir)
end
data_dir("/var/lib/snapd/desktop")
hl.env("XDG_DATA_DIRS", table.concat(data_dirs, ":"))

hl.config({
  xwayland = {
    force_zero_scaling = true,
  },
  ecosystem = {
    no_update_news = true,
    no_donation_nag = true,
  },
})
