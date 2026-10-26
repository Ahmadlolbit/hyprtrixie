register_module 11 "Developer tools"

option DEV_TOOLS --section dev --label "Developer tools" --type multi \
  --values "docker@developer"
option DOCKER_PORTS --section dev --label "Docker publishes ports on" --type choice \
  --default localhost --values "localhost all" --when docker_wanted

tool_wanted() {
  option_enabled DEV_TOOLS "$1"
}

docker_wanted() {
  tool_wanted docker
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

module_11() {
  tool_wanted docker && run_step "Install Docker" docker_install
  return 0
}

verify_11_docker() {
  docker_wanted || return 0
  run_check "Docker works and you are in the docker group" docker_ready
}
