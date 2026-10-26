register_module 11 "Developer tools"

option DEV_TOOLS --section dev --label "Developer tools" --type multi \
  --values "docker@developer podman@developer mise@developer python@developer rust@developer go@developer"
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

module_11() {
  tool_wanted docker && run_step "Install Docker" docker_install
  tool_wanted podman && run_step "Install Podman" podman_install
  { tool_wanted docker || tool_wanted podman; } && run_step "Set up engine-switch" engine_install
  tool_wanted mise && run_step "Install mise with Node.js LTS" mise_install
  tool_wanted python && run_step "Install pyenv" apt_install_list dev-python
  tool_wanted rust && run_step "Install Rust (stable)" rust_install
  tool_wanted go && run_step "Install Go" apt_install_list dev-go
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
