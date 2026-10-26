TARGET_USER=""
TARGET_HOME=""
TARGET_UID=""

human_users() {
  getent passwd | awk -F: '$3 >= 1000 && $3 < 60000 && $7 !~ /(nologin|false)$/ { print $1 }'
}

set_target_user() {
  local entry
  entry="$(getent passwd "$1")" || return 1
  IFS=: read -r TARGET_USER _ TARGET_UID _ _ TARGET_HOME _ <<<"$entry"
}

detect_target_user() {
  local candidate="${HYPRTRIXIE_USER:-${SUDO_USER:-}}" users=()
  if [[ -n $candidate && $candidate != "root" ]]; then
    set_target_user "$candidate"
    return
  fi
  mapfile -t users < <(human_users)
  if ((${#users[@]} == 1)); then
    set_target_user "${users[0]}"
    return
  fi
  return 1
}

user_shell() {
  getent passwd "${1:-$TARGET_USER}" | cut -d: -f7
}

as_user() {
  local runtime="/run/user/$TARGET_UID" env=()
  env=(
    HOME="$TARGET_HOME"
    USER="$TARGET_USER"
    LOGNAME="$TARGET_USER"
    SHELL="$(user_shell)"
    PATH="$TARGET_HOME/.local/bin:/usr/local/bin:/usr/bin:/bin"
    LANG="${LANG:-C.UTF-8}"
  )
  if [[ -d $runtime ]]; then
    env+=(XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus")
  fi
  (cd "$TARGET_HOME" && runuser -u "$TARGET_USER" -- env "${env[@]}" "$@")
}

user_in_group() {
  id -nG "${2:-$TARGET_USER}" 2>/dev/null | tr ' ' '\n' | grep -qx "$1"
}

user_add_groups() {
  local group
  for group in "$@"; do
    getent group "$group" >/dev/null || continue
    user_in_group "$group" && continue
    usermod -aG "$group" "$TARGET_USER" || return 1
  done
}

user_has_password() {
  [[ "$(passwd -S "${1:-$TARGET_USER}" 2>/dev/null | awk '{ print $2 }')" == "P" ]]
}

user_systemctl() {
  systemctl --user --machine="$TARGET_USER@" "$@"
}

user_session_running() {
  [[ -S /run/user/$TARGET_UID/bus ]]
}
