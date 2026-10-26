register_module 08 "Terminal and shell"

FONT_DIR="${HYPRTRIXIE_FONT_DIR:-/usr/local/share/fonts/jetbrains-mono-nerd}"

nerd_font_install() {
  local dir
  release_current jetbrainsmono-nerd && compgen -G "$FONT_DIR/*.ttf" >/dev/null && return 0
  fetch_release jetbrainsmono-nerd || return 1
  dir="$(mktemp -d)" || return 1
  tar -xJf "$FETCHED_PATH" -C "$dir" || return 1
  mkdir -p "$FONT_DIR" || return 1
  find "$dir" -name 'JetBrainsMonoNerdFont*.ttf' -exec install -m 0644 -t "$FONT_DIR" {} + || return 1
  rm -rf "$dir"
  fc-cache -f "$FONT_DIR" >/dev/null || return 1
  release_mark jetbrainsmono-nerd
}

nerd_font_present() {
  fc-list 2>/dev/null | grep -qi 'JetBrainsMono Nerd Font'
}

module_08() {
  run_step "Install terminal packages" apt_install_list terminal
  run_step "Install JetBrains Mono Nerd Font" nerd_font_install
}

verify_08_terminal() {
  run_check "kitty, zsh and tmux are installed" commands_present kitty zsh tmux
}

verify_08_font() {
  run_check "JetBrains Mono Nerd Font is installed" nerd_font_present
}
