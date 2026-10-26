STATE_DIR="${HYPRTRIXIE_STATE_DIR:-/var/lib/hyprtrixie}"
STATUS_FILE="$STATE_DIR/status"
STEP_KIND="step"
CURRENT_MODULE=""
MODULE_FAILED=0
MODULE_PARTIAL=0
MODULE_IDS=()
STEP_FAILURES=()
STEP_SKIPS=()
declare -gA MODULE_TITLES=()
declare -gA MODULE_STATUS=()

register_module() {
  MODULE_IDS+=("$1")
  MODULE_TITLES[$1]="$2"
}

step_exec() {
  local fd pid rc
  exec {fd}> >(log_stream)
  pid=$!
  "$@" >&"$fd" 2>&1
  rc=$?
  exec {fd}>&-
  wait "$pid" 2>/dev/null
  return "$rc"
}

step_show_output() {
  local offset="$1" line
  while IFS= read -r line; do
    ui_note "$line"
  done < <(log_tail_since "$offset" 12)
}

step_choice() {
  local title="$1" pick
  case "${FAILURE_MODE:-ask}" in
    halt | continue)
      printf '%s' "${FAILURE_MODE:-ask}"
      return 0
      ;;
  esac
  if ! ui_can_ask; then
    printf 'continue'
    return 0
  fi
  pick="$(ui_choose "\"$title\" failed. What now?" "" Retry Skip Continue Halt)" || pick="Halt"
  printf '%s' "${pick,,}"
}

step_halt() {
  log error "run halted in module ${CURRENT_MODULE:-none}"
  exit 1
}

run_step() {
  local interactive=0 title rc offset choice
  if [[ ${1:-} == "--interactive" ]]; then
    interactive=1
    shift
  fi
  title="$1"
  shift
  while :; do
    log info "$STEP_KIND: $title"
    offset="$(log_size)"
    if ((interactive)); then
      "$@"
      rc=$?
    else
      ui_spin_start "$title"
      step_exec "$@"
      rc=$?
      ui_spin_stop
    fi
    if ((rc == 0)); then
      ui_status ok "$title"
      return 0
    fi
    log error "$STEP_KIND failed (exit $rc): $title"
    ui_status fail "$title"
    step_show_output "$offset"
    choice="$(step_choice "$title")"
    case "$choice" in
      retry)
        log info "retrying: $title"
        ;;
      skip)
        log warn "skipped: $title"
        STEP_SKIPS+=("${CURRENT_MODULE:-none}|$STEP_KIND|$title")
        MODULE_PARTIAL=1
        ui_status skip "$title"
        return 2
        ;;
      halt)
        STEP_FAILURES+=("${CURRENT_MODULE:-none}|$STEP_KIND|$title")
        MODULE_FAILED=1
        step_halt
        ;;
      *)
        STEP_FAILURES+=("${CURRENT_MODULE:-none}|$STEP_KIND|$title")
        MODULE_FAILED=1
        return 1
        ;;
    esac
  done
}

run_check() {
  STEP_KIND="check" run_step "$@"
}

module_begin() {
  CURRENT_MODULE="$1"
  MODULE_FAILED=0
  MODULE_PARTIAL=0
  ui_rule "$1  ${MODULE_TITLES[$1]:-}"
  log info "module $1 begin: ${MODULE_TITLES[$1]:-}"
}

module_finish() {
  local id="$1" status="PASS"
  if ((MODULE_FAILED)); then
    status="FAIL"
  elif ((MODULE_PARTIAL)); then
    status="PARTIAL"
  fi
  MODULE_STATUS[$id]="$status"
  log info "module $id end: $status"
  status_save
}

verify_module() {
  local id="$1" fn checks=()
  mapfile -t checks < <(compgen -A function "verify_${id}_" | sort)
  for fn in "${checks[@]}"; do
    "$fn"
  done
}

run_module() {
  local id="$1"
  module_begin "$id"
  if declare -F "module_$id" >/dev/null; then
    "module_$id"
  fi
  verify_module "$id"
  module_finish "$id"
}

status_save() {
  local id tmp
  mkdir -p "$STATE_DIR" || return 1
  tmp="$(mktemp "$STATUS_FILE.XXXXXX")" || return 1
  {
    [[ -f $STATUS_FILE ]] && awk -F'|' 'NR == FNR { seen[$1] = 1; next } !($1 in seen)' \
      <(for id in "${!MODULE_STATUS[@]}"; do printf '%s|\n' "$id"; done) "$STATUS_FILE"
    for id in "${!MODULE_STATUS[@]}"; do
      printf '%s|%s|%s\n' "$id" "${MODULE_STATUS[$id]}" "$(date '+%F %T')"
    done
  } | sort >"$tmp"
  chmod 0644 "$tmp"
  mv "$tmp" "$STATUS_FILE"
}

status_of() {
  [[ -f $STATUS_FILE ]] || return 1
  awk -F'|' -v id="$1" '$1 == id { print $2 "|" $3 }' "$STATUS_FILE"
}
