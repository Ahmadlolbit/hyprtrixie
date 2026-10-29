hl.config({
  decoration = {
    blur = {
      enabled = true,
      size = 6,
      passes = 2,
      vibrancy = 0.17,
      popups = true,
    },
  },
})

hl.layer_rule({
  name = "glass-panels",
  match = { namespace = "^(quickshell|hyprtrixie-(menu|keys|welcome|osd)|walker|notifications)$" },
  blur = true,
  ignore_alpha = 0.3,
})
