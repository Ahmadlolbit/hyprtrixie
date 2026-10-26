register_module 11 "Developer tools"

option DEV_TOOLS --section dev --label "Developer tools" --type multi \
  --values "docker@developer podman@developer mise@developer python@developer rust@developer go@developer kubernetes@developer neovim@developer vscode@developer vscodium@developer zed@developer toolbox@developer"
option CONTAINER_ENGINE --section dev --label "Engine behind the docker command" --type choice \
  --default docker --values "docker podman" --when both_engines
option DOCKER_PORTS --section dev --label "Docker publishes ports on" --type choice \
  --default localhost --values "localhost all" --when docker_wanted

tool_wanted() {
  option_enabled DEV_TOOLS "$1"
}

docker_wanted() {
  tool_wanted docker
}

both_engines() {
  tool_wanted docker && tool_wanted podman
}

docker_install() {
  extrepo_try docker-ce || return 1
  apt_install_list dev-docker || return 1
  deploy_file "$HYPRTRIXIE_ROOT/system/docker/daemon-$DOCKER_PORTS.json" /etc/docker/daemon.json || return 1
  if ((DEPLOY_CHANGED)) && service_active docker.service; then
    systemctl restart docker.service || return 1
  fi
  user_add_groups docker
}

docker_ready() {
  commands_present docker && user_in_group docker
}

podman_install() {
  apt_install_list dev-podman || return 1
  grep -q "^$TARGET_USER:" /etc/subuid 2>/dev/null && grep -q "^$TARGET_USER:" /etc/subgid 2>/dev/null && return 0
  usermod --add-subuids 100000-165535 --add-subgids 100000-165535 "$TARGET_USER"
}

engine_wanted() {
  if tool_wanted podman && { ! tool_wanted docker || [[ $CONTAINER_ENGINE == podman ]]; }; then
    printf 'podman'
  else
    printf 'docker'
  fi
}

engine_install() {
  local engine
  engine="$(engine_wanted)"
  deploy_file "$HYPRTRIXIE_ROOT/bin/engine-switch" "$BIN_DIR/engine-switch" 0755 || return 1
  if [[ "$("$BIN_DIR/engine-switch" status | awk '/^engine:/ { print $2 }')" == "$engine" && -e $BIN_DIR/docker ]]; then
    return 0
  fi
  SUDO_USER="$TARGET_USER" "$BIN_DIR/engine-switch" "$engine"
}

engine_ready() {
  [[ "$("$BIN_DIR/engine-switch" status | awk '/^engine:/ { print $2 }')" == "$(engine_wanted)" ]]
}

mise_install() {
  extrepo_try mise || return 1
  apt_install_list dev-mise || return 1
  as_user mise use --global node@lts
}

rust_install() {
  apt_install_list dev-rust || return 1
  as_user rustup default stable
}

toolchains_ready() {
  tool_wanted mise && ! as_user mise exec -- node --version >/dev/null 2>&1 && return 1
  tool_wanted python && ! commands_present pyenv && return 1
  tool_wanted rust && ! as_user rustc --version >/dev/null 2>&1 && return 1
  tool_wanted go && ! commands_present go && return 1
  return 0
}

kubernetes_install() {
  extrepo_try kubernetes || return 1
  extrepo_try helm || return 1
  apt_install_list dev-kubernetes || return 1
  deb_install_release k9s k9s || return 1
  if ! release_current kind || [[ ! -x $BIN_DIR/kind ]]; then
    fetch_release kind || return 1
    install -D -m 0755 "$FETCHED_PATH" "$BIN_DIR/kind" || return 1
    release_mark kind
  fi
}

neovim_install() {
  local dir tmp
  release_get neovim || return 1
  dir="/opt/nvim-${RELEASE_VERSION#v}"
  if [[ ! -x $dir/bin/nvim ]]; then
    fetch_release neovim || return 1
    tmp="$(mktemp -d)" || return 1
    tar -xzf "$FETCHED_PATH" -C "$tmp" || return 1
    rm -rf "$dir"
    mv "$tmp/nvim-linux-x86_64" "$dir" || return 1
    rm -rf "$tmp"
  fi
  ln -sfn "$dir/bin/nvim" "$BIN_DIR/nvim" || return 1
  deploy_file "$dir/share/applications/nvim.desktop" /usr/local/share/applications/nvim.desktop || return 1
  apt_install_list dev-neovim
}

lazyvim_setup() {
  local config
  config="$(user_path .config/nvim)"
  [[ -e $config ]] && return 0
  fetch_git_cached lazyvim-starter || return 1
  user_mkdir .config || return 1
  cp -a "$FETCHED_PATH" "$config" || return 1
  rm -rf "$config/.git"
  chown -R "$TARGET_USER:" "$config"
}

lazyvim_plugins() {
  as_user "$BIN_DIR/nvim" --headless "+Lazy! sync" +qa
}

neovim_ready() {
  "$BIN_DIR/nvim" --version 2>/dev/null | head -n1 | grep -qE 'NVIM v(0\.(1[1-9]|[2-9][0-9])|[1-9])'
}

vscode_install() {
  extrepo_try vscode || return 1
  apt_install_list dev-vscode
}

vscodium_install() {
  extrepo_try vscodium || return 1
  apt_install_list dev-vscodium
}

zed_install() {
  local app tmp desktop
  app="$(user_path .local/zed.app)"
  if ! release_current zed || [[ ! -x $app/bin/zed ]]; then
    fetch_release zed || return 1
    tmp="$(mktemp -d)" || return 1
    tar -xzf "$FETCHED_PATH" -C "$tmp" || return 1
    rm -rf "$app"
    user_mkdir .local || return 1
    mv "$tmp/zed.app" "$app" || return 1
    rm -rf "$tmp"
    chown -R "$TARGET_USER:" "$app" || return 1
    release_mark zed
  fi
  user_mkdir .local/bin || return 1
  as_user ln -sfn "$app/bin/zed" "$(user_path .local/bin/zed)" || return 1
  user_mkdir .local/share/applications || return 1
  desktop="$(user_path .local/share/applications/dev.zed.Zed.desktop)"
  sed -e "s|Exec=zed|Exec=$app/bin/zed|g" -e "s|Icon=zed|Icon=$app/share/icons/hicolor/512x512/apps/zed.png|g" \
    "$app/share/applications/dev.zed.Zed.desktop" >"$desktop" || return 1
  chown "$TARGET_USER:" "$desktop"
}

editors_ready() {
  tool_wanted vscode && ! commands_present code && return 1
  tool_wanted vscodium && ! commands_present codium && return 1
  tool_wanted zed && [[ ! -x "$(user_path .local/zed.app/bin/zed)" ]] && return 1
  return 0
}

toolbox_install() {
  local dir=/opt/jetbrains-toolbox tmp
  if ! release_current toolbox || [[ ! -x $dir/bin/jetbrains-toolbox ]]; then
    fetch_release toolbox || return 1
    tmp="$(mktemp -d)" || return 1
    tar -xzf "$FETCHED_PATH" -C "$tmp" || return 1
    rm -rf "$dir"
    mv "$tmp"/jetbrains-toolbox-* "$dir" || return 1
    rm -rf "$tmp"
    release_mark toolbox
  fi
  ln -sfn "$dir/bin/jetbrains-toolbox" "$BIN_DIR/jetbrains-toolbox" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/applications/jetbrains-toolbox.desktop" /usr/local/share/applications/jetbrains-toolbox.desktop
}

module_11() {
  tool_wanted docker && run_step "Install Docker" docker_install
  tool_wanted podman && run_step "Install Podman" podman_install
  { tool_wanted docker || tool_wanted podman; } && run_step "Set up engine-switch" engine_install
  tool_wanted mise && run_step "Install mise with Node.js LTS" mise_install
  tool_wanted python && run_step "Install pyenv" apt_install_list dev-python
  tool_wanted rust && run_step "Install Rust (stable)" rust_install
  tool_wanted go && run_step "Install Go" apt_install_list dev-go
  tool_wanted kubernetes && run_step "Install kubectl, helm, k9s and kind" kubernetes_install
  if tool_wanted neovim; then
    run_step "Install Neovim" neovim_install
    run_step "Set up LazyVim" lazyvim_setup
    run_step "Install LazyVim plugins" lazyvim_plugins
  fi
  tool_wanted vscode && run_step "Install Visual Studio Code" vscode_install
  tool_wanted vscodium && run_step "Install VSCodium" vscodium_install
  tool_wanted zed && run_step "Install Zed" zed_install
  tool_wanted toolbox && run_step "Install JetBrains Toolbox" toolbox_install
  return 0
}

verify_11_docker() {
  docker_wanted || return 0
  run_check "Docker works and you are in the docker group" docker_ready
}

verify_11_podman() {
  tool_wanted podman || return 0
  run_check "Podman is installed" commands_present podman
}

verify_11_engine() {
  tool_wanted docker || tool_wanted podman || return 0
  run_check "engine-switch points docker at $(engine_wanted)" engine_ready
}

verify_11_toolchains() {
  run_check "Language toolchains are ready" toolchains_ready
}

verify_11_kubernetes() {
  tool_wanted kubernetes || return 0
  run_check "kubectl, helm, k9s and kind are installed" commands_present kubectl helm k9s kind
}

verify_11_neovim() {
  tool_wanted neovim || return 0
  run_check "Neovim 0.11 or newer is installed" neovim_ready
}

verify_11_editors() {
  run_check "Code editors are installed" editors_ready
}

verify_11_toolbox() {
  tool_wanted toolbox || return 0
  run_check "JetBrains Toolbox is installed" commands_present jetbrains-toolbox
}
