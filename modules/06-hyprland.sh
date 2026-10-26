register_module 06 "Hyprland desktop"

hyprland_recent() {
  dpkg --compare-versions "$(pkg_version hyprland)" ge 0.55
}

quickshell_works() {
  qs --version >/dev/null 2>&1
}

module_06() {
  run_step "Install desktop services" apt_install_list desktop
  run_step "Install Hyprland from backports" apt_install_list hyprland
}

verify_06_hyprland() {
  run_check "Hyprland 0.55 or newer is installed" hyprland_recent
  run_check "Quickshell runs" quickshell_works
}

verify_06_desktop() {
  run_check "Desktop services are installed" list_present desktop
}
