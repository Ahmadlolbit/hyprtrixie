register_module 08 "Terminal and shell"

module_08() {
  run_step "Install terminal packages" apt_install_list terminal
}

verify_08_terminal() {
  run_check "kitty, zsh and tmux are installed" commands_present kitty zsh tmux
}
