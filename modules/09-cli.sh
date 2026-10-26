register_module 09 "Command-line tools"

CLI_COMMANDS=(batcat fdfind rg fzf eza btop lazygit fastfetch grim slurp wl-copy swappy brightnessctl upower ufw restic)

cli_names() {
  mkdir -p "$BIN_DIR" || return 1
  [[ -x /usr/bin/batcat && ! -e $BIN_DIR/bat ]] && ln -s /usr/bin/batcat "$BIN_DIR/bat"
  [[ -x /usr/bin/fdfind && ! -e $BIN_DIR/fd ]] && ln -s /usr/bin/fdfind "$BIN_DIR/fd"
  return 0
}

lazydocker_install() {
  local dir
  release_current lazydocker && [[ -x $BIN_DIR/lazydocker ]] && return 0
  fetch_release lazydocker || return 1
  dir="$(mktemp -d)" || return 1
  tar -xzf "$FETCHED_PATH" -C "$dir" lazydocker || return 1
  install -D -m 0755 "$dir/lazydocker" "$BIN_DIR/lazydocker" || return 1
  rm -rf "$dir"
  release_mark lazydocker
}

module_09() {
  run_step "Install command-line tools" apt_install_list cli
  run_step "Add short names for bat and fd" cli_names
  run_step "Install lazydocker" lazydocker_install
}

verify_09_cli() {
  run_check "Command-line tools are available" commands_present "${CLI_COMMANDS[@]}" bat fd
}

verify_09_lazydocker() {
  run_check "lazydocker is installed" commands_present lazydocker
}
