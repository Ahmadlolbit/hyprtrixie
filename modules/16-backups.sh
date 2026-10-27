register_module 16 "Backups"

backup_helper_install() {
  apt_install restic || return 1
  deploy_user_file "$HYPRTRIXIE_ROOT/bin/restic-backup" bin/restic-backup 0755
}

module_16() {
  run_step "Install the restic-backup helper" backup_helper_install
}

verify_16_backups() {
  run_check "restic-backup is in ~/bin" test -x "$(user_path bin/restic-backup)"
}
