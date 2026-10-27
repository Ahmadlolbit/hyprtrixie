@define-color window_bg_color {{ background }};
@define-color accent_bg_color {{ accent }};
@define-color theme_fg_color {{ foreground }};
@define-color error_bg_color {{ red }};
@define-color error_fg_color {{ background }};

* {
  all: unset;
  font-family: "JetBrainsMono Nerd Font";
}

scrollbar {
  opacity: 0;
}

.normal-icons {
  -gtk-icon-size: 16px;
}

.large-icons {
  -gtk-icon-size: 32px;
}

.box-wrapper {
  background: @window_bg_color;
  border: 2px solid @accent_bg_color;
  padding: 16px;
  min-width: 640px;
}

.input {
  background: {{ lighter_background }};
  color: @theme_fg_color;
  caret-color: @accent_bg_color;
  padding: 12px;
  margin-bottom: 8px;
}

.input placeholder {
  opacity: 0.5;
}

.list,
.placeholder,
.elephant-hint,
.preview-box {
  color: @theme_fg_color;
}

.item-box {
  padding: 8px 10px;
}

child:selected .item-box,
row:selected .item-box {
  background: alpha(@accent_bg_color, 0.2);
}

.item-subtext {
  font-size: 12px;
  opacity: 0.6;
}

.item-quick-activation {
  background: alpha(@accent_bg_color, 0.2);
  padding: 4px 8px;
}

.error {
  background: @error_bg_color;
  color: @error_fg_color;
  padding: 8px;
}
