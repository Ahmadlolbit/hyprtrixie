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
  local simulate=()
  apt_env
  [[ ${APT_SIMULATE:-0} == 1 ]] && simulate=(-s)
  apt-get -y "${simulate[@]}" -o Dpkg::Use-Pty=0 \
    -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold "$@"
}

apt_mark_dirty() {
  APT_UPDATE_NEEDED=1
}

apt_update() {
  local age=999999
  [[ ${APT_SIMULATE:-0} == 1 ]] && return 0
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
  local status
  status="$(dpkg-query -W -f='${Status}\n' "$1" 2>/dev/null)"
  [[ $'\n'$status$'\n' == *$'\n'"install ok installed"$'\n'* ]]
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

backports_siblings() {
  local pkg
  sed -nE 's/^ *([a-z0-9][a-z0-9.+-]*) : Depends: [a-z0-9.+-]+ \(= [^)]+\) but ([^ ]*~bpo[^ ]* is to be installed|it is not going to be installed).*/\1/p' | sort -u | while IFS= read -r pkg; do
    [[ -n "$(pkg_backports_version "$pkg")" ]] && printf '%s\n' "$pkg"
  done
  return 0
}

apt_install_stable() {
  local extra=() more=() out rc attempt
  for attempt in 1 2 3 4 5; do
    out="$(apt_get install "$@" "${extra[@]}" 2>&1)"
    rc=$?
    printf '%s\n' "$out"
    ((rc == 0)) && return 0
    mapfile -t more < <(backports_siblings <<<"$out")
    ((${#more[@]})) || return "$rc"
    log warn "attempt $attempt: taking ${more[*]} from $BACKPORTS_SUITE to match libraries already installed from there"
    extra+=("${more[@]/%//$BACKPORTS_SUITE}")
  done
  return 1
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
    apt_install_stable "${stable[@]}" || return 1
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

commands_present() {
  local name missing=()
  for name in "$@"; do
    command -v "$name" >/dev/null 2>&1 || missing+=("$name")
  done
  ((${#missing[@]} == 0)) && return 0
  printf 'missing: %s\n' "${missing[*]}"
  return 1
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

pkg_candidate() {
  apt-cache policy "$1" 2>/dev/null | awk '/Candidate:/ { if ($2 != "(none)") print $2; exit }'
}

extrepo_release_url() {
  local file="/etc/apt/sources.list.d/extrepo_$1.sources" uri suite
  [[ -f $file ]] || return 1
  uri="$(awk '/^URIs:/ { print $2; exit }' "$file")"
  suite="$(awk '/^Suites:/ { print $2; exit }' "$file")"
  if [[ $suite == */ ]]; then
    printf '%s/%sInRelease' "${uri%/}" "${suite#./}"
  else
    printf '%s/dists/%s/InRelease' "${uri%/}" "$suite"
  fi
}

extrepo_try() {
  local name="$1" url
  extrepo_enable "$name" || return 1
  apt_update --force && return 0
  url="$(extrepo_release_url "$name")" || return 1
  if curl -fsS --max-time 20 -o /dev/null "$url"; then
    return 1
  fi
  log warn "the $name repository can't be reached from here, turning it off"
  extrepo disable "$name" || true
  apt_update --force
  return 2
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
