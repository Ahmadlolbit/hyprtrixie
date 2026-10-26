register_module 04 "Swap and memory"

option SWAPFILE_GB --section storage --label "Swapfile size in GiB (0 = none)" --type number --default 8
option ZRAM_PERCENT --section storage --label "zram size, % of memory (0 = off)" --type number --default 50
option SWAPPINESS --section storage --label "Swappiness" --type number --default 180

SWAP_DIR="${HYPRTRIXIE_SWAP_DIR:-/swap}"
SWAPFILE="$SWAP_DIR/swapfile"
ZRAM_CONFIG="${HYPRTRIXIE_ZRAM_CONFIG:-/etc/default/zramswap}"
SYSCTL_SWAP="${HYPRTRIXIE_SYSCTL_DIR:-/etc/sysctl.d}/90-hyprtrixie-swap.conf"

swap_dir_prepare() {
  if root_is_btrfs; then
    mount_subvolume @swap "$SWAP_DIR" noatime || return 1
  else
    mkdir -p "$SWAP_DIR" || return 1
  fi
  chmod 0700 "$SWAP_DIR"
}

swap_active() {
  swapon --noheadings --show=NAME 2>/dev/null | grep -qx "$1"
}

swapfile_size_ok() {
  [[ -f $SWAPFILE ]] && (($(stat -c %s "$SWAPFILE") == SWAPFILE_GB * 1024 * 1024 * 1024))
}

swapfile_create() {
  swapfile_size_ok && return 0
  if swap_active "$SWAPFILE"; then
    swapoff "$SWAPFILE" || return 1
  fi
  rm -f "$SWAPFILE"
  if root_is_btrfs; then
    btrfs filesystem mkswapfile --size "${SWAPFILE_GB}g" "$SWAPFILE"
  else
    fallocate -l "${SWAPFILE_GB}G" "$SWAPFILE" && chmod 0600 "$SWAPFILE" && mkswap "$SWAPFILE"
  fi
}

swapfile_enable() {
  fstab_ensure "$SWAPFILE" none swap "defaults,pri=10" || return 1
  swap_active "$SWAPFILE" || swapon --priority 10 "$SWAPFILE"
}

swapfile_disable() {
  if swap_active "$SWAPFILE"; then
    swapoff "$SWAPFILE" || return 1
  fi
  fstab_drop "$SWAPFILE" none
}

zram_configure() {
  local before after
  if ((ZRAM_PERCENT == 0)); then
    service_disable zramswap.service
    return
  fi
  apt_install zram-tools || return 1
  before="$(file_hash "$ZRAM_CONFIG" || true)"
  conf_set "$ZRAM_CONFIG" ALGO zstd || return 1
  conf_set "$ZRAM_CONFIG" PERCENT "$ZRAM_PERCENT" || return 1
  conf_set "$ZRAM_CONFIG" PRIORITY 100 || return 1
  after="$(file_hash "$ZRAM_CONFIG")"
  service_enable zramswap.service || return 1
  if [[ $before != "$after" ]] || ! service_active zramswap.service; then
    systemctl restart zramswap.service
  fi
}

zram_active() {
  swapon --noheadings --show=NAME 2>/dev/null | grep -q '^/dev/zram'
}

swappiness_apply() {
  deploy_rendered "$HYPRTRIXIE_ROOT/system/sysctl/90-hyprtrixie-swap.conf" "$SYSCTL_SWAP" 0644 root \
    "SWAPPINESS=$SWAPPINESS" || return 1
  sysctl -q -p "$SYSCTL_SWAP"
}

swappiness_set() {
  [[ "$(sysctl -n vm.swappiness)" == "$SWAPPINESS" && "$(sysctl -n vm.page-cluster)" == 0 ]]
}

swapfile_nocow() {
  root_is_btrfs || return 0
  [[ "$(lsattr "$SWAPFILE" 2>/dev/null | cut -d' ' -f1)" == *C* ]]
}

module_04() {
  if ((SWAPFILE_GB > 0)); then
    run_step "Prepare a place for the swapfile" swap_dir_prepare
    run_step "Create a $SWAPFILE_GB GiB swapfile" swapfile_create
    run_step "Turn the swapfile on" swapfile_enable
  else
    run_step "Turn the swapfile off" swapfile_disable
  fi
  run_step "Compress memory with zram" zram_configure
  run_step "Tune swappiness" swappiness_apply
}

verify_04_swapfile() {
  ((SWAPFILE_GB > 0)) || return 0
  run_check "The swapfile is on" swap_active "$SWAPFILE"
  run_check "The swapfile is not copy-on-write" swapfile_nocow
}

verify_04_zram() {
  ((ZRAM_PERCENT > 0)) || return 0
  run_check "zram swap is on" zram_active
}

verify_04_swappiness() {
  run_check "Swappiness is $SWAPPINESS" swappiness_set
}
