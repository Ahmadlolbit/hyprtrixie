register_module 08 "Terminal and shell"

FONT_DIR="${HYPRTRIXIE_FONT_DIR:-/usr/local/share/fonts/jetbrains-mono-nerd}"
ZSH_SHARE="$SHARE_DIR/zsh"

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

zsh_setup() {
  local zshrc
  fetch_git oh-my-zsh "$SHARE_DIR/oh-my-zsh" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/zsh/hyprtrixie.zsh" "$ZSH_SHARE/hyprtrixie.zsh" || return 1
  zshrc="$(user_path .zshrc)"
  if [[ -e $zshrc ]]; then
    ensure_line "$zshrc" "source $ZSH_SHARE/hyprtrixie.zsh" || return 1
  else
    deploy_user_once "$HYPRTRIXIE_ROOT/config/zsh/zshrc" .zshrc || return 1
  fi
  [[ "$(user_shell)" == /usr/bin/zsh ]] || chsh -s /usr/bin/zsh "$TARGET_USER"
}

zsh_ready() {
  [[ "$(user_shell)" == /usr/bin/zsh ]] && grep -qF "$ZSH_SHARE/hyprtrixie.zsh" "$(user_path .zshrc)"
}

module_08() {
  run_step "Install terminal packages" apt_install_list terminal
  run_step "Install JetBrains Mono Nerd Font" nerd_font_install
  run_step "Set up zsh with Oh My Zsh" zsh_setup
}

verify_08_terminal() {
  run_check "kitty, zsh and tmux are installed" commands_present kitty zsh tmux
}

verify_08_font() {
  run_check "JetBrains Mono Nerd Font is installed" nerd_font_present
}

verify_08_zsh() {
  run_check "zsh is the login shell" zsh_ready
}
