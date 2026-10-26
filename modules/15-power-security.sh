register_module 15 "Power and security"

option FIREWALL --section security --label "Firewall that blocks incoming connections" --type bool --default yes

power_install() {
  local lists=(power)
  [[ $CPU_VENDOR == intel ]] && lists+=(power-intel)
  apt_install_list "${lists[@]}" || return 1
  systemctl enable power-profiles-daemon.service
}

power_ready() {
  list_present power && systemctl is-enabled --quiet power-profiles-daemon.service
}

firewall_setup() {
  ufw default deny incoming >/dev/null || return 1
  ufw default allow outgoing >/dev/null || return 1
  ufw --force enable
}

firewall_ready() {
  ufw status | grep -q '^Status: active'
}

module_15() {
  run_step "Set up power profiles" power_install
  if [[ $FIREWALL == yes ]]; then
    run_step "Turn on the firewall" firewall_setup
  fi
  return 0
}

verify_15_power() {
  run_check "Power profiles are available" power_ready
}

verify_15_firewall() {
  [[ $FIREWALL == yes ]] || return 0
  run_check "The firewall is on" firewall_ready
}
