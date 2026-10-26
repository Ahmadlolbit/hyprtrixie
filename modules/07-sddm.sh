register_module 07 "Login screen"

SDDM_CONF="${HYPRTRIXIE_SDDM_CONF:-/etc/sddm.conf.d/10-hyprtrixie.conf}"
SDDM_STATE="${HYPRTRIXIE_SDDM_STATE:-/var/lib/sddm/state.conf}"
SDDM_THEMES="${HYPRTRIXIE_SDDM_THEMES:-/usr/share/sddm/themes}"
SDDM_THEME="catppuccin-mocha-blue"

sddm_packages() {
  apt_install_list sddm
}

sddm_configure() {
  deploy_rendered "$HYPRTRIXIE_ROOT/system/sddm/x11.conf" "$SDDM_CONF" 0644 root "THEME=$SDDM_THEME" || return 1
  if [[ -n $TARGET_USER ]] && ! grep -q '^Session=' "$SDDM_STATE" 2>/dev/null; then
    mkdir -p "${SDDM_STATE%/*}" || return 1
    render_template "$HYPRTRIXIE_ROOT/system/sddm/state.conf" "USER=$TARGET_USER" >"$SDDM_STATE" || return 1
    chown sddm:sddm "$SDDM_STATE" 2>/dev/null || true
  fi
  printf '/usr/bin/sddm\n' >/etc/X11/default-display-manager
}

sddm_enable() {
  service_enable sddm.service || return 1
  [[ "$(systemctl get-default)" == graphical.target ]] || systemctl set-default graphical.target
}

sddm_theme_install() {
  local dir
  apt_install_list sddm-theme || return 1
  release_current catppuccin-sddm && [[ -f $SDDM_THEMES/$SDDM_THEME/Main.qml ]] && return 0
  fetch_release catppuccin-sddm || return 1
  dir="$(mktemp -d)" || return 1
  unzip -q "$FETCHED_PATH" -d "$dir" || return 1
  rm -rf "${SDDM_THEMES:?}/$SDDM_THEME"
  mkdir -p "$SDDM_THEMES" && mv "$dir/$SDDM_THEME" "$SDDM_THEMES/" || return 1
  rm -rf "$dir"
  release_mark catppuccin-sddm
}

sddm_theme_ready() {
  [[ -f $SDDM_THEMES/$SDDM_THEME/Main.qml ]] && grep -q "^Current=$SDDM_THEME" "$SDDM_CONF"
}

hyprland_session_present() {
  [[ -f /usr/share/wayland-sessions/hyprland.desktop ]]
}

module_07() {
  run_step "Install the login screen" sddm_packages
  run_step "Install the Catppuccin Mocha theme" sddm_theme_install
  run_step "Configure the login screen" sddm_configure
  run_step "Start into the login screen at boot" sddm_enable
}

verify_07_sddm() {
  run_check "The login screen starts at boot" service_enabled sddm.service
  run_check "Hyprland is offered as a session" hyprland_session_present
  run_check "The login screen uses the Catppuccin theme" sddm_theme_ready
}
