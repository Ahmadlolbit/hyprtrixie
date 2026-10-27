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
