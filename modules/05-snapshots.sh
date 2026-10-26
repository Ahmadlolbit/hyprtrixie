register_module 05 "Snapshots and rollback"

option SNAPSHOTS --section storage --label "System snapshots" --type bool --default yes
option SNAPSHOT_DAILY --section storage --label "Daily snapshots to keep" --type number --default 7 --when snapshots_enabled
option SNAPSHOT_WEEKLY --section storage --label "Weekly snapshots to keep" --type number --default 4 --when snapshots_enabled
option SNAPSHOT_MONTHLY --section storage --label "Monthly snapshots to keep" --type number --default 1 --when snapshots_enabled

SNAPPER_DEFAULTS="${HYPRTRIXIE_SNAPPER_DEFAULTS:-/etc/default/snapper}"
SNAPPER_CONFIG="${HYPRTRIXIE_SNAPPER_CONFIG:-/etc/snapper/configs/root}"
SNAPPER_TEMPLATE="/usr/share/snapper/config-templates/default"
SNAPSHOT_DIR="${HYPRTRIXIE_SNAPSHOT_DIR:-/.snapshots}"
FIRST_SNAPSHOT="hyprtrixie: before setup"

snapshots_enabled() {
  [[ $SNAPSHOTS == yes ]]
}

snapshots_mount() {
  local options
  options="$(findmnt -no OPTIONS "$SNAPSHOT_DIR" 2>/dev/null)"
  [[ $options =~ (^|,)subvol=/?@snapshots(,|$) ]] && return 0
  if [[ -d $SNAPSHOT_DIR ]]; then
    if [[ -n "$(ls -A "$SNAPSHOT_DIR" 2>/dev/null)" ]]; then
      printf '%s already holds snapshots inside the root subvolume; move or delete them first\n' "$SNAPSHOT_DIR"
      return 1
    fi
    if btrfs subvolume show "$SNAPSHOT_DIR" >/dev/null 2>&1; then
      btrfs subvolume delete "$SNAPSHOT_DIR" || return 1
    else
      rmdir "$SNAPSHOT_DIR" || return 1
    fi
  fi
  mount_subvolume @snapshots "$SNAPSHOT_DIR" noatime || return 1
  chmod 0750 "$SNAPSHOT_DIR"
}

snapper_register() {
  local current
  current="$(sed -nE 's/^SNAPPER_CONFIGS="(.*)"$/\1/p' "$SNAPPER_DEFAULTS" 2>/dev/null)"
  [[ " $current " == *" root "* ]] && return 0
  conf_set "$SNAPPER_DEFAULTS" SNAPPER_CONFIGS "\"${current:+$current }root\""
}

snapper_configure() {
  local pair
  if [[ ! -f $SNAPPER_CONFIG ]]; then
    install -D -m 0640 "$SNAPPER_TEMPLATE" "$SNAPPER_CONFIG" || return 1
  fi
  snapper_register || return 1
  for pair in SUBVOLUME=/ FSTYPE=btrfs TIMELINE_CREATE=yes TIMELINE_CLEANUP=yes TIMELINE_LIMIT_HOURLY=0 \
    "TIMELINE_LIMIT_DAILY=$SNAPSHOT_DAILY" "TIMELINE_LIMIT_WEEKLY=$SNAPSHOT_WEEKLY" \
    "TIMELINE_LIMIT_MONTHLY=$SNAPSHOT_MONTHLY" TIMELINE_LIMIT_YEARLY=0 NUMBER_CLEANUP=yes NUMBER_LIMIT=10 \
    NUMBER_LIMIT_IMPORTANT=5 ALLOW_GROUPS=sudo SYNC_ACL=yes; do
    conf_set "$SNAPPER_CONFIG" "${pair%%=*}" "\"${pair#*=}\"" || return 1
  done
}

snapper_first() {
  snapper --no-dbus -c root list --columns description 2>/dev/null | grep -qF "$FIRST_SNAPSHOT" && return 0
  snapper --no-dbus -c root create --description "$FIRST_SNAPSHOT" --cleanup-algorithm number
}

snapper_ready() {
  snapper --no-dbus list-configs 2>/dev/null | grep -qE '^root ' && snapper --no-dbus -c root list >/dev/null 2>&1
}

timers_active() {
  service_active snapper-timeline.timer && service_active snapper-cleanup.timer
}

container_storage() {
  local pair dir name
  for pair in /var/lib/docker:@docker /var/lib/containers:@containers; do
    dir="${pair%%:*}"
    name="${pair#*:}"
    mountpoint -q "$dir" && continue
    if [[ -n "$(ls -A "$dir" 2>/dev/null)" ]]; then
      printf '%s already has data; it stays inside the root subvolume\n' "$dir"
      continue
    fi
    mount_subvolume "$name" "$dir" noatime || return 1
  done
}

container_storage_ok() {
  local dir
  for dir in /var/lib/docker /var/lib/containers; do
    mountpoint -q "$dir" && continue
    [[ -n "$(ls -A "$dir" 2>/dev/null)" ]] || return 1
  done
}

grub_btrfs_install() {
  local stamp="$STATE_DIR/grub-btrfs.commit" build
  GRUB_BTRFS_FRESH=0
  apt_install_list grub-btrfs || return 1
  fetch_git_cached grub-btrfs || return 1
  if [[ -x /etc/grub.d/41_snapshots-btrfs && "$(cat "$stamp" 2>/dev/null)" == "$SOURCE_COMMIT" ]]; then
    return 0
  fi
  build="$(mktemp -d)" || return 1
  cp -a "$FETCHED_PATH/." "$build/" || return 1
  make -C "$build" install GRUB_UPDATE_EXCLUDE=true || return 1
  rm -rf "$build"
  mkdir -p "$STATE_DIR" && printf '%s\n' "$SOURCE_COMMIT" >"$stamp"
  GRUB_BTRFS_FRESH=1
}

grub_btrfs_enable() {
  service_start grub-btrfsd.service || return 1
  if ((${GRUB_BTRFS_FRESH:-0})) || [[ ! -f /boot/grub/grub-btrfs.cfg ]]; then
    update-grub
  fi
}

grub_btrfs_ready() {
  [[ -x /etc/grub.d/41_snapshots-btrfs ]] && service_active grub-btrfsd.service
}

rollback_install() {
  apt_install_list rollback || return 1
  fetch_git_cached snapper-rollback || return 1
  deploy_file "$FETCHED_PATH/snapper-rollback.py" /usr/local/sbin/snapper-rollback 0755 || return 1
  deploy_rendered "$HYPRTRIXIE_ROOT/system/snapper-rollback.conf" /etc/snapper-rollback.conf 0644 root \
    "SUBVOL=$(root_subvolume)" "DEVICE=$(root_device)" || return 1
  mkdir -p /btrfsroot
}

rollback_ready() {
  /usr/local/sbin/snapper-rollback --help >/dev/null 2>&1
}

module_05() {
  if ! root_is_btrfs; then
    module_skip "Snapshots need a btrfs root; this system uses $(root_fstype)"
    return
  fi
  if ! snapshots_enabled; then
    module_skip "Snapshots are turned off in the configuration"
    return
  fi
  [[ $(root_subvolume) == @rootfs ]] && ui_note "Root is the installer's @rootfs; /home is inside root snapshots."
  run_step "Install snapper" apt_install_list snapshots
  run_step "Mount the snapshots subvolume" snapshots_mount
  run_step "Configure snapper" snapper_configure
  run_step "Turn on the snapshot timers" service_start snapper-timeline.timer snapper-cleanup.timer
  run_step "Take the first snapshot" snapper_first
  run_step "Keep container storage out of snapshots" container_storage
  run_step "Install grub-btrfs" grub_btrfs_install
  run_step "Show snapshots in the boot menu" grub_btrfs_enable
  run_step "Install snapper-rollback" rollback_install
}

verify_05_snapper() {
  root_is_btrfs && snapshots_enabled || return 0
  run_check "Snapper is set up for /" snapper_ready
  run_check "Snapshot timers are running" timers_active
}

verify_05_containers() {
  root_is_btrfs && snapshots_enabled || return 0
  run_check "Container storage is outside root snapshots" container_storage_ok
}

verify_05_grub() {
  root_is_btrfs && snapshots_enabled || return 0
  run_check "Snapshots appear in the boot menu" grub_btrfs_ready
}

verify_05_rollback() {
  root_is_btrfs && snapshots_enabled || return 0
  run_check "snapper-rollback works" rollback_ready
}
