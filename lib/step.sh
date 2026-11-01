STATE_DIR="${HYPRTRIXIE_STATE_DIR:-/var/lib/hyprtrixie}"
STATUS_FILE="$STATE_DIR/status"
STEP_KIND="step"
CURRENT_MODULE=""
MODULE_FAILED=0
MODULE_PARTIAL=0
MODULE_SKIPPED=0
MODULE_POSITION=0
MODULE_TOTAL=0
MODULE_IDS=()
STEP_FAILURES=()
STEP_SKIPS=()
STEPS_OK=0
CHECKS_OK=0
RUN_STARTED=$SECONDS
RUN_PACKAGES=""
CORE_RESULT=""
TOOLS_FAILED=0
GO_MODULES=(06 08 09 13 16)
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

step_show_log() {
  local offset="$1" file
  file="$(mktemp)" || return 1
  log_tail_since "$offset" 100000 >"$file"
  ui_pager "$file" >&2
  rm -f "$file"
}

step_choice() {
  local title="$1" offset="${2:-0}" pick
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
  while :; do
    pick="$(ui_choose "\"$title\" failed. What now?" "" Retry Skip Continue Halt "Show log")" || pick="Halt"
    [[ $pick == "Show log" ]] || break
    step_show_log "$offset"
  done
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
      ui_spin_start "$title" "$offset"
      step_exec "$@"
      rc=$?
      ui_spin_stop
    fi
    if ((rc == 0)); then
      if [[ $STEP_KIND == check ]]; then
        CHECKS_OK=$((CHECKS_OK + 1))
      else
        STEPS_OK=$((STEPS_OK + 1))
      fi
      ui_status ok "$title"
      return 0
    fi
    log error "$STEP_KIND failed (exit $rc): $title"
    ui_status fail "$title"
    step_show_output "$offset"
    choice="$(step_choice "$title" "$offset")"
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
  local where=""
  CURRENT_MODULE="$1"
  MODULE_FAILED=0
  MODULE_PARTIAL=0
  MODULE_SKIPPED=0
  ((MODULE_TOTAL)) && where="  ·  $MODULE_POSITION of $MODULE_TOTAL"
  ui_rule "$1  ${MODULE_TITLES[$1]:-}$where"
  log info "module $1 begin: ${MODULE_TITLES[$1]:-}"
}

module_finish() {
  local id="$1" status="PASS"
  if ((MODULE_FAILED)); then
    status="FAIL"
  elif ((MODULE_SKIPPED)); then
    status="SKIP"
  elif ((MODULE_PARTIAL)); then
    status="PARTIAL"
  fi
  MODULE_STATUS[$id]="$status"
  log info "module $id end: $status"
  status_save
}

module_skip() {
  MODULE_SKIPPED=1
  ui_status skip "$*"
  log info "module ${CURRENT_MODULE:-none} skipped: $*"
}

tools_ready() {
  local title="Build the hyprtrixie tools" rc
  ((TOOLS_BUILT)) && return 0
  ((TOOLS_FAILED)) && return 1
  [[ ${RUN_MODE:-} != verify ]] || command -v go >/dev/null || return 1
  log info "step: $title"
  ui_spin_start "$title" "$(log_size)"
  step_exec tools_build
  rc=$?
  ui_spin_stop
  if ((rc == 0)); then
    ui_status ok "$title"
    return 0
  fi
  log warn "the tools could not be built, so this module runs in Bash"
  ui_status warn "The tools could not be built, so this module runs in Bash"
  TOOLS_FAILED=1
  return 1
}

module_in_go() {
  [[ " ${GO_MODULES[*]} " == *" $1 "* ]] || return 1
  tools_ready || return 1
  [[ -x $TOOLS_DIR/hyprtrixie-core ]]
}

core_results() {
  local what kind title
  while IFS='|' read -r what kind title; do
    case "$what" in
      steps) STEPS_OK=$((STEPS_OK + kind)) ;;
      checks) CHECKS_OK=$((CHECKS_OK + kind)) ;;
      failed)
        STEP_FAILURES+=("${CURRENT_MODULE:-none}|$kind|$title")
        MODULE_FAILED=1
        ;;
      skipped)
        STEP_SKIPS+=("${CURRENT_MODULE:-none}|$kind|$title")
        MODULE_PARTIAL=1
        ;;
      module-skipped) MODULE_SKIPPED=1 ;;
    esac
  done <"$CORE_RESULT"
}

core_stopped() {
  log error "$2"
  STEP_FAILURES+=("${CURRENT_MODULE:-none}|$1|$2")
  MODULE_FAILED=1
  step_halt
}

core_module() {
  local rc mode="${FAILURE_MODE:-ask}" kind="${1%s}"
  [[ $mode != ask ]] || ui_can_ask || mode="continue"
  CORE_RESULT="$(mktemp)" || core_stopped "$kind" "cannot make a temporary file for module $2"
  HYPRTRIXIE_ROOT="$HYPRTRIXIE_ROOT" HYPRTRIXIE_LOG_FILE="$LOG_FILE" HYPRTRIXIE_USER_LOG="$USER_LOG" \
    HYPRTRIXIE_MODULE_RESULT="$CORE_RESULT" HYPRTRIXIE_FAILURE_MODE="$mode" HYPRTRIXIE_USER="$TARGET_USER" \
    HYPRTRIXIE_RUN_CONFIG="${RUN_CONFIG:-}" "$TOOLS_DIR/hyprtrixie-core" module "$@"
  rc=$?
  core_results
  rm -f "$CORE_RESULT"
  CORE_RESULT=""
  case "$rc" in
    0) ;;
    1) step_halt ;;
    130) on_interrupt ;;
    *) core_stopped "$kind" "hyprtrixie-core stopped with exit code $rc" ;;
  esac
}

verify_module() {
  local id="$1" fn checks=()
  if module_in_go "$id"; then
    core_module checks "$id"
    return
  fi
  mapfile -t checks < <(compgen -A function "verify_${id}_" | sort)
  for fn in "${checks[@]}"; do
    "$fn"
  done
}

run_module() {
  local id="$1"
  module_begin "$id"
  if module_in_go "$id"; then
    core_module steps "$id"
  elif declare -F "module_$id" >/dev/null; then
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

run_duration() {
  local seconds="$1"
  if ((seconds >= 3600)); then
    printf '%d h %d min' $((seconds / 3600)) $((seconds % 3600 / 60))
  elif ((seconds >= 60)); then
    printf '%d min %d s' $((seconds / 60)) $((seconds % 60))
  else
    printf '%d s' "$seconds"
  fi
}

run_count() {
  local kind="$1" entry count=0
  shift
  for entry in "$@"; do
    [[ $entry == *"|$kind|"* ]] && count=$((count + 1))
  done
  printf '%s' "$count"
}

packages_snapshot() {
  dpkg-query -W -f='${Package}\n' 2>/dev/null | sort
}

run_new_packages() {
  [[ -n $RUN_PACKAGES && -f $RUN_PACKAGES ]] || return 0
  comm -13 "$RUN_PACKAGES" <(packages_snapshot)
}

run_summary_lines() {
  local added=() updated=0 kept=0
  mapfile -t added < <(run_new_packages)
  if [[ -n $LOG_FILE && -f $LOG_FILE ]]; then
    updated="$(grep -c '\] deployed ' "$LOG_FILE")"
    kept="$(grep -c 'kept local edits in ' "$LOG_FILE")"
  fi
  printf 'Took %s\n' "$(run_duration $((SECONDS - RUN_STARTED)))"
  printf 'Steps: %s passed, %s skipped, %s failed\n' "$STEPS_OK" \
    "$(run_count step "${STEP_SKIPS[@]}")" "$(run_count step "${STEP_FAILURES[@]}")"
  printf 'Checks: %s passed, %s failed\n' "$CHECKS_OK" "$(run_count check "${STEP_FAILURES[@]}")"
  printf 'Packages installed: %s\n' "${#added[@]}"
  printf 'Files updated: %s, kept with your edits: %s\n' "$updated" "$kept"
  if [[ " ${added[*]} " == *" linux-image-"* ]]; then
    printf 'Next: restart to start the new kernel\n'
  elif [[ -n ${MODULE_STATUS[12]:-}${MODULE_STATUS[13]:-}${MODULE_STATUS[14]:-} ]]; then
    printf 'Next: log out and back in to start the updated desktop\n'
  fi
  ((kept)) && printf 'Files you edited kept your edits; the new versions are next to them as *.hyprtrixie-new\n'
  ((${#STEP_FAILURES[@]})) && printf 'Something failed: run sudo hyprtrixie collect --verbose and send the file it prints\n'
  return 0
}
