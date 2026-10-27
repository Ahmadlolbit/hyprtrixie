service_enabled() {
  systemctl is-enabled --quiet "$1" 2>/dev/null
}

service_active() {
  systemctl is-active --quiet "$1" 2>/dev/null
}

service_enable() {
  local unit
  for unit in "$@"; do
    service_enabled "$unit" || systemctl enable "$unit" || return 1
  done
}

service_start() {
  local unit
  for unit in "$@"; do
    service_enable "$unit" || return 1
    service_active "$unit" || systemctl start "$unit" || return 1
  done
}

service_disable() {
  local unit
  for unit in "$@"; do
    if service_enabled "$unit" || service_active "$unit"; then
      systemctl disable --now "$unit" || return 1
    fi
  done
}

service_reload() {
  systemctl daemon-reload
}
