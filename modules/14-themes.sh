register_module 14 "Themes"

OMARCHY_DIR="${HYPRTRIXIE_OMARCHY_DIR:-/usr/share/omarchy}"

theme_copy() {
  local src="$1" dest="$2" old="${3:-}" new="${4:-}" file base
  [[ -f $src/colors.toml ]] || return 0
  rm -rf "$dest.part"
  mkdir -p "$dest.part/backgrounds" || return 1
  for file in colors.toml icons.theme preview.png; do
    if [[ -f $src/$file ]]; then
      install -m 0644 "$src/$file" "$dest.part/$file" || return 1
    fi
  done
  for file in "$src"/backgrounds/*; do
    [[ -f $file && ! -L $file ]] || continue
    base="${file##*/}"
    [[ -n $old ]] && base="${base//"$old"/"$new"}"
    install -m 0644 "$file" "$dest.part/backgrounds/$base" || return 1
  done
  rm -rf "$dest"
  mv "$dest.part" "$dest"
}

omarchy_themes_install() {
  local dir
  source_current omarchy && [[ -f $OMARCHY_DIR/LICENSE.omarchy ]] && return 0
  fetch_git_cached omarchy /themes/ /LICENSE || return 1
  for dir in "$FETCHED_PATH"/themes/*/; do
    dir="${dir%/}"
    theme_copy "$dir" "$OMARCHY_DIR/themes/${dir##*/}" || return 1
  done
  install -D -m 0644 "$FETCHED_PATH/LICENSE" "$OMARCHY_DIR/LICENSE.omarchy" || return 1
  rm -rf "$FETCHED_PATH"
  source_mark omarchy
}

themes_present() {
  compgen -G "$OMARCHY_DIR/themes/*/colors.toml" >/dev/null
}

module_14() {
  run_step "Install the Omarchy themes" omarchy_themes_install
}

verify_14_themes() {
  run_check "All themes are installed" themes_present
}
