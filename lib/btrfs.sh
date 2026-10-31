FSTAB="${HYPRTRIXIE_FSTAB:-/etc/fstab}"
BTRFS_TOP="${HYPRTRIXIE_BTRFS_TOP:-/run/hyprtrixie/btrfs}"

root_fstype() {
  findmnt -no FSTYPE / 2>/dev/null
}

root_is_btrfs() {
  [[ "$(root_fstype)" == btrfs ]]
}

root_uuid() {
  findmnt -no UUID / 2>/dev/null
}

root_device() {
  local source
  source="$(findmnt -no SOURCE / 2>/dev/null)"
  printf '%s' "${source%%\[*}"
}

root_subvolume() {
  local options
  options="$(findmnt -no OPTIONS / 2>/dev/null)"
  [[ $options =~ (^|,)subvol=/?([^,]*) ]] && printf '%s' "${BASH_REMATCH[2]}"
  return 0
}

btrfs_top_mount() {
  mountpoint -q "$BTRFS_TOP" && return 0
  mkdir -p "$BTRFS_TOP" || return 1
  mount -o subvolid=5 "$(root_device)" "$BTRFS_TOP"
}

btrfs_top_umount() {
  if mountpoint -q "$BTRFS_TOP"; then
    umount "$BTRFS_TOP"
  fi
  return 0
}

btrfs_subvolume_ensure() {
  local name="$1"
  btrfs_top_mount || return 1
  [[ -d $BTRFS_TOP/$name ]] && return 0
  btrfs subvolume create "$BTRFS_TOP/$name"
}

fstab_line_for() {
  local spec="$1" target="$2"
  [[ -f $FSTAB ]] || return 1
  awk -v spec="$spec" -v target="$target" '
    $1 ~ /^#/ { next }
    target == "none" && $1 == spec { print; found = 1 }
    target != "none" && $2 == target { print; found = 1 }
    END { exit !found }
  ' "$FSTAB"
}

fstab_ensure() {
  local spec="$1" target="$2" type="$3" options="$4" line tmp
  line="$spec $target $type $options 0 0"
  if [[ "$(fstab_line_for "$spec" "$target")" == "$line" ]]; then
    return 0
  fi
  if fstab_line_for "$spec" "$target" >/dev/null; then
    tmp="$(mktemp)" || return 1
    awk -v spec="$spec" -v target="$target" -v line="$line" '
      $1 !~ /^#/ && ((target == "none" && $1 == spec) || (target != "none" && $2 == target)) { print line; next }
      { print }
    ' "$FSTAB" >"$tmp"
    cat "$tmp" >"$FSTAB"
    rm -f "$tmp"
  else
    printf '%s\n' "$line" >>"$FSTAB"
  fi
  log info "fstab: $line"
  systemctl daemon-reload 2>/dev/null || true
}

fstab_drop() {
  local spec="$1" target="$2" tmp
  fstab_line_for "$spec" "$target" >/dev/null || return 0
  tmp="$(mktemp)" || return 1
  awk -v spec="$spec" -v target="$target" '
    $1 !~ /^#/ && ((target == "none" && $1 == spec) || (target != "none" && $2 == target)) { next }
    { print }
  ' "$FSTAB" >"$tmp"
  cat "$tmp" >"$FSTAB"
  rm -f "$tmp"
  systemctl daemon-reload 2>/dev/null || true
}

mount_subvolume() {
  local name="$1" target="$2" options="${3:-noatime}" uuid
  btrfs_subvolume_ensure "$name" || return 1
  mkdir -p "$target" || return 1
  uuid="$(root_uuid)"
  if [[ -z $uuid ]]; then
    log error "cannot find the root filesystem's UUID"
    return 1
  fi
  fstab_ensure "UUID=$uuid" "$target" btrfs "subvol=$name,$options" || return 1
  mountpoint -q "$target" || mount "$target"
}
