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
      if [[ -n ${OPTION_GROUPS[$key]} ]]; then
        menu_edit_groups "$key"
        return 0
      fi
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

declare -gA MENU_GROUP=()

menu_group_map() {
  local key="$1" item
  MENU_GROUP=()
  for item in $(option_items "$key"); do
    MENU_GROUP[$item]="$("${OPTION_GROUP_OF[$key]}" "$item")"
  done
}

menu_group_items() {
  local key="$1" group="$2" item
  for item in $(option_items "$key"); do
    [[ ${MENU_GROUP[$item]:-} == "$group" ]] && printf '%s\n' "$item"
  done
  return 0
}

menu_group_display() {
  local key="$1" group="$2" item count=0 total=0
  for item in $(menu_group_items "$key" "$group"); do
    total=$((total + 1))
    option_enabled "$key" "$item" && count=$((count + 1))
  done
  printf '%s of %s' "$count" "$total"
}

menu_edit_group() {
  local key="$1" group="$2" title="$3" items=() names=() ticked=() picked=() chosen=() item out i name
  mapfile -t items < <(menu_group_items "$key" "$group")
  for item in "${items[@]}"; do
    name="$(option_name "$key" "$item")"
    names+=("$name")
    option_enabled "$key" "$item" && ticked+=("$name")
  done
  out="$(ui_choose_many "$title" "$(
    IFS=,
    printf '%s' "${ticked[*]}"
  )" "${names[@]}")" || return 0
  [[ -n $out ]] && mapfile -t picked <<<"$out"
  for item in $(option_items "$key"); do
    if [[ ${MENU_GROUP[$item]:-} != "$group" ]]; then
      option_enabled "$key" "$item" && chosen+=("$item")
      continue
    fi
    for i in "${!items[@]}"; do
      [[ ${items[$i]} == "$item" ]] || continue
      for name in "${picked[@]}"; do
        [[ $name == "${names[$i]}" ]] && chosen+=("$item")
      done
    done
  done
  printf -v "$key" '%s' "${chosen[*]}"
}

menu_edit_groups() {
  local key="$1" group title choice lines=() groups=() titles=() i
  menu_group_map "$key"
  while :; do
    lines=()
    groups=()
    titles=()
    while IFS=$'\t' read -r group title; do
      groups+=("$group")
      titles+=("$title")
      lines+=("$(menu_line "$title" "$(menu_group_display "$key" "$group")")")
    done < <("${OPTION_GROUPS[$key]}")
    lines+=("Done")
    choice="$(ui_choose "$(menu_header "$key")" "" "${lines[@]}")" || return 0
    [[ -z $choice || $choice == "Done" ]] && return 0
    for i in "${!lines[@]}"; do
      [[ ${lines[$i]} == "$choice" ]] && menu_edit_group "$key" "${groups[$i]}" "${titles[$i]}"
    done
  done
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
