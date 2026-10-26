register_module 10 "Applications"

option APPS --section apps --label "Applications" --type multi \
  --values "helium@minimal whatsapp@everyday dolphin@minimal libreoffice@everyday vlc@everyday telegram@everyday signal@everyday vesktop@everyday teams@everyday"
option TEAMS_SOURCE --section apps --label "Install Teams from" --type choice \
  --default apt --values "apt snap" --when teams_wanted

app_wanted() {
  option_enabled APPS "$1"
}

teams_wanted() {
  app_wanted teams
}

helium_install() {
  local rc=0
  pkg_installed helium-bin && return 0
  extrepo_try helium || rc=$?
  if ((rc == 0)) && [[ -n "$(pkg_candidate helium-bin)" ]]; then
    apt_install helium-bin
    return
  fi
  printf 'Helium repository unavailable, installing the pinned package instead\n'
  deb_install_release helium helium-bin
}

helium_default() {
  as_user xdg-mime default helium.desktop x-scheme-handler/http x-scheme-handler/https text/html application/xhtml+xml
}

helium_is_default() {
  [[ "$(as_user xdg-mime query default x-scheme-handler/https)" == helium.desktop ]]
}

whatsapp_install() {
  pkg_installed helium-bin || helium_install || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/applications/whatsapp-web.svg" "$SHARE_DIR/icons/whatsapp-web.svg" || return 1
  deploy_file "$HYPRTRIXIE_ROOT/config/applications/whatsapp-web.desktop" /usr/local/share/applications/whatsapp-web.desktop
}

desktop_apps_install() {
  local app lists=()
  for app in dolphin libreoffice vlc telegram; do
    app_wanted "$app" && lists+=("app-$app")
  done
  ((${#lists[@]})) || return 0
  apt_install_list "${lists[@]}"
}

desktop_apps_present() {
  local app
  for app in dolphin libreoffice vlc telegram; do
    app_wanted "$app" || continue
    list_present "app-$app" || return 1
  done
}

messaging_install() {
  if app_wanted signal; then
    extrepo_try signal || return 1
    apt_install_list app-signal || return 1
  fi
  if app_wanted vesktop; then
    deb_install_release vesktop vesktop || return 1
  fi
  if app_wanted teams; then
    teams_install || return 1
  fi
}

teams_install() {
  if [[ $TEAMS_SOURCE == snap ]]; then
    apt_install_list app-teams-snap || return 1
    snap wait system seed.loaded || return 1
    snap list teams-for-linux >/dev/null 2>&1 || snap install teams-for-linux
    return
  fi
  extrepo_try teams-for-linux || return 1
  apt_install_list app-teams
}

teams_present() {
  if [[ $TEAMS_SOURCE == snap ]]; then
    snap list teams-for-linux >/dev/null 2>&1
  else
    pkg_installed teams-for-linux
  fi
}

messaging_present() {
  app_wanted signal && ! pkg_installed signal-desktop && return 1
  app_wanted vesktop && ! pkg_installed vesktop && return 1
  app_wanted teams && ! teams_present && return 1
  return 0
}

module_10() {
  if app_wanted helium; then
    run_step "Install the Helium browser" helium_install
    run_step "Make Helium the default browser" helium_default
  fi
  app_wanted whatsapp && run_step "Add the WhatsApp web app" whatsapp_install
  run_step "Install desktop applications" desktop_apps_install
  run_step "Install messaging apps" messaging_install
  return 0
}

verify_10_helium() {
  app_wanted helium || return 0
  run_check "Helium is installed" pkg_installed helium-bin
  run_check "Helium is the default browser" helium_is_default
}

verify_10_whatsapp() {
  app_wanted whatsapp || return 0
  run_check "The WhatsApp web app is in the launcher" test -f /usr/local/share/applications/whatsapp-web.desktop
}

verify_10_desktop_apps() {
  run_check "Desktop applications are installed" desktop_apps_present
}

verify_10_messaging() {
  run_check "Messaging apps are installed" messaging_present
}
