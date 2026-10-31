register_module 02 "Repositories and base packages"

option DEBIAN_MIRROR --section system --label "Debian mirror" --type text \
  --default "https://deb.debian.org/debian" \
  --help "Where Debian packages download from; the default picks a nearby server"

APT_SOURCES_DIR="${HYPRTRIXIE_SOURCES_DIR:-/etc/apt/sources.list.d}"
APT_SOURCES_LIST="${HYPRTRIXIE_SOURCES_LIST:-/etc/apt/sources.list}"
DEBIAN_SECURITY_MIRROR="https://security.debian.org/debian-security"

sources_legacy_cleanup() {
  local list="$APT_SOURCES_LIST" tmp
  [[ -s $list ]] || return 0
  grep -qE '^[[:space:]]*deb' "$list" || return 0
  [[ -e $list.hyprtrixie-old ]] || cp -a "$list" "$list.hyprtrixie-old" || return 1
  tmp="$(mktemp)" || return 1
  awk '/^[[:space:]]*deb(-src)?[[:space:]]/ && ($0 ~ /debian\.org/ || $0 ~ /cdrom:/) { next } { print }' "$list" >"$tmp"
  cat "$tmp" >"$list"
  rm -f "$tmp"
  log info "moved the Debian entries out of $list (backup in $list.hyprtrixie-old)"
}

SOURCES_CHANGED=0

sources_write() {
  apt_install ca-certificates || return 1
  sources_legacy_cleanup || return 1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/apt/debian.sources" "$APT_SOURCES_DIR/debian.sources" 0644 root \
    "MIRROR=$DEBIAN_MIRROR" "SECURITY=$DEBIAN_SECURITY_MIRROR" || return 1
  ((DEPLOY_CHANGED)) && SOURCES_CHANGED=1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/apt/debian-backports.sources" "$APT_SOURCES_DIR/debian-backports.sources" 0644 root \
    "MIRROR=$DEBIAN_MIRROR" || return 1
  ((DEPLOY_CHANGED)) && SOURCES_CHANGED=1
  if ((SOURCES_CHANGED)); then
    apt_update --force || return 1
    SOURCES_CHANGED=0
  else
    apt_update
  fi
}

extrepo_setup() {
  apt_install extrepo || return 1
  extrepo_policy main && extrepo_policy contrib && extrepo_policy non-free
}

extrepo_ready() {
  pkg_installed extrepo && grep -qE '^- non-free$' "$EXTREPO_CONFIG"
}

firmware_lists() {
  local vendor lists=(firmware)
  case "$CPU_VENDOR" in
    intel) lists+=(cpu-intel audio-sof) ;;
    amd) lists+=(cpu-amd) ;;
  esac
  for vendor in intel amd nvidia; do
    has_gpu "$vendor" && lists+=("gpu-$vendor")
  done
  for vendor in intel realtek mediatek atheros broadcom; do
    has_wifi "$vendor" && lists+=("wifi-$vendor")
  done
  printf '%s\n' "${lists[@]}"
}

firmware_packages() {
  local lists=() list pkg
  mapfile -t lists < <(firmware_lists)
  for list in "${lists[@]}"; do
    while IFS= read -r pkg; do
      [[ ${KERNEL:-backports} == stable ]] && pkg="${pkg%/backports}"
      printf '%s\n' "$pkg"
    done < <(pkg_list "$list")
  done
}

firmware_install() {
  local pkgs=()
  mapfile -t pkgs < <(firmware_packages)
  printf 'firmware for: %s\n' "$(firmware_lists | tr '\n' ' ')"
  apt_install "${pkgs[@]}"
}

firmware_present() {
  local pkgs=()
  mapfile -t pkgs < <(firmware_packages)
  packages_present "${pkgs[@]}"
}

apt_has_suite() {
  apt-cache policy 2>/dev/null | grep -q "n=$1,"
}

apt_has_component() {
  apt-cache policy 2>/dev/null | grep "n=trixie," | grep -q "c=$1,"
}

module_02() {
  run_step "Use Debian over HTTPS with backports" sources_write
  run_step "Prepare vendor repositories" extrepo_setup
  run_step "Install base packages" apt_install_list base
  run_step "Install firmware for this machine" firmware_install
}

verify_02_sources() {
  run_check "Backports are available" apt_has_suite trixie-backports
  run_check "Firmware and non-free components are on" apt_has_component non-free-firmware
}

verify_02_extrepo() {
  run_check "Vendor repositories can be enabled" extrepo_ready
}

verify_02_base() {
  run_check "Base packages are installed" list_present base
}

verify_02_firmware() {
  run_check "Firmware for this machine is installed" firmware_present
}
