register_module 01 "System preflight"

MIN_FREE_GB=30

os_release_get() {
  awk -F= -v key="$1" '$1 == key { gsub(/"/, "", $2); print $2 }' /etc/os-release 2>/dev/null
}

preflight_os() {
  local id codename
  id="$(os_release_get ID)"
  codename="$(os_release_get VERSION_CODENAME)"
  if [[ $id != debian || $codename != trixie ]]; then
    printf 'This system is %s %s; hyprtrixie is made for Debian 13 (trixie).\n' "${id:-unknown}" "${codename:-unknown}"
    return 1
  fi
}

preflight_arch() {
  local arch
  arch="$(dpkg --print-architecture)"
  if [[ $arch != amd64 ]]; then
    printf 'This system is %s; hyprtrixie supports amd64 only.\n' "$arch"
    return 1
  fi
}

preflight_secure_boot() {
  local state
  state="$(secure_boot_state)"
  printf 'Secure Boot: %s\n' "$state"
  if [[ $state == enabled ]]; then
    printf 'Turn Secure Boot off in the firmware settings: drivers built on this machine are not signed.\n'
    return 1
  fi
}

preflight_network() {
  curl -fsS --max-time 20 -o /dev/null https://deb.debian.org/debian/dists/trixie/InRelease
}

preflight_disk() {
  local free
  free="$(df --output=avail -BG / | tail -n1 | tr -dc '0-9')"
  printf '%s GiB free on /\n' "$free"
  if ((free < MIN_FREE_GB)); then
    printf 'At least %s GiB of free space is needed.\n' "$MIN_FREE_GB"
    return 1
  fi
}

preflight_choose_user() {
  local users=() pick name
  mapfile -t users < <(human_users)
  pick="Create a new account"
  if ((${#users[@]})); then
    pick="$(ui_choose "Which account should this computer be set up for?" "" "${users[@]}" "Create a new account")" || return 1
  fi
  if [[ $pick == "Create a new account" ]]; then
    name="$(ui_input "Name for the new account (lowercase, no spaces)" "")" || return 1
    if [[ ! $name =~ ^[a-z_][a-z0-9_-]{0,31}$ ]]; then
      printf 'Account names use lowercase letters, digits, - and _.\n'
      return 1
    fi
    adduser --disabled-password --comment "" "$name" || return 1
    pick="$name"
  fi
  set_target_user "$pick"
}

preflight_password() {
  local first second
  while :; do
    first="$(ui_password "Choose a password for $TARGET_USER")" || return 1
    second="$(ui_password "Type it once more")" || return 1
    [[ -n $first && $first == "$second" ]] && break
    ui_status warn "Those didn't match, try again"
  done
  printf '%s:%s\n' "$TARGET_USER" "$first" | chpasswd
}

preflight_user() {
  if [[ -z $TARGET_USER ]]; then
    if ! ui_can_ask; then
      printf 'No admin account found. Run hyprtrixie with sudo from your own account.\n'
      return 1
    fi
    preflight_choose_user || return 1
    log_attach_user "$TARGET_HOME" "$TARGET_USER"
  fi
  apt_install sudo || return 1
  user_add_groups sudo || return 1
  if ! user_has_password; then
    if ! ui_can_ask; then
      printf '%s has no password yet; set one with: passwd %s\n' "$TARGET_USER" "$TARGET_USER"
      return 1
    fi
    preflight_password || return 1
  fi
  printf 'Admin account: %s (%s)\n' "$TARGET_USER" "$TARGET_HOME"
}

user_is_admin() {
  [[ -n $TARGET_USER ]] && sudo -l -U "$TARGET_USER" 2>/dev/null | grep -qE '\(ALL( : ALL)?\) ALL'
}

preflight_detect() {
  detect_all
  detect_summary
}

module_01() {
  run_step "Debian 13 (trixie)" preflight_os
  run_step "64-bit PC (amd64)" preflight_arch
  run_step "Secure Boot is off" preflight_secure_boot
  run_step "Internet connection" preflight_network
  run_step "Enough free disk space" preflight_disk
  run_step --interactive "Admin account" preflight_user
  run_step "Detect hardware" preflight_detect
}

verify_01_os() {
  run_check "Running on Debian 13 (trixie)" preflight_os
}

verify_01_admin() {
  run_check "${TARGET_USER:-The admin account} can use sudo" user_is_admin
}
