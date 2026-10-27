APT_STAMP="${HYPRTRIXIE_STATE_DIR:-/var/lib/hyprtrixie}/apt-updated"
BACKPORTS_SUITE="trixie-backports"
EXTREPO_CONFIG="/etc/extrepo/config.yaml"
APT_UPDATE_NEEDED=0

apt_env() {
  export DEBIAN_FRONTEND=noninteractive
  export NEEDRESTART_MODE=a NEEDRESTART_SUSPEND=1
  export APT_LISTCHANGES_FRONTEND=none
}

apt_get() {
  apt_env
  apt-get -y -o Dpkg::Use-Pty=0 \
    -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold "$@"
}

apt_mark_dirty() {
  APT_UPDATE_NEEDED=1
}

apt_update() {
  local age=999999
  if [[ -f $APT_STAMP ]]; then
    age=$(($(date +%s) - $(stat -c %Y "$APT_STAMP")))
  fi
  if [[ ${1:-} != "--force" ]] && ((APT_UPDATE_NEEDED == 0 && age < 3600)); then
    return 0
  fi
  apt_get update || return 1
  mkdir -p "${APT_STAMP%/*}" && touch "$APT_STAMP"
  APT_UPDATE_NEEDED=0
}

pkg_installed() {
  [[ "$(dpkg-query -W -f='${Status}' "$1" 2>/dev/null)" == "install ok installed" ]]
}

pkg_version() {
  dpkg-query -W -f='${Version}' "$1" 2>/dev/null
}

pkg_backports_version() {
  apt-cache madison "$1" 2>/dev/null |
    awk -F'|' -v suite="$BACKPORTS_SUITE" '$3 ~ suite { gsub(/ /, "", $2); print $2; exit }'
}

pkg_needs_backports() {
  local name="$1" wanted current
  pkg_installed "$name" || return 0
  wanted="$(pkg_backports_version "$name")"
  [[ -n $wanted ]] || return 1
  current="$(pkg_version "$name")"
  dpkg --compare-versions "$current" lt "$wanted"
}

apt_install() {
  local spec stable=() backports=()
  ((APT_UPDATE_NEEDED)) && { apt_update || return 1; }
  for spec in "$@"; do
    if [[ $spec == */backports ]]; then
      pkg_needs_backports "${spec%/backports}" && backports+=("${spec%/backports}")
    elif ! pkg_installed "$spec"; then
      stable+=("$spec")
    fi
  done
  if ((${#stable[@]} + ${#backports[@]} == 0)); then
    log info "already installed: $*"
    return 0
  fi
  apt_update || return 1
  if ((${#stable[@]})); then
    log info "installing: ${stable[*]}"
    apt_get install "${stable[@]}" || return 1
  fi
  if ((${#backports[@]})); then
    log info "installing from $BACKPORTS_SUITE: ${backports[*]}"
    apt_get install -t "$BACKPORTS_SUITE" "${backports[@]}" || return 1
  fi
}

pkg_list() {
  local file="$MANIFEST_DIR/packages/$1.list"
  if [[ ! -f $file ]]; then
    log error "unknown package list: $1"
    return 1
  fi
  grep -v '^[[:space:]]*$' "$file"
}

apt_install_list() {
  local list pkgs=() more=()
  for list in "$@"; do
    mapfile -t more < <(pkg_list "$list")
    ((${#more[@]})) || return 1
    pkgs+=("${more[@]}")
  done
  apt_install "${pkgs[@]}"
}

packages_present() {
  local spec
  for spec in "$@"; do
    pkg_installed "${spec%/backports}" || return 1
  done
}

list_present() {
  local list pkgs=()
  for list in "$@"; do
    mapfile -t pkgs < <(pkg_list "$list")
    packages_present "${pkgs[@]}" || return 1
  done
}

extrepo_policy() {
  local policy="$1"
  [[ -f $EXTREPO_CONFIG ]] || return 1
  grep -qE "^- $policy\$" "$EXTREPO_CONFIG" && return 0
  if grep -qE "^# *- $policy\$" "$EXTREPO_CONFIG"; then
    sed -i -E "s/^# *- $policy\$/- $policy/" "$EXTREPO_CONFIG"
  else
    printf -- '- %s\n' "$policy" >>"$EXTREPO_CONFIG"
  fi
}

extrepo_enabled() {
  local file="/etc/apt/sources.list.d/extrepo_$1.sources"
  [[ -f $file ]] && ! grep -q '^Enabled: no' "$file"
}

extrepo_enable() {
  local name
  apt_install extrepo || return 1
  for name in "$@"; do
    extrepo_enabled "$name" && continue
    extrepo enable "$name" || return 1
    apt_mark_dirty
  done
}

deb_install_release() {
  local release="$1" package="$2"
  release_get "$release" || return 1
  if pkg_installed "$package" && [[ "$(pkg_version "$package")" == "$RELEASE_VERSION"* ]]; then
    log info "$package $RELEASE_VERSION already installed"
    return 0
  fi
  fetch_release "$release" || return 1
  apt_update || return 1
  apt_get install "$FETCHED_PATH"
}
