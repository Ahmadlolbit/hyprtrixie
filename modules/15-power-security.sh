register_module 15 "Power and security"

option FIREWALL --section security --label "Firewall" --type bool --default yes \
  --help "Blocks connections from other computers; your own connections still work"
option AUTO_UPDATES --section security --label "Automatic security updates" --type bool --default yes \
  --help "Debian's security fixes install on their own in the background"
option SSH_POLICY --section security --label "SSH" --type choice --default client --values "client leave" \
  --help "Client: the SSH server doesn't start at boot. Leave: keep it as it is"
option NM_HANDOVER --section security --label "NetworkManager for everything" --type bool --default yes \
  --help "Wi-Fi and wired connections move to NetworkManager, so the bar manages them"

INTERFACES=/etc/network/interfaces

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

wifi_from_interfaces() {
  awk '$1 == "iface" { iface = $2 } $1 == "wpa-ssid" { sub(/^[[:space:]]*wpa-ssid[[:space:]]+/, ""); gsub(/^"|"$/, ""); ssid[iface] = $0 }
    $1 == "wpa-psk" { psk[iface] = $2 } END { for (i in ssid) printf "%s\t%s\t%s\n", i, ssid[i], psk[i] }' "$INTERFACES"
}

interfaces_beyond_loopback() {
  awk '$1 == "iface" && $2 != "lo" { found = 1 } END { exit !found }' "$INTERFACES" 2>/dev/null
}

nm_handover() {
  local iface ssid psk
  [[ -f $INTERFACES ]] || return 0
  interfaces_beyond_loopback || return 0
  systemctl is-active --quiet NetworkManager || systemctl start NetworkManager || return 1
  while IFS=$'\t' read -r iface ssid psk; do
    [[ -n $ssid ]] || continue
    nmcli -t -e no -f NAME connection show | grep -xF -- "$ssid" >/dev/null && continue
    if [[ -n $psk ]]; then
      nmcli connection add type wifi ifname "$iface" con-name "$ssid" ssid "$ssid" wifi-sec.key-mgmt wpa-psk wifi-sec.psk "$psk" >/dev/null
    else
      nmcli connection add type wifi ifname "$iface" con-name "$ssid" ssid "$ssid" >/dev/null
    fi || return 1
  done < <(wifi_from_interfaces)
  [[ -e $INTERFACES.hyprtrixie-old ]] || cp -a "$INTERFACES" "$INTERFACES.hyprtrixie-old" || return 1
  printf 'source /etc/network/interfaces.d/*\n\nauto lo\niface lo inet loopback\n' >"$INTERFACES"
  log info "NetworkManager takes over the network connections from the next boot"
}

nm_ready() {
  ! interfaces_beyond_loopback
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
  if [[ $NM_HANDOVER == yes ]]; then
    run_step "Hand the network over to NetworkManager" nm_handover
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

verify_15_network() {
  [[ $NM_HANDOVER == yes ]] || return 0
  run_check "NetworkManager manages every connection" nm_ready
}
