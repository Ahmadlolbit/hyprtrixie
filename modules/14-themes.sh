register_module 14 "Themes"

option THEME --section look --label "Theme" --type choice --default catppuccin \
  --values "catppuccin catppuccin-latte ethereal everforest flexoki-light gruvbox hackerman kanagawa last-horizon lumon lupine matte-black miasma nord osaka-jade retro-82 ristretto rose-pine solitude tokyo-night vantablack white" \
  --help "Colours, wallpaper and icons for every app; Super+Ctrl+T changes it later"
option ACCENT_COLOR --section look --label "Accent colour" --type choice --default theme \
  --values "theme red orange yellow green cyan blue magenta" \
  --help "The theme's own accent, or another of its colours, for borders and highlights"
option GLASS --section look --label "Glass mode" --type bool --default no \
  --help "See-through terminals and panels, with the wallpaper blurred behind them"
option AUTO_THEME --section look --label "Automatic light and dark" --type choice --default off \
  --values "off sun times" \
  --help "A light theme by day and a dark one at night: by the sun or at set times"
option LIGHT_THEME --section look --label "Light theme" --type choice --default catppuccin-latte \
  --values "${OPTION_VALUES[THEME]}" --when auto_theme_on \
  --help "The theme for the day"
option DARK_THEME --section look --label "Dark theme" --type choice --default catppuccin \
  --values "${OPTION_VALUES[THEME]}" --when auto_theme_on \
  --help "The theme for the night"
option LIGHT_AT --section look --label "Light from" --type choice --default 07:00 \
  --values "05:00 06:00 07:00 08:00 09:00" --when auto_theme_times \
  --help "When the light theme starts"
option DARK_AT --section look --label "Dark from" --type choice --default 19:00 \
  --values "17:00 18:00 19:00 20:00 21:00 22:00" --when auto_theme_times \
  --help "When the dark theme starts"
option WALLPAPER_MINUTES --section look --label "Change wallpaper every (minutes)" --type number --default 0 \
  --help "Move to the theme's next wallpaper every so many minutes; 0 keeps one"
option BOOT_SPLASH --section look --label "Boot splash" --type bool --default yes \
  --help "The logo and the disk password prompt in your theme's colours while it boots"
option BOOT_MENU_THEME --section look --label "Boot menu in the theme" --type bool --default yes \
  --help "The boot menu, where the snapshots are, in your theme's colours"
option CONSOLE_THEME --section look --label "Console colours and font" --type bool --default no \
  --help "The text consoles (Ctrl+Alt+F3) in your theme's colours, with a larger font"

OMARCHY_DIR="${HYPRTRIXIE_OMARCHY_DIR:-/usr/share/omarchy}"
THEME_STATE=.local/state/omarchy/current
THEME_APPLIED="$STATE_DIR/theme"
DROPPED_THEMES=(bloodline neon-plum noir cobalt-night navy-mint porcelain dayshift inkwell terracotta marble green-magic roseglass)
FOLLOW_SHARE="$SHARE_DIR/follow"
GRUB_SETTINGS=/etc/default/grub.d/hyprtrixie.cfg
GRUB_THEME_DIR="${HYPRTRIXIE_GRUB_THEME_DIR:-/boot/grub/themes/hyprtrixie}"
FOLLOW_INITRAMFS=0
LOOK_CONF="${HYPRTRIXIE_LOOK_CONF:-/etc/hyprtrixie/look.conf}"
LOOK_CHANGED=0

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
  for command in omarchy-theme-set omarchy-theme-install hyprtrixie-background hyprtrixie-look; do
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

look_install() {
  deploy_rendered "$HYPRTRIXIE_ROOT/system/look.conf" "$LOOK_CONF" 0644 root "ACCENT=$ACCENT_COLOR" "GLASS=$GLASS" \
    "AUTO=$AUTO_THEME" "LIGHT=$LIGHT_THEME" "DARK=$DARK_THEME" "LIGHT_AT=$LIGHT_AT" "DARK_AT=$DARK_AT" "ROTATE=$WALLPAPER_MINUTES" || return 1
  ((DEPLOY_CHANGED)) || return 0
  LOOK_CHANGED=1
  rm -f "$(user_path .local/state/hyprtrixie/look/period)"
}

auto_theme_on() {
  [[ $AUTO_THEME != off ]]
}

auto_theme_times() {
  [[ $AUTO_THEME == times ]]
}

theme_apply() {
  if theme_in_use && [[ "$(cat "$THEME_APPLIED" 2>/dev/null)" == "$THEME" ]]; then
    log info "keeping the theme in use; $THEME was applied before"
    if ((LOOK_CHANGED)); then
      as_user env OMARCHY_PATH="$OMARCHY_DIR" HYPRTRIXIE_LOOK_SYSTEM="$LOOK_CONF" "$BIN_DIR/omarchy-theme-set" \
        "$(cat "$(user_path "$THEME_STATE/theme.name")")" || return 1
    fi
  else
    as_user env OMARCHY_PATH="$OMARCHY_DIR" HYPRTRIXIE_LOOK_SYSTEM="$LOOK_CONF" "$BIN_DIR/omarchy-theme-set" "$THEME" || return 1
    mkdir -p "${THEME_APPLIED%/*}" && printf '%s\n' "$THEME" >"$THEME_APPLIED"
  fi
  if [[ -f "$(user_path .config/mako/config)" ]]; then
    ensure_line "$(user_path .config/mako/config)" "include=~/$THEME_STATE/theme/mako.ini"
  fi
}

theme_ready() {
  theme_in_use && [[ -f "$(user_path "$THEME_STATE/theme/colors.toml")" ]]
}

follow_install() {
  deploy_file "$HYPRTRIXIE_ROOT/bin/hyprtrixie-follow-theme" "$BIN_DIR/hyprtrixie-follow-theme" 0755 || return 1
  deploy_file "$HYPRTRIXIE_ROOT/system/plymouth/hyprtrixie.script" "$FOLLOW_SHARE/hyprtrixie.script" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/system/grub/theme.txt" "$FOLLOW_SHARE/theme.txt" || return 1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/follow-theme.conf" /etc/hyprtrixie/follow-theme 0644 root \
    "USER=$TARGET_USER" "SPLASH=$BOOT_SPLASH" "MENU=$BOOT_MENU_THEME" "CONSOLE=$CONSOLE_THEME" "CLOCK=$CLOCK_FORMAT"
}

splash_file() {
  deploy_file "$HYPRTRIXIE_ROOT/system/$1" "$2" || return 1
  ((DEPLOY_CHANGED)) && FOLLOW_INITRAMFS=1
  return 0
}

boot_splash_install() {
  [[ $BOOT_SPLASH == yes ]] || return 0
  apt_install_list boot-splash || return 1
  splash_file plymouth/hyprtrixie.plymouth /usr/share/plymouth/themes/hyprtrixie/hyprtrixie.plymouth || return 1
  splash_file plymouth/plymouthd.conf /etc/plymouth/plymouthd.conf || return 1
  splash_file initramfs/splash.conf /etc/initramfs-tools/conf.d/hyprtrixie-splash
}

grub_theme_install() {
  local size
  [[ $BOOT_MENU_THEME == yes ]] || return 0
  apt_install fonts-dejavu-core || return 1
  mkdir -p "$GRUB_THEME_DIR" || return 1
  for size in 16 20 28; do
    [[ -f $GRUB_THEME_DIR/dejavu-$size.pf2 ]] && continue
    grub-mkfont -s "$size" -o "$GRUB_THEME_DIR/dejavu-$size.pf2" /usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf || return 1
  done
}

grub_settings() {
  local tmp rc
  tmp="$(mktemp)" || return 1
  [[ $BOOT_SPLASH == yes ]] && cat "$HYPRTRIXIE_ROOT/system/grub/splash.cfg" >>"$tmp"
  [[ $BOOT_MENU_THEME == yes ]] && cat "$HYPRTRIXIE_ROOT/system/grub/theme.cfg" >>"$tmp"
  cat "$HYPRTRIXIE_ROOT/system/grub/hidden.cfg" >>"$tmp"
  deploy_file "$tmp" "$GRUB_SETTINGS"
  rc=$?
  rm -f "$tmp"
  ((rc == 0)) || return 1
  if ((DEPLOY_CHANGED)) && command -v update-grub >/dev/null 2>&1; then
    update-grub
  fi
}

console_theme_install() {
  if [[ $CONSOLE_THEME != yes ]]; then
    service_disable hyprtrixie-console.service
    return
  fi
  apt_install kbd console-setup || return 1
  conf_set /etc/default/console-setup FONTFACE '"Terminus"' || return 1
  conf_set /etc/default/console-setup FONTSIZE '"16x32"' || return 1
  deploy_file "$HYPRTRIXIE_ROOT/system/systemd/hyprtrixie-console.service" \
    /etc/systemd/system/hyprtrixie-console.service || return 1
  systemctl daemon-reload || return 1
  service_enable hyprtrixie-console.service
}

follow_run() {
  if ((FOLLOW_INITRAMFS)); then
    "$BIN_DIR/hyprtrixie-follow-theme" --initramfs
  else
    "$BIN_DIR/hyprtrixie-follow-theme"
  fi
}

follow_watch() {
  deploy_file "$HYPRTRIXIE_ROOT/system/systemd/hyprtrixie-follow-theme.service" \
    /etc/systemd/system/hyprtrixie-follow-theme.service || return 1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/systemd/hyprtrixie-follow-theme.path" \
    /etc/systemd/system/hyprtrixie-follow-theme.path 0644 root "HOME=$TARGET_HOME" || return 1
  systemctl daemon-reload || return 1
  service_start hyprtrixie-follow-theme.path
}

module_14() {
  run_step "Install theme packages" apt_install_list themes
  run_step "Install the Omarchy themes" omarchy_themes_install
  run_step "Remove the dropped community themes" dropped_themes_remove
  run_step "Install the theme switcher" theme_engine_install
  run_step "Point btop at the theme" btop_theme_setup
  run_step "Write the look settings" look_install
  run_step "Apply the $THEME theme" theme_apply
  run_step "Install the theme follower" follow_install
  run_step "Show a boot splash in the theme's colours" boot_splash_install
  run_step "Theme the boot menu" grub_theme_install
  run_step "Set up the boot menu" grub_settings
  run_step "Theme the text consoles" console_theme_install
  run_step "Paint the boot and login screens in the theme's colours" follow_run
  run_step "Follow theme switches on the boot and login screens" follow_watch
}

verify_14_themes() {
  run_check "All themes are installed" themes_present
}

verify_14_theme() {
  run_check "A theme is applied" theme_ready
}

verify_14_look() {
  run_check "The look settings are written" test -f "$LOOK_CONF"
}
verify_14_splash() {
  [[ $BOOT_SPLASH == yes ]] || return 0
  run_check "The boot splash is in the theme's colours" test -f /usr/share/plymouth/themes/hyprtrixie/hyprtrixie.script
}

verify_14_boot_menu() {
  [[ $BOOT_MENU_THEME == yes ]] || return 0
  run_check "The boot menu is in the theme's colours" test -f "$GRUB_THEME_DIR/theme.txt"
}

verify_14_follow() {
  run_check "Theme switches reach the boot screens" service_enabled hyprtrixie-follow-theme.path
}
