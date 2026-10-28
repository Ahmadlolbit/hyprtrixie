register_module 10 "Applications"

APP_CATEGORIES=("browsers|Browsers" "chat|Chat and calls" "office|Office and notes" "viewers|Viewers"
  "files|Files and sharing" "security|Passwords and VPN" "creative|Creative" "media|Audio and video"
  "system|System tools" "games|Games")

app_field() {
  local line fields=()
  line="$(manifest_line apps "$1")" || return 1
  read -r -a fields <<<"$line"
  printf '%s' "${fields[$2 - 1]}"
}

app_name() {
  local line
  line="$(manifest_line apps "$1")" || return 1
  read -r _ _ _ _ _ line <<<"$line"
  printf '%s' "$line"
}

app_category() {
  app_field "$1" 2
}

app_source() {
  app_field "$1" 4
}

app_packages() {
  local packages
  packages="$(app_field "$1" 5)" || return 1
  [[ -n $packages && $packages != - ]] || return 0
  tr ',' '\n' <<<"$packages"
}

apps_values() {
  [[ -f $MANIFEST_DIR/apps ]] || return 0
  awk 'NF >= 6 { printf "%s%s@%s", sep, $1, $3; sep = " " }' "$MANIFEST_DIR/apps"
}

apps_categories() {
  local entry
  for entry in "${APP_CATEGORIES[@]}"; do
    awk -v c="${entry%%|*}" '$2 == c { found = 1; exit } END { exit !found }' "$MANIFEST_DIR/apps" 2>/dev/null &&
      printf '%s\t%s\n' "${entry%%|*}" "${entry#*|}"
  done
  return 0
}

option APPS --section apps --label "Applications" --type multi --values "$(apps_values)" \
  --groups apps_categories --group-of app_category --names app_name \
  --help "Apps to install, by category; untick any you don't want"
option TEAMS_SOURCE --section apps --label "Install Teams from" --type choice \
  --default apt --values "apt snap" --when teams_wanted \
  --help "Teams for Linux from its apt repository or from the Snap Store"

app_wanted() {
  option_enabled APPS "$1"
}

teams_wanted() {
  app_wanted teams
}

helium_default() {
  as_user xdg-mime default helium.desktop x-scheme-handler/http x-scheme-handler/https text/html application/xhtml+xml
}

helium_is_default() {
  [[ "$(as_user xdg-mime query default x-scheme-handler/https)" == helium.desktop ]]
}

extrepo_app_install() {
  local id="$1" repo="$2" rc=0
  shift 2
  packages_present "$@" && return 0
  extrepo_try "$repo" || rc=$?
  if ((rc == 0)) && [[ -n "$(pkg_candidate "${1%/backports}")" ]]; then
    apt_install "$@"
    return
  fi
  manifest_line releases "$id" >/dev/null || return 1
  printf 'The %s repository is unavailable, installing the pinned package instead\n' "$repo"
  deb_install_release "$id" "$1"
}

webapp_install() {
  local id="$1"
  pkg_installed helium-bin || app_install helium || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/applications/$id-web.svg" "$SHARE_DIR/icons/$id-web.svg" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/applications/$id-web.desktop" "/usr/local/share/applications/$id-web.desktop"
}

teams_install() {
  if [[ $TEAMS_SOURCE == snap ]]; then
    apt_install snapd || return 1
    snap wait system seed.loaded || return 1
    snap list teams-for-linux >/dev/null 2>&1 || snap install teams-for-linux
    return
  fi
  extrepo_try teams-for-linux || return 1
  apt_install teams-for-linux
}

teams_present() {
  if [[ $TEAMS_SOURCE == snap ]]; then
    snap list teams-for-linux >/dev/null 2>&1
  else
    pkg_installed teams-for-linux
  fi
}

app_install() {
  local id="$1" source packages=()
  source="$(app_source "$id")" || {
    log error "no app named $id in the catalogue"
    return 1
  }
  mapfile -t packages < <(app_packages "$id")
  case "$source" in
    debian) apt_install "${packages[@]}" ;;
    extrepo:*) extrepo_app_install "$id" "${source#extrepo:}" "${packages[@]}" ;;
    release:*) deb_install_release "${source#release:}" "${packages[0]}" ;;
    webapp) webapp_install "$id" ;;
    teams) teams_install ;;
    *)
      log error "unknown source $source for $id"
      return 1
      ;;
  esac
}

app_present() {
  local id="$1" source packages=()
  source="$(app_source "$id")" || return 1
  mapfile -t packages < <(app_packages "$id")
  case "$source" in
    webapp) [[ -f /usr/local/share/applications/$id-web.desktop ]] ;;
    teams) teams_present ;;
    *) packages_present "${packages[@]}" ;;
  esac
}

apps_missing() {
  local id missing=()
  for id in $(option_items APPS); do
    app_wanted "$id" || continue
    app_present "$id" || missing+=("$id")
  done
  ((${#missing[@]} == 0)) && return 0
  printf 'not installed: %s\n' "${missing[*]}"
  return 1
}

module_10() {
  local id
  for id in $(option_items APPS); do
    app_wanted "$id" || continue
    run_step "Install $(app_name "$id")" app_install "$id"
  done
  app_wanted helium && run_step "Make Helium the default browser" helium_default
  return 0
}

verify_10_apps() {
  run_check "Every ticked app is installed" apps_missing
}

verify_10_helium() {
  app_wanted helium || return 0
  run_check "Helium is the default browser" helium_is_default
}
