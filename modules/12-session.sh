register_module 12 "Desktop session"

HYPR_SHARE="$SHARE_DIR/hypr"

hyprland_defaults() {
  deploy_tree "$HYPRTRIXIE_ROOT/config/hypr" "$HYPR_SHARE" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/system/hypr/hyprland.lua" /etc/xdg/hypr/hyprland.lua
}

hyprland_config_ok() {
  local runtime out
  runtime="$(mktemp -d)" || return 1
  chown "$TARGET_USER:" "$runtime" || return 1
  out="$(as_user env XDG_RUNTIME_DIR="$runtime" Hyprland --verify-config 2>&1)"
  rm -rf "$runtime"
  if [[ "$(awk '/Config parsing result:/ { found = 1; next } found && NF { print; exit }' <<<"$out")" == "config ok" ]]; then
    return 0
  fi
  sed -n '/Config parsing result:/,$p' <<<"$out" | head -n 20
  return 1
}

module_12() {
  run_step "Install session packages" apt_install_list session
  run_step "Install the Hyprland defaults" hyprland_defaults
}

verify_12_packages() {
  run_check "Session packages are installed" list_present session
}

verify_12_hyprland() {
  run_check "Hyprland accepts the configuration" hyprland_config_ok
}
