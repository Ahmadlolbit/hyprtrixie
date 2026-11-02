hl.config({
  input = {
    touchpad = {
      natural_scroll = {{ NATURAL_SCROLL }},
      tap_to_click = {{ TAP_TO_CLICK }},
    },
  },
  animations = {
    enabled = {{ ANIMATIONS }},
  },
})

hl.monitor({ output = "", mode = "preferred", position = "auto", scale = {{ SCALE }} })

require("hyprtrixie.hypr.style-{{ WINDOW_STYLE }}")

if {{ NVIDIA }} then
  hl.env("LIBVA_DRIVER_NAME", "nvidia")
  hl.env("__GLX_VENDOR_LIBRARY_NAME", "nvidia")
  hl.env("NVD_BACKEND", "direct")
end
