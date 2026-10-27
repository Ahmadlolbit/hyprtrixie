CPU_VENDOR=""
CPU_MODEL=""
GPU_VENDORS=""
GPU_NAMES=()
GPU_HYBRID=0
NVIDIA_OPEN=0
WIFI_VENDORS=""
IS_LAPTOP=0
IS_EFI=0
SECURE_BOOT="unknown"
VIRT="none"
ROOT_FS=""
ROOT_SUBVOL=""
ROOT_DEVICE=""
ROOT_ENCRYPTED=0
RAM_MB=0
DMI_VENDOR=""
DMI_PRODUCT=""

pci_vendor() {
  case "$1" in
    8086) printf 'intel' ;;
    1002 | 1022) printf 'amd' ;;
    10de) printf 'nvidia' ;;
    10ec) printf 'realtek' ;;
    14c3) printf 'mediatek' ;;
    168c | 17cb) printf 'atheros' ;;
    14e4) printf 'broadcom' ;;
    *) printf 'other' ;;
  esac
}

pci_ids() {
  grep -oE '\[[0-9a-f]{4}:[0-9a-f]{4}\]' <<<"$1" | tail -n1 | tr -d '[]'
}

parse_cpu_vendor() {
  awk -F': *' '/^vendor_id/ {
    if ($2 == "GenuineIntel") print "intel"; else if ($2 == "AuthenticAMD") print "amd"; else print "other"
    exit
  }'
}

parse_gpus() {
  grep -E '\[(0300|0302|0380)\]:' || true
}

parse_wifi() {
  grep -E '\[0280\]:' || true
}

nvidia_open_capable() {
  local device=$((16#$1))
  ((device >= 16#1e00))
}

wifi_driver_vendor() {
  case "$1" in
    iwlwifi | iwlmvm | iwlmld) printf 'intel' ;;
    rtw88* | rtw89* | rtl8* | r8188eu) printf 'realtek' ;;
    mt7*) printf 'mediatek' ;;
    ath*) printf 'atheros' ;;
    brcmfmac | brcmsmac | wl) printf 'broadcom' ;;
    *) printf 'other' ;;
  esac
}

add_word() {
  local list="$1" word="$2"
  [[ " $list " == *" $word "* ]] && {
    printf '%s' "$list"
    return
  }
  printf '%s' "${list:+$list }$word"
}

detect_gpus() {
  local lspci_out="$1" line ids vendor
  GPU_VENDORS=""
  GPU_NAMES=()
  NVIDIA_OPEN=0
  while IFS= read -r line; do
    [[ -n $line ]] || continue
    ids="$(pci_ids "$line")"
    vendor="$(pci_vendor "${ids%%:*}")"
    GPU_VENDORS="$(add_word "$GPU_VENDORS" "$vendor")"
    GPU_NAMES+=("$(sed -E 's/^[^:]+:[^:]+: //; s/ \[[0-9a-f]{4}:[0-9a-f]{4}\].*$//' <<<"$line")")
    if [[ $vendor == nvidia ]] && nvidia_open_capable "${ids#*:}"; then
      NVIDIA_OPEN=1
    fi
  done < <(parse_gpus <<<"$lspci_out")
  GPU_HYBRID=0
  [[ $GPU_VENDORS == *" "* ]] && GPU_HYBRID=1
  return 0
}

detect_wifi() {
  local lspci_out="$1" line ids iface driver
  WIFI_VENDORS=""
  while IFS= read -r line; do
    [[ -n $line ]] || continue
    ids="$(pci_ids "$line")"
    WIFI_VENDORS="$(add_word "$WIFI_VENDORS" "$(pci_vendor "${ids%%:*}")")"
  done < <(parse_wifi <<<"$lspci_out")
  for iface in /sys/class/net/*/wireless; do
    [[ -e $iface ]] || continue
    driver="$(basename "$(readlink -f "${iface%/wireless}/device/driver")")"
    WIFI_VENDORS="$(add_word "$WIFI_VENDORS" "$(wifi_driver_vendor "$driver")")"
  done
  return 0
}

secure_boot_state() {
  local var
  if [[ ! -d /sys/firmware/efi ]]; then
    printf 'legacy'
    return
  fi
  if command -v mokutil >/dev/null 2>&1; then
    case "$(mokutil --sb-state 2>/dev/null)" in
      *enabled*) printf 'enabled' ;;
      *disabled*) printf 'disabled' ;;
      *) printf 'unknown' ;;
    esac
    return
  fi
  for var in /sys/firmware/efi/efivars/SecureBoot-*; do
    [[ -f $var ]] || continue
    if [[ "$(od -An -t u1 -j4 -N1 "$var" | tr -d ' ')" == "1" ]]; then
      printf 'enabled'
    else
      printf 'disabled'
    fi
    return
  done
  printf 'unknown'
}

detect_laptop() {
  local chassis
  chassis="$(cat /sys/class/dmi/id/chassis_type 2>/dev/null || printf 0)"
  case "$chassis" in
    8 | 9 | 10 | 11 | 12 | 14 | 18 | 21 | 30 | 31 | 32) return 0 ;;
  esac
  compgen -G '/sys/class/power_supply/BAT*' >/dev/null
}

detect_root() {
  local source options
  ROOT_FS="$(findmnt -no FSTYPE / 2>/dev/null)"
  options="$(findmnt -no OPTIONS / 2>/dev/null)"
  ROOT_SUBVOL=""
  if [[ $options =~ (^|,)subvol=/?([^,]*) ]]; then
    ROOT_SUBVOL="${BASH_REMATCH[2]}"
  fi
  source="$(findmnt -no SOURCE / 2>/dev/null)"
  ROOT_DEVICE="${source%%\[*}"
  ROOT_ENCRYPTED=0
  if [[ -n $ROOT_DEVICE ]] && lsblk -no TYPE -s "$ROOT_DEVICE" 2>/dev/null | grep -qx crypt; then
    ROOT_ENCRYPTED=1
  fi
}

detect_all() {
  local lspci_out=""
  CPU_VENDOR="$(parse_cpu_vendor </proc/cpuinfo)"
  CPU_MODEL="$(awk -F': *' '/^model name/ { print $2; exit }' /proc/cpuinfo)"
  command -v lspci >/dev/null 2>&1 && lspci_out="$(lspci -nn 2>/dev/null)"
  detect_gpus "$lspci_out"
  detect_wifi "$lspci_out"
  IS_LAPTOP=0
  detect_laptop && IS_LAPTOP=1
  IS_EFI=0
  [[ -d /sys/firmware/efi ]] && IS_EFI=1
  SECURE_BOOT="$(secure_boot_state)"
  VIRT="$(systemd-detect-virt 2>/dev/null || true)"
  VIRT="${VIRT:-none}"
  detect_root
  RAM_MB=$(($(awk '/^MemTotal/ { print $2 }' /proc/meminfo) / 1024))
  DMI_VENDOR="$(cat /sys/class/dmi/id/sys_vendor 2>/dev/null || true)"
  DMI_PRODUCT="$(cat /sys/class/dmi/id/product_version 2>/dev/null || true)"
  [[ -z $DMI_PRODUCT || $DMI_PRODUCT == "None" ]] && DMI_PRODUCT="$(cat /sys/class/dmi/id/product_name 2>/dev/null || true)"
  return 0
}

has_gpu() {
  [[ " $GPU_VENDORS " == *" $1 "* ]]
}

has_wifi() {
  [[ " $WIFI_VENDORS " == *" $1 "* ]]
}

is_thinkpad() {
  [[ $DMI_PRODUCT == ThinkPad* ]]
}

detect_summary() {
  local gpu
  printf 'Machine     %s %s\n' "${DMI_VENDOR:-unknown}" "${DMI_PRODUCT:-}"
  printf 'CPU         %s (%s)\n' "${CPU_MODEL:-unknown}" "${CPU_VENDOR:-?}"
  for gpu in "${GPU_NAMES[@]}"; do
    printf 'GPU         %s\n' "$gpu"
  done
  ((GPU_HYBRID)) && printf 'Graphics    hybrid (%s)\n' "$GPU_VENDORS"
  ((NVIDIA_OPEN)) && printf 'NVIDIA      open kernel modules supported\n'
  ((${#GPU_NAMES[@]})) || printf 'GPU         none found\n'
  printf 'Wi-Fi       %s\n' "${WIFI_VENDORS:-none found}"
  printf 'Memory      %s MiB\n' "$RAM_MB"
  printf 'Form        %s%s\n' "$( ((IS_LAPTOP)) && printf laptop || printf desktop)" "$([[ $VIRT != none ]] && printf ', virtual machine (%s)' "$VIRT")"
  printf 'Firmware    %s, Secure Boot %s\n' "$( ((IS_EFI)) && printf UEFI || printf BIOS)" "$SECURE_BOOT"
  printf 'Root        %s%s%s\n' "${ROOT_FS:-?}" "${ROOT_SUBVOL:+ (subvolume $ROOT_SUBVOL)}" "$( ((ROOT_ENCRYPTED)) && printf ', encrypted')"
}
