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
  run_step "Detect hardware" preflight_detect
}

verify_01_os() {
  run_check "Running on Debian 13 (trixie)" preflight_os
}
