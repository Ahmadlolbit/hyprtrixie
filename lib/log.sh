LOG_DIR="${HYPRTRIXIE_LOG_DIR:-/var/log/hyprtrixie}"
LOG_FILE=""
USER_LOG=""

log_init() {
  local stamp
  stamp="$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$LOG_DIR" || return 1
  chmod 0755 "$LOG_DIR"
  LOG_FILE="$LOG_DIR/run-$stamp.log"
  : >>"$LOG_FILE" || return 1
  ln -sfn "${LOG_FILE##*/}" "$LOG_DIR/latest.log"
  log info "hyprtrixie ${HYPRTRIXIE_VERSION:-dev} started ($*)"
}

log_attach_user() {
  local home="$1" owner="$2" dir
  dir="$home/runbook-logs"
  mkdir -p "$dir" || return 1
  USER_LOG="$dir/runbook.log"
  {
    printf '\n'
    [[ -n $LOG_FILE ]] && cat "$LOG_FILE"
  } >>"$USER_LOG" || return 1
  chown -R "$owner:" "$dir" 2>/dev/null || true
}

log_write() {
  [[ -n $LOG_FILE ]] && printf '%s\n' "$1" >>"$LOG_FILE"
  [[ -n $USER_LOG ]] && printf '%s\n' "$1" >>"$USER_LOG"
  return 0
}

log() {
  local level="$1" line
  shift
  printf -v line '%s [%-5s] %s' "$(date '+%F %T')" "$level" "$*"
  log_write "$line"
}

log_stream() {
  if [[ -n $LOG_FILE && -n $USER_LOG ]]; then
    tee -a "$LOG_FILE" >>"$USER_LOG"
  elif [[ -n $LOG_FILE ]]; then
    cat >>"$LOG_FILE"
  else
    cat >/dev/null
  fi
}

log_size() {
  if [[ -n $LOG_FILE && -f $LOG_FILE ]]; then
    stat -c %s "$LOG_FILE"
  else
    printf '0'
  fi
}

log_tail_since() {
  local offset="$1" lines="${2:-20}"
  [[ -n $LOG_FILE && -f $LOG_FILE ]] || return 0
  tail -c +"$((offset + 1))" "$LOG_FILE" | tail -n "$lines"
}
