hl.config({
  general = {
    col = {
      active_border = "rgb({{ accent_strip }})",
      inactive_border = "rgba({{ muted_strip }}aa)",
    },
  },
  group = {
    col = {
      border_active = "rgb({{ accent_strip }})",
      border_inactive = "rgba({{ muted_strip }}aa)",
    },
    groupbar = {
      text_color = "rgb({{ foreground_strip }})",
      col = {
        active = "rgba({{ accent_strip }}55)",
        inactive = "rgba({{ lighter_background_strip }}55)",
      },
    },
  },
  misc = {
    background_color = "rgb({{ background_strip }})",
  },
})
