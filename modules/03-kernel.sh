register_module 03 "Kernel and drivers"

option KERNEL --section system --label "Kernel" --type choice \
  --default backports --values "backports stable"

kernel_packages() {
  local pkg
  for pkg in "$@"; do
    [[ $KERNEL == stable ]] && pkg="${pkg%/backports}"
    printf '%s\n' "$pkg"
  done
}

kernel_install() {
  local pkgs=()
  mapfile -t pkgs < <(kernel_packages linux-image-amd64/backports)
  apt_install "${pkgs[@]}"
}

kernel_newest() {
  local image
  for image in /boot/vmlinuz-*; do
    [[ -e $image ]] && printf '%s\n' "${image#/boot/vmlinuz-}"
  done | sort -V | tail -n1
}

kernel_from_backports() {
  [[ "$(pkg_version linux-image-amd64)" == *bpo* ]]
}

boot_time() {
  printf '%s' "$(($(date +%s) - $(cut -d. -f1 /proc/uptime)))"
}

kernel_booted() {
  local newest running info
  newest="$(kernel_newest)"
  running="$(uname -r)"
  printf 'newest installed: %s, running: %s\n' "${newest:-none}" "$running"
  [[ -z $newest || $newest == "$running" ]] && return 0
  info="/var/lib/dpkg/info/linux-image-$newest.list"
  if [[ -f $info ]] && (($(stat -c %Y "$info") > $(boot_time))); then
    printf 'reboot to start %s\n' "$newest"
    return 0
  fi
  printf 'this boot did not use the newest kernel; check the default GRUB entry\n'
  return 1
}

module_03() {
  run_step "Install the $KERNEL kernel" kernel_install
}

verify_03_kernel() {
  [[ $KERNEL == backports ]] && run_check "The kernel comes from backports" kernel_from_backports
  run_check "The newest kernel is running (or starts after a reboot)" kernel_booted
}
