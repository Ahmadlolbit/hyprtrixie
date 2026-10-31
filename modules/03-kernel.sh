register_module 03 "Kernel and drivers"

option KERNEL --section system --label "Kernel" --type choice \
  --default backports --values "backports stable" \
  --help "Backports has a newer kernel for recent laptops; stable is Debian's own"
option GRAPHICS --section system --label "NVIDIA driver" --type choice \
  --default proprietary --values "proprietary nouveau" --when nvidia_present \
  --help "NVIDIA's own driver is faster; nouveau is open source and slower"

NVIDIA_INITRAMFS=0

nvidia_present() {
  has_gpu nvidia
}

nvidia_wanted() {
  nvidia_present && [[ $GRAPHICS == proprietary ]]
}

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

nvidia_list() {
  if ((NVIDIA_OPEN)); then
    printf 'nvidia-open'
  else
    printf 'nvidia-closed'
  fi
}

nvidia_install() {
  local pkgs=() listed=()
  mapfile -t listed < <(pkg_list "$(nvidia_list)")
  mapfile -t pkgs < <(kernel_packages "${listed[@]}")
  apt_install "${pkgs[@]}"
}

nvidia_configure() {
  local power=""
  ((GPU_HYBRID)) && power=" NVreg_DynamicPowerManagement=0x02"
  deploy_rendered "$HYPRTRIXIE_ROOT/system/modprobe/nvidia.conf" /etc/modprobe.d/hyprtrixie-nvidia.conf 0644 root \
    "POWER=$power" || return 1
  ((DEPLOY_CHANGED)) && NVIDIA_INITRAMFS=1
  service_enable nvidia-suspend.service nvidia-resume.service nvidia-hibernate.service || return 1
  ((NVIDIA_INITRAMFS)) || return 0
  update-initramfs -u -k all || return 1
  NVIDIA_INITRAMFS=0
}

nvidia_built() {
  modinfo -k "$(kernel_newest)" nvidia >/dev/null 2>&1
}

module_03() {
  run_step "Install the $KERNEL kernel" kernel_install
  if nvidia_wanted; then
    run_step "Install the NVIDIA driver" nvidia_install
    run_step "Configure the NVIDIA driver" nvidia_configure
  fi
}

verify_03_kernel() {
  [[ $KERNEL == backports ]] && run_check "The kernel comes from backports" kernel_from_backports
  run_check "The newest kernel is running (or starts after a reboot)" kernel_booted
}

verify_03_nvidia() {
  nvidia_wanted || return 0
  run_check "The NVIDIA module is built for the newest kernel" nvidia_built
}
