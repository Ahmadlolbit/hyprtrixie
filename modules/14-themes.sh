register_module 14 "Themes"

option THEME --section look --label "Theme" --type choice --default catppuccin \
  --values "catppuccin catppuccin-latte ethereal everforest flexoki-light gruvbox hackerman kanagawa last-horizon lumon lupine matte-black miasma nord osaka-jade retro-82 ristretto rose-pine solitude tokyo-night vantablack white" \
  --help "Colours, wallpaper and icons for every app; Super+Ctrl+T changes it later"

OMARCHY_DIR="${HYPRTRIXIE_OMARCHY_DIR:-/usr/share/omarchy}"
THEME_STATE=.local/state/omarchy/current
THEME_APPLIED="$STATE_DIR/theme"
DROPPED_THEMES=(bloodline neon-plum noir cobalt-night navy-mint porcelain dayshift inkwell terracotta marble green-magic roseglass)

theme_copy() {
  local src="$1" dest="$2" old="${3:-}" new="${4:-}" file base
  [[ -f $src/colors.toml ]] || return 0
  rm -rf "$dest.part"
  mkdir -p "$dest.part/backgrounds" || return 1
  for file in colors.toml icons.theme preview.png; do
    if [[ -f $src/$file ]]; then
      install -m 0644 "$src/$file" "$dest.part/$file" || return 1
    fi
  done
  for file in "$src"/backgrounds/*; do
    [[ -f $file && ! -L $file ]] || continue
    base="${file##*/}"
    [[ -n $old ]] && base="${base//"$old"/"$new"}"
    install -m 0644 "$file" "$dest.part/backgrounds/$base" || return 1
  done
  rm -rf "$dest"
  mv "$dest.part" "$dest"
}

omarchy_themes_install() {
  local dir
  source_current omarchy && [[ -f $OMARCHY_DIR/LICENSE.omarchy ]] && return 0
  fetch_git_cached omarchy /themes/ /LICENSE || return 1
  for dir in "$FETCHED_PATH"/themes/*/; do
    dir="${dir%/}"
    theme_copy "$dir" "$OMARCHY_DIR/themes/${dir##*/}" || return 1
  done
  install -D -m 0644 "$FETCHED_PATH/LICENSE" "$OMARCHY_DIR/LICENSE.omarchy" || return 1
  rm -rf "$FETCHED_PATH"
  source_mark omarchy
}

dropped_themes_remove() {
  local name
  for name in "${DROPPED_THEMES[@]}"; do
    rm -rf "${OMARCHY_DIR:?}/themes/$name"
  done
  rm -f "$OMARCHY_DIR/LICENSE.community-themes" "$STATE_DIR/sources/community-themes"
}

themes_present() {
  local name
  for name in $(option_items THEME); do
    [[ -f $OMARCHY_DIR/themes/$name/colors.toml ]] || return 1
  done
}

theme_engine_install() {
  local command
  deploy_tree "$HYPRTRIXIE_ROOT/themed" "$OMARCHY_DIR/default/themed" || return 1
  for command in omarchy-theme-set omarchy-theme-install hyprtrixie-background; do
    deploy_file "$HYPRTRIXIE_ROOT/bin/$command" "$BIN_DIR/$command" 0755 || return 1
  done
  rm -f "$BIN_DIR/hyprtrixie-theme-picker"
}

btop_theme_setup() {
  local conf
  conf="$(user_path .config/btop/btop.conf)"
  if [[ ! -f $conf ]]; then
    user_mkdir .config/btop || return 1
    printf 'color_theme = "hyprtrixie"\n' | as_user tee "$conf" >/dev/null
    return
  fi
  if grep -q '^color_theme = ' "$conf"; then
    sed -i 's|^color_theme = .*|color_theme = "hyprtrixie"|' "$conf"
  else
    printf 'color_theme = "hyprtrixie"\n' >>"$conf"
  fi
}

theme_in_use() {
  local name
  name="$(cat "$(user_path "$THEME_STATE/theme.name")" 2>/dev/null)"
  [[ -n $name && (-d $OMARCHY_DIR/themes/$name || -d "$(user_path ".config/omarchy/themes/$name")") ]]
}

theme_apply() {
  if theme_in_use && [[ "$(cat "$THEME_APPLIED" 2>/dev/null)" == "$THEME" ]]; then
    log info "keeping the theme in use; $THEME was applied before"
  else
    as_user env OMARCHY_PATH="$OMARCHY_DIR" "$BIN_DIR/omarchy-theme-set" "$THEME" || return 1
    mkdir -p "${THEME_APPLIED%/*}" && printf '%s\n' "$THEME" >"$THEME_APPLIED"
  fi
  if [[ -f "$(user_path .config/mako/config)" ]]; then
    ensure_line "$(user_path .config/mako/config)" "include=~/$THEME_STATE/theme/mako.ini"
  fi
}

theme_ready() {
  theme_in_use && [[ -f "$(user_path "$THEME_STATE/theme/colors.toml")" ]]
}

module_14() {
  run_step "Install theme packages" apt_install_list themes
  run_step "Install the Omarchy themes" omarchy_themes_install
  run_step "Remove the dropped community themes" dropped_themes_remove
  run_step "Install the theme switcher" theme_engine_install
  run_step "Point btop at the theme" btop_theme_setup
  run_step "Apply the $THEME theme" theme_apply
}

verify_14_themes() {
  run_check "All themes are installed" themes_present
}

verify_14_theme() {
  run_check "A theme is applied" theme_ready
}
