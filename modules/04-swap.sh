register_module 04 "Swap and memory"

option SWAPFILE_GB --section storage --label "Swapfile size in GiB (0 = none)" --type number --default 8

SWAP_DIR="${HYPRTRIXIE_SWAP_DIR:-/swap}"
SWAPFILE="$SWAP_DIR/swapfile"

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
}

verify_04_swapfile() {
  ((SWAPFILE_GB > 0)) || return 0
  run_check "The swapfile is on" swap_active "$SWAPFILE"
  run_check "The swapfile is not copy-on-write" swapfile_nocow
}
