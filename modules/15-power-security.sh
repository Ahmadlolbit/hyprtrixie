register_module 15 "Power and security"

power_install() {
  local lists=(power)
  [[ $CPU_VENDOR == intel ]] && lists+=(power-intel)
  apt_install_list "${lists[@]}" || return 1
  systemctl enable power-profiles-daemon.service
}

power_ready() {
  list_present power && systemctl is-enabled --quiet power-profiles-daemon.service
}

module_15() {
  run_step "Set up power profiles" power_install
  return 0
}

verify_15_power() {
  run_check "Power profiles are available" power_ready
}
