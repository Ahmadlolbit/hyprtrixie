UI_INTERACTIVE=0
UI_GUM=0
UI_ACCENT=5
UI_MUTED=8
UI_WIDTH=64
UI_SPIN_PID=""
C_RESET=""
C_BOLD=""
C_DIM=""
C_RED=""
C_GREEN=""
C_YELLOW=""
C_BLUE=""
C_ACCENT=""

ui_init() {
  UI_INTERACTIVE=0
  if [[ -t 0 && -t 1 && ${ASSUME_YES:-0} != 1 ]]; then
    UI_INTERACTIVE=1
  fi
  UI_GUM=0
  if command -v gum >/dev/null 2>&1; then
    UI_GUM=1
  fi
  if [[ -t 1 ]]; then
    C_RESET=$'\e[0m'
    C_BOLD=$'\e[1m'
    C_DIM=$'\e[2m'
    C_RED=$'\e[31m'
    C_GREEN=$'\e[32m'
    C_YELLOW=$'\e[33m'
    C_BLUE=$'\e[34m'
    C_ACCENT=$'\e[35m'
  fi
}

ui_can_ask() {
  ((UI_INTERACTIVE))
}

ui_fancy() {
  ((UI_GUM)) && [[ -t 1 ]]
}

ui_columns() {
  local size columns=""
  size="$(stty size 2>/dev/null </dev/tty)" && columns="${size#* }"
  [[ $columns =~ ^[0-9]+$ ]] || columns="${COLUMNS:-80}"
  printf '%s' "$columns"
}

ui_logo() {
  local file="$HYPRTRIXIE_ROOT/system/screensaver.txt" line
  [[ -t 1 && -f $file ]] || return 0
  (($(ui_columns) >= 60)) || return 0
  printf '\n'
  while IFS= read -r line; do
    printf '  %s%s%s\n' "$C_ACCENT$C_BOLD" "$line" "$C_RESET"
  done <"$file"
  return 0
}

ui_rule() {
  local text="$1" fill
  fill=$((UI_WIDTH - ${#text} - 4))
  ((fill < 3)) && fill=3
  printf '\n%s── %s %s%s\n' "$C_ACCENT$C_BOLD" "$text" "$(printf '─%.0s' $(seq "$fill"))" "$C_RESET"
}

ui_header() {
  if ui_fancy; then
    gum style --border double --border-foreground "$UI_ACCENT" --bold \
      --padding "0 2" --margin "1 0 0 0" --width "$UI_WIDTH" "$@"
  else
    printf '\n%s%s%s\n' "$C_BOLD" "$1" "$C_RESET"
    shift
    (($#)) && printf '%s\n' "$@"
  fi
  return 0
}

ui_box() {
  if ui_fancy; then
    gum style --border normal --border-foreground "$UI_MUTED" --padding "0 1" --width "$UI_WIDTH" "$@"
  else
    printf '%s\n' "$@"
  fi
  return 0
}

ui_status() {
  local state="$1" text="$2" tag colour
  case "$state" in
    ok) tag=" ok " colour="$C_GREEN" ;;
    fail) tag="FAIL" colour="$C_RED" ;;
    skip) tag="skip" colour="$C_YELLOW" ;;
    warn) tag="warn" colour="$C_YELLOW" ;;
    *) tag=" .. " colour="$C_BLUE" ;;
  esac
  printf '  %s[%s]%s %s\n' "$colour" "$tag" "$C_RESET" "$text"
}

ui_note() {
  printf '         %s%s%s\n' "$C_DIM" "$*" "$C_RESET"
}

ui_spin_start() {
  ((UI_INTERACTIVE)) || return 0
  local text="$1"
  (
    trap 'exit 0' TERM
    local frames="|/-\\" i=0
    while :; do
      printf '\r  %s[ %s  ]%s %s' "$C_ACCENT" "${frames:i%4:1}" "$C_RESET" "$text"
      i=$((i + 1))
      sleep 0.15
    done
  ) &
  UI_SPIN_PID=$!
}

ui_spin_stop() {
  [[ -n $UI_SPIN_PID ]] || return 0
  kill "$UI_SPIN_PID" 2>/dev/null
  wait "$UI_SPIN_PID" 2>/dev/null
  UI_SPIN_PID=""
  printf '\r\e[K'
}

ui_confirm() {
  local prompt="$1" default="${2:-yes}" reply
  if ! ui_can_ask; then
    [[ $default == yes ]]
    return
  fi
  if ((UI_GUM)); then
    gum confirm --default="$([[ $default == yes ]] && printf true || printf false)" \
      --selected.background "$UI_ACCENT" "$prompt"
    return
  fi
  read -r -p "$prompt [$([[ $default == yes ]] && printf 'Y/n' || printf 'y/N')] " reply
  case "${reply,,}" in
    y | yes) return 0 ;;
    n | no) return 1 ;;
    *) [[ $default == yes ]] ;;
  esac
}

ui_choose() {
  local header="$1" current="$2" reply i
  shift 2
  if ! ui_can_ask; then
    printf '%s\n' "${current:-$1}"
    return 0
  fi
  if ((UI_GUM)); then
    gum choose --header "$header" --height 20 --cursor "> " \
      --cursor.foreground "$UI_ACCENT" --header.foreground "$UI_MUTED" \
      ${current:+--selected "$current"} -- "$@"
    return
  fi
  printf '%s\n' "$header"
  for ((i = 1; i <= $#; i++)); do
    printf '  %2d) %s\n' "$i" "${!i}"
  done
  read -r -p "> " reply
  [[ $reply =~ ^[0-9]+$ ]] && ((reply >= 1 && reply <= $#)) || return 1
  printf '%s\n' "${!reply}"
}

ui_choose_many() {
  local header="$1" selected="$2" reply item picked=()
  shift 2
  if ! ui_can_ask; then
    local items=()
    IFS=',' read -r -a items <<<"$selected"
    ((${#items[@]})) && printf '%s\n' "${items[@]}"
    return 0
  fi
  if ((UI_GUM)); then
    gum choose --no-limit --header "$header" --height 20 --cursor "> " \
      --cursor-prefix "[ ] " --selected-prefix "[x] " --unselected-prefix "[ ] " \
      --cursor.foreground "$UI_ACCENT" --header.foreground "$UI_MUTED" \
      ${selected:+--selected "$selected"} -- "$@"
    return
  fi
  printf '%s (space-separated, current: %s)\n' "$header" "${selected//,/ }"
  read -r -p "> " reply
  for item in $reply; do
    picked+=("$item")
  done
  ((${#picked[@]})) && printf '%s\n' "${picked[@]}"
  return 0
}

ui_input() {
  local header="$1" value="${2:-}" placeholder="${3:-}" reply
  if ! ui_can_ask; then
    printf '%s\n' "$value"
    return 0
  fi
  if ((UI_GUM)); then
    gum input --header "$header" --value "$value" --placeholder "$placeholder" \
      --prompt "> " --prompt.foreground "$UI_ACCENT" --header.foreground "$UI_MUTED"
    return
  fi
  read -r -e -p "$header [$value] " reply
  printf '%s\n' "${reply:-$value}"
}

ui_password() {
  local header="$1" reply
  ui_can_ask || return 1
  if ((UI_GUM)); then
    gum input --password --header "$header" --prompt "> " --prompt.foreground "$UI_ACCENT"
    return
  fi
  read -r -s -p "$header " reply
  printf '\n%s\n' "$reply"
}

ui_pager() {
  local file="$1"
  if ui_can_ask && ((UI_GUM)); then
    gum pager <"$file"
  else
    cat "$file"
  fi
}
