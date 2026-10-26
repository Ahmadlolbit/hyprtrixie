register_module 10 "Applications"

option APPS --section apps --label "Applications" --type multi \
  --values "helium@minimal"

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

module_10() {
  if app_wanted helium; then
    run_step "Install the Helium browser" helium_install
    run_step "Make Helium the default browser" helium_default
  fi
  return 0
}

verify_10_helium() {
  app_wanted helium || return 0
  run_check "Helium is installed" pkg_installed helium-bin
  run_check "Helium is the default browser" helium_is_default
}
