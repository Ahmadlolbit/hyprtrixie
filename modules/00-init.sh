register_module 00 "Setup screens and logging"

init_record_run() {
  local commit
  commit="$(git -C "$HYPRTRIXIE_ROOT" rev-parse --short HEAD 2>/dev/null || printf 'unknown')"
  log info "hyprtrixie $HYPRTRIXIE_VERSION ($commit) from $HYPRTRIXIE_ROOT"
  log info "admin user: ${TARGET_USER:-not chosen yet}"
  config_show | while IFS= read -r line; do log info "config $line"; done
}

init_logs_writable() {
  [[ -n $LOG_FILE && -w $LOG_FILE ]] || return 1
  [[ -z $USER_LOG || -w $USER_LOG ]]
}

module_00() {
  run_step "Install gum for the setup screens" apt_install gum
  run_step "Record what this run uses" init_record_run
}

verify_00_logs() {
  run_check "Logs are being written" init_logs_writable
}

verify_00_gum() {
  run_check "gum is installed" pkg_installed gum
}
