#!/bin/sh
set -eu

options="noatime,compress=zstd:1"
top=/mnt/hyprtrixie-top
fstab=/target/etc/fstab

say() {
  printf '%s\n' "$*"
}

fail() {
  printf 'Stopped: %s\n' "$*" >&2
  exit 1
}

mounted_on() {
  awk -v target="$1" '$2 == target { print $1; exit }' /proc/mounts
}

is_mounted() {
  [ -n "$(mounted_on "$1")" ]
}

target_dev=$(mounted_on /target)
[ -n "$target_dev" ] || fail "nothing is mounted on /target. Run this right after partitioning, before the base system is installed."
target_fs=$(awk '$2 == "/target" { print $3; exit }' /proc/mounts)
[ "$target_fs" = btrfs ] || fail "/target is $target_fs, not btrfs."
[ -f "$fstab" ] || fail "$fstab is missing."
[ ! -d /target/usr ] || fail "the base system is already installed; this has to run before that step."

uuid=$(blkid "$target_dev" | sed -n 's/.* UUID="\([^"]*\)".*/\1/p')
[ -n "$uuid" ] || fail "could not read the filesystem UUID of $target_dev."
boot_dev=$(mounted_on /target/boot)
efi_dev=$(mounted_on /target/boot/efi)
say "Filesystem $target_dev ($uuid)"

for dir in boot/efi boot home .snapshots var/log swap; do
  if is_mounted "/target/$dir"; then
    umount "/target/$dir"
  fi
done
umount /target

mkdir -p "$top"
mount -t btrfs -o subvolid=5 "$target_dev" "$top"
if [ -d "$top/@rootfs" ] && [ ! -e "$top/@" ]; then
  mv "$top/@rootfs" "$top/@"
  say "Renamed @rootfs to @"
fi
if [ ! -d "$top/@" ]; then
  umount "$top"
  fail "there is no @ or @rootfs subvolume on $target_dev."
fi
for name in @home @snapshots @var_log @swap; do
  if [ ! -d "$top/$name" ]; then
    btrfs subvolume create "$top/$name" >/dev/null
    say "Created $name"
  fi
done
umount "$top"
rmdir "$top"

mount -t btrfs -o "subvol=@,$options" "$target_dev" /target
for pair in home:@home .snapshots:@snapshots var/log:@var_log swap:@swap; do
  dir=${pair%%:*}
  name=${pair#*:}
  mkdir -p "/target/$dir"
  if [ "$name" = @swap ]; then
    mount -t btrfs -o "subvol=$name,noatime" "$target_dev" "/target/$dir"
  else
    mount -t btrfs -o "subvol=$name,$options" "$target_dev" "/target/$dir"
  fi
done
mkdir -p /target/boot
if [ -n "$boot_dev" ]; then
  mount "$boot_dev" /target/boot
fi
if [ -n "$efi_dev" ]; then
  mkdir -p /target/boot/efi
  mount "$efi_dev" /target/boot/efi
fi

[ -f "$fstab.hyprtrixie-old" ] || cp "$fstab" "$fstab.hyprtrixie-old"
awk -v uuid="$uuid" -v opts="$options" '
  $1 !~ /^#/ && $2 == "/" { print "UUID=" uuid " / btrfs subvol=@," opts " 0 0"; next }
  $1 !~ /^#/ && ($2 == "/home" || $2 == "/.snapshots" || $2 == "/var/log" || $2 == "/swap") { next }
  { print }
' "$fstab.hyprtrixie-old" >"$fstab"
{
  printf 'UUID=%s /home btrfs subvol=@home,%s 0 0\n' "$uuid" "$options"
  printf 'UUID=%s /.snapshots btrfs subvol=@snapshots,%s 0 0\n' "$uuid" "$options"
  printf 'UUID=%s /var/log btrfs subvol=@var_log,%s 0 0\n' "$uuid" "$options"
  printf 'UUID=%s /swap btrfs subvol=@swap,noatime 0 0\n' "$uuid"
} >>"$fstab"

say ""
say "Done. Mounted now:"
awk '$2 ~ /^\/target/ { print "  " $2 "  (" $4 ")" }' /proc/mounts
say ""
say "New $fstab:"
grep -v '^#' "$fstab"
say ""
say "Go back to the installer and continue with \"Install the base system\"."
