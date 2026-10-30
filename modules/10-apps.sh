register_module 10 "Applications"

APPLICATIONS_DIR="${HYPRTRIXIE_APPLICATIONS_DIR:-/usr/share/applications}"
APPS_NEW_IN_2="imv evince obsidian localsend calculator"
APP_CATEGORIES=("browsers|Browsers" "chat|Chat and calls" "office|Office and notes" "viewers|Viewers"
  "files|Files and sharing" "security|Passwords and VPN" "creative|Creative" "media|Audio and video"
  "system|System tools" "games|Games")
IMAGE_TYPES=(image/png image/jpeg image/gif image/webp image/bmp image/tiff image/svg+xml image/avif image/heif)
VIDEO_TYPES=(video/mp4 video/x-matroska video/webm video/quicktime video/x-msvideo video/mpeg video/ogg)
AUDIO_TYPES=(audio/mpeg audio/ogg audio/flac audio/x-wav audio/mp4 audio/aac audio/opus)
ARCHIVE_TYPES=(application/zip application/x-tar application/gzip application/x-compressed-tar
  application/x-7z-compressed application/vnd.rar application/x-rar application/x-xz
  application/x-xz-compressed-tar application/zstd application/x-bzip2)

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

apps_add_new_defaults() {
  local id defaults
  defaults=" $(option_default APPS) "
  for id in $APPS_NEW_IN_2; do
    [[ $defaults == *" $id "* ]] || continue
    app_wanted "$id" || APPS="${APPS:+$APPS }$id"
  done
}

config_migration 2 apps_add_new_defaults

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

app_repo_in_use() {
  local repo="$1" skip="$2" id
  for id in $(option_items APPS); do
    [[ $id == "$skip" ]] && continue
    [[ "$(app_source "$id")" == "extrepo:$repo" ]] || continue
    app_present "$id" && return 0
  done
  return 1
}

app_remove() {
  local id="$1" source packages=() installed=() package repo=""
  source="$(app_source "$id")" || {
    log error "no app named $id in the catalogue"
    return 1
  }
  case "$source" in
    webapp)
      rm -f "/usr/local/share/applications/$id-web.desktop" "$SHARE_DIR/icons/$id-web.svg"
      return 0
      ;;
    teams)
      if [[ $TEAMS_SOURCE == snap ]]; then
        snap list teams-for-linux >/dev/null 2>&1 || return 0
        snap remove teams-for-linux
        return
      fi
      repo=teams-for-linux
      ;;
    extrepo:*) repo="${source#extrepo:}" ;;
  esac
  mapfile -t packages < <(app_packages "$id")
  for package in "${packages[@]}"; do
    package="${package%/backports}"
    pkg_installed "$package" && installed+=("$package")
  done
  if ((${#installed[@]})); then
    apt_get remove --autoremove "${installed[@]}" || return 1
  fi
  if [[ -n $repo ]] && extrepo_enabled "$repo" && ! app_repo_in_use "$repo" "$id"; then
    extrepo disable "$repo" || return 1
    apt_mark_dirty
  fi
  return 0
}

default_app_set() {
  local name="$1" desktop="$2" stamp="$STATE_DIR/defaults/$1"
  shift 2
  [[ -f $stamp ]] && return 0
  [[ -f $APPLICATIONS_DIR/$desktop ]] || return 0
  as_user xdg-mime default "$desktop" "$@" || return 1
  mkdir -p "${stamp%/*}" && printf '%s\n' "$desktop" >"$stamp"
}

default_apps_set() {
  default_app_set images imv.desktop "${IMAGE_TYPES[@]}" || return 1
  if app_wanted evince; then
    default_app_set pdf org.gnome.Evince.desktop application/pdf || return 1
  else
    default_app_set pdf okularApplication_pdf.desktop application/pdf || return 1
  fi
  default_app_set videos vlc.desktop "${VIDEO_TYPES[@]}" "${AUDIO_TYPES[@]}" || return 1
  default_app_set archives org.kde.ark.desktop "${ARCHIVE_TYPES[@]}"
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

apps_list() {
  local filter="${1:-}" category title id mark
  if [[ -n $filter ]]; then
    for id in $(option_items APPS); do
      if app_present "$id"; then
        [[ $filter == installed ]] && printf '%s\t%s\n' "$id" "$(app_name "$id")"
      else
        [[ $filter == available ]] && printf '%s\t%s\n' "$id" "$(app_name "$id")"
      fi
    done
    return 0
  fi
  local -A group=()
  for id in $(option_items APPS); do
    group[$id]="$(app_category "$id")"
  done
  while IFS=$'\t' read -r category title; do
    printf '%s\n' "$title"
    for id in $(option_items APPS); do
      [[ ${group[$id]} == "$category" ]] || continue
      mark=" "
      app_present "$id" && mark="*"
      printf '  %s %-14s %s\n' "$mark" "$id" "$(app_name "$id")"
    done
  done < <(apps_categories)
  printf '\n* installed\n'
}

module_10() {
  local id
  for id in $(option_items APPS); do
    app_wanted "$id" || continue
    run_step "Install $(app_name "$id")" app_install "$id"
  done
  app_wanted helium && run_step "Make Helium the default browser" helium_default
  run_step "Choose the apps that open images, PDFs, videos and archives" default_apps_set
  return 0
}

verify_10_apps() {
  run_check "Every ticked app is installed" apps_missing
}

verify_10_helium() {
  app_wanted helium || return 0
  run_check "Helium is the default browser" helium_is_default
}
