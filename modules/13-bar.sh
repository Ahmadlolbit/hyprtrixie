register_module 13 "Bar"

option BAR_POSITION --section look --label "Bar position" --type choice \
  --default top --values "top bottom"
option CLOCK_FORMAT --section look --label "Clock" --type choice \
  --default 24h --values "24h 12h"

BAR_DIR="$SHARE_DIR/shell"

bar_install() {
  deploy_tree "$HYPRTRIXIE_ROOT/shell" "$BAR_DIR"
}

bar_settings() {
  deploy_rendered "$HYPRTRIXIE_ROOT/system/shell.json" /etc/hyprtrixie/shell.json 0644 root \
    "POSITION=$BAR_POSITION" "CLOCK=$CLOCK_FORMAT"
}

bar_ready() {
  [[ -f $BAR_DIR/shell.qml && -f /etc/hyprtrixie/shell.json ]] && commands_present qs
}

module_13() {
  run_step "Install the bar" bar_install
  run_step "Apply the bar settings" bar_settings
}

verify_13_bar() {
  run_check "The bar is installed" bar_ready
}
