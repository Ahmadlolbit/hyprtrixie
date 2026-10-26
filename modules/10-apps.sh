register_module 10 "Applications"

option APPS --section apps --label "Applications" --type multi \
  --values "helium@minimal whatsapp@everyday dolphin@minimal libreoffice@everyday vlc@everyday telegram@everyday"

app_wanted() {
  option_enabled APPS "$1"
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

module_10() {
  if app_wanted helium; then
    run_step "Install the Helium browser" helium_install
    run_step "Make Helium the default browser" helium_default
  fi
  app_wanted whatsapp && run_step "Add the WhatsApp web app" whatsapp_install
  run_step "Install desktop applications" desktop_apps_install
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
