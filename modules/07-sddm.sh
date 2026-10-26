register_module 07 "Login screen"

SDDM_CONF="${HYPRTRIXIE_SDDM_CONF:-/etc/sddm.conf.d/10-hyprtrixie.conf}"
SDDM_STATE="${HYPRTRIXIE_SDDM_STATE:-/var/lib/sddm/state.conf}"

sddm_packages() {
  apt_install_list sddm
}

sddm_configure() {
  deploy_file "$HYPRTRIXIE_ROOT/system/sddm/x11.conf" "$SDDM_CONF" || return 1
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

hyprland_session_present() {
  [[ -f /usr/share/wayland-sessions/hyprland.desktop ]]
}

module_07() {
  run_step "Install the login screen" sddm_packages
  run_step "Configure the login screen" sddm_configure
  run_step "Start into the login screen at boot" sddm_enable
}

verify_07_sddm() {
  run_check "The login screen starts at boot" service_enabled sddm.service
  run_check "Hyprland is offered as a session" hyprland_session_present
}
