MENU_SECTIONS=(profile apps dev look shell input system storage security run)
declare -gA MENU_TITLES=(
  [profile]="Profile"
  [apps]="Applications"
  [dev]="Developer tools"
  [look]="Look and feel"
  [shell]="Shell"
  [input]="Input and display"
  [system]="System"
  [storage]="Storage"
  [security]="Security and network"
  [run]="Run"
)
MENU_SEPARATOR="──────────────────────────────"

option_display() {
  local key="$1" value="${!1}" count total
  case "${OPTION_TYPE[$key]}" in
    multi)
      count="$(wc -w <<<"$value")"
      total="$(option_items "$key" | wc -l)"
      printf '%s of %s' "$count" "$total"
      ;;
    *) printf '%s' "${value:-none}" ;;
  esac
}

section_summary() {
  local keys=() key summary=""
  mapfile -t keys < <(section_keys "$1")
  for key in "${keys[@]:0:3}"; do
    summary+="${summary:+ · }$(option_display "$key")"
  done
  printf '%s' "$summary"
}

menu_line() {
  if [[ -n ${3:-} ]] && (($(ui_columns) >= 130)); then
    printf '%-30s %-16s %s%s%s' "$1" "$2" "$C_DIM" "$3" "$C_RESET"
  else
    printf '%-30s %s' "$1" "$2"
  fi
}

menu_header() {
  local key="$1"
  printf '%s' "${OPTION_LABEL[$key]}"
  [[ -n ${OPTION_HELP[$key]} ]] && printf '\n%s' "${OPTION_HELP[$key]}"
  return 0
}

menu_edit_number() {
  local key="$1" label="$2" value
  while :; do
    value="$(ui_input "$label" "${!key}")" || return 0
    if option_valid "$key" "$value"; then
      printf -v "$key" '%s' "$value"
      return 0
    fi
    ui_status warn "Please enter a whole number"
  done
}

menu_edit() {
  local key="$1" label value picked=() before
  label="$(menu_header "$key")"
  case "${OPTION_TYPE[$key]}" in
    choice)
      mapfile -t picked < <(option_items "$key")
      value="$(ui_choose "$label" "${!key}" "${picked[@]}")" || return 0
      [[ -n $value ]] || return 0
      before="${!key}"
      printf -v "$key" '%s' "$value"
      if [[ $key == PROFILE && $before != "$value" ]] &&
        ui_confirm "Reset applications and tools to the $value preset?" yes; then
        config_reset_profile
      fi
      ;;
    bool)
      value="$(ui_choose "$label" "${!key}" yes no)" || return 0
      [[ -n $value ]] && printf -v "$key" '%s' "$value"
      ;;
    number)
      menu_edit_number "$key" "$label"
      ;;
    multi)
      mapfile -t picked < <(option_items "$key")
      local current="${!key}" out
      out="$(ui_choose_many "$label" "${current// /,}" "${picked[@]}")" || return 0
      picked=()
      [[ -n $out ]] && mapfile -t picked <<<"$out"
      printf -v "$key" '%s' "${picked[*]}"
      ;;
    *)
      value="$(ui_input "$label" "${!key}")" || return 0
      if [[ $value =~ ^$CONFIG_CHARS*$ ]]; then
        printf -v "$key" '%s' "$value"
      else
        ui_status warn "Only letters, digits, spaces and . _ : / @ + , - are allowed"
      fi
      ;;
  esac
}

menu_section() {
  local section="$1" keys=() lines=() key choice i
  while :; do
    mapfile -t keys < <(section_keys "$section")
    ((${#keys[@]})) || return 0
    if ((${#keys[@]} == 1)); then
      menu_edit "${keys[0]}"
      return 0
    fi
    lines=()
    for key in "${keys[@]}"; do
      lines+=("$(menu_line "${OPTION_LABEL[$key]}" "$(option_display "$key")" "${OPTION_HELP[$key]}")")
    done
    lines+=("Back")
    choice="$(ui_choose "${MENU_TITLES[$section]}" "" "${lines[@]}")" || return 0
    [[ -z $choice || $choice == "Back" ]] && return 0
    for i in "${!lines[@]}"; do
      [[ ${lines[$i]} == "$choice" ]] && menu_edit "${keys[$i]}"
    done
  done
}

menu_load() {
  local path
  path="$(ui_input "Load a configuration file" "$CONFIG_FILE")" || return 0
  [[ -n $path ]] || return 0
  if config_load "$path"; then
    ui_status ok "Loaded $path"
  else
    ui_status fail "Could not read $path"
  fi
}

menu_main() {
  local section choice lines=() sections=() keys=() i
  while :; do
    lines=()
    sections=()
    for section in "${MENU_SECTIONS[@]}"; do
      mapfile -t keys < <(section_keys "$section")
      ((${#keys[@]})) || continue
      lines+=("$(menu_line "${MENU_TITLES[$section]}" "$(section_summary "$section")")")
      sections+=("$section")
    done
    lines+=("$MENU_SEPARATOR" "Save configuration" "Load configuration" "Install" "Exit")
    choice="$(ui_choose "hyprtrixie ${HYPRTRIXIE_VERSION:-}  ·  choose what to change, then Install" "" "${lines[@]}")" || return 1
    case "$choice" in
      "" | "$MENU_SEPARATOR") continue ;;
      "Save configuration")
        config_save && ui_status ok "Saved to $CONFIG_FILE"
        continue
        ;;
      "Load configuration")
        menu_load
        continue
        ;;
      "Install") return 0 ;;
      "Exit") return 1 ;;
    esac
    for i in "${!lines[@]}"; do
      [[ ${lines[$i]} == "$choice" ]] && menu_section "${sections[$i]}"
    done
  done
}
