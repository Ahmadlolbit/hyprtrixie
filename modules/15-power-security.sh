register_module 15 "Power and security"

option FIREWALL --section security --label "Firewall that blocks incoming connections" --type bool --default yes
option AUTO_UPDATES --section security --label "Install security updates automatically" --type bool --default yes
option SSH_POLICY --section security --label "SSH" --type choice --default client --values "client leave"

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

auto_updates_setup() {
  local enabled=0
  [[ $AUTO_UPDATES == yes ]] && enabled=1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/apt/52hyprtrixie-auto-upgrades" /etc/apt/apt.conf.d/52hyprtrixie-auto-upgrades 0644 root \
    "ENABLED=$enabled"
}

auto_updates_ready() {
  pkg_installed unattended-upgrades && apt-config dump APT::Periodic::Unattended-Upgrade | grep '"1";' >/dev/null
}

ssh_server_off() {
  local unit
  pkg_installed openssh-server || return 0
  for unit in ssh.socket ssh.service; do
    systemctl disable "$unit" >/dev/null 2>&1 || true
  done
  log info "the SSH server stays off from the next boot"
}

ssh_server_disabled() {
  ! systemctl is-enabled --quiet ssh.service 2>/dev/null && ! systemctl is-enabled --quiet ssh.socket 2>/dev/null
}

module_15() {
  run_step "Set up power profiles" power_install
  if [[ $FIREWALL == yes ]]; then
    run_step "Turn on the firewall" firewall_setup
  fi
  run_step "Set up automatic security updates" auto_updates_setup
  if [[ $SSH_POLICY == client ]]; then
    run_step "Keep the SSH server off" ssh_server_off
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

verify_15_updates() {
  [[ $AUTO_UPDATES == yes ]] || return 0
  run_check "Security updates install automatically" auto_updates_ready
}

verify_15_ssh() {
  [[ $SSH_POLICY == client ]] || return 0
  run_check "The SSH server is off" ssh_server_disabled
}
