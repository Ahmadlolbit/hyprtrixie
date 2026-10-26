FILES_STATE_DIR="${HYPRTRIXIE_STATE_DIR:-/var/lib/hyprtrixie}"
FILES_DB="$FILES_STATE_DIR/files.db"
DEPLOY_CHANGED=0
SHARE_DIR="${HYPRTRIXIE_SHARE_DIR:-/usr/share/hyprtrixie}"
BIN_DIR="${HYPRTRIXIE_BIN_DIR:-/usr/local/bin}"

file_hash() {
  [[ -f $1 ]] || return 1
  sha256sum "$1" | cut -d' ' -f1
}

files_db_get() {
  [[ -f $FILES_DB ]] || return 0
  awk -F'\t' -v path="$1" '$1 == path { hash = $2 } END { if (hash) print hash }' "$FILES_DB"
}

files_db_set() {
  local path="$1" hash="$2" tmp
  mkdir -p "$FILES_STATE_DIR" || return 1
  tmp="$(mktemp "$FILES_DB.XXXXXX")" || return 1
  {
    [[ -f $FILES_DB ]] && awk -F'\t' -v path="$path" '$1 != path' "$FILES_DB"
    printf '%s\t%s\n' "$path" "$hash"
  } >"$tmp"
  mv "$tmp" "$FILES_DB"
}

owner_group() {
  id -gn "$1" 2>/dev/null || printf '%s' "$1"
}

deploy_file() {
  local src="$1" dest="$2" mode="${3:-0644}" owner="${4:-root}" new current known
  DEPLOY_CHANGED=0
  new="$(file_hash "$src")" || {
    log error "missing source file: $src"
    return 1
  }
  if [[ -e $dest ]]; then
    current="$(file_hash "$dest")"
    known="$(files_db_get "$dest")"
    if [[ $current == "$new" ]]; then
      chmod "$mode" "$dest" || return 1
      chown "$owner:$(owner_group "$owner")" "$dest" || return 1
      files_db_set "$dest" "$new"
      return 0
    fi
    if [[ -n $known && $current != "$known" ]]; then
      [[ $new == "$known" ]] && return 0
      install -m "$mode" -o "$owner" -g "$(owner_group "$owner")" "$src" "$dest.hyprtrixie-new" || return 1
      log warn "kept local edits in $dest, new version written to $dest.hyprtrixie-new"
      return 0
    fi
    if [[ -z $known && ! -e $dest.hyprtrixie-old ]]; then
      cp -a "$dest" "$dest.hyprtrixie-old" || return 1
      log info "saved the previous $dest as $dest.hyprtrixie-old"
    fi
  fi
  install -D -m "$mode" -o "$owner" -g "$(owner_group "$owner")" "$src" "$dest" || return 1
  files_db_set "$dest" "$new"
  DEPLOY_CHANGED=1
  log info "deployed $dest"
}

deploy_once() {
  local src="$1" dest="$2" mode="${3:-0644}" owner="${4:-root}"
  [[ -e $dest ]] && return 0
  install -D -m "$mode" -o "$owner" -g "$(owner_group "$owner")" "$src" "$dest" || return 1
  log info "created $dest"
}

deploy_tree() {
  local src="$1" dest="$2" owner="${3:-root}" file rel mode
  [[ -d $src ]] || return 1
  while IFS= read -r -d '' file; do
    rel="${file#"$src"/}"
    mode=0644
    [[ -x $file ]] && mode=0755
    deploy_file "$file" "$dest/$rel" "$mode" "$owner" || return 1
  done < <(find "$src" -type f -print0 | sort -z)
}

user_path() {
  printf '%s/%s' "$TARGET_HOME" "$1"
}

user_mkdir() {
  as_user mkdir -p "$(user_path "$1")"
}

deploy_user_file() {
  local src="$1" rel="$2" mode="${3:-0644}"
  user_mkdir "$(dirname "$rel")" || return 1
  deploy_file "$src" "$(user_path "$rel")" "$mode" "$TARGET_USER"
}

deploy_user_once() {
  local src="$1" rel="$2" mode="${3:-0644}"
  user_mkdir "$(dirname "$rel")" || return 1
  deploy_once "$src" "$(user_path "$rel")" "$mode" "$TARGET_USER"
}

sed_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//&/\\&}"
  value="${value//|/\\|}"
  printf '%s' "$value"
}

render_template() {
  local src="$1" pair args=()
  shift
  for pair in "$@"; do
    args+=(-e "s|{{ ${pair%%=*} }}|$(sed_escape "${pair#*=}")|g")
  done
  sed "${args[@]}" "$src"
}

deploy_rendered() {
  local src="$1" dest="$2" mode="$3" owner="$4" tmp rc
  shift 4
  tmp="$(mktemp)" || return 1
  render_template "$src" "$@" >"$tmp" && deploy_file "$tmp" "$dest" "$mode" "$owner"
  rc=$?
  rm -f "$tmp"
  return "$rc"
}

conf_set() {
  local file="$1" key="$2" value="$3"
  if [[ -f $file ]] && grep -qE "^#?[[:space:]]*$key=" "$file"; then
    sed -i -E "s|^#?[[:space:]]*$key=.*|$key=$value|" "$file"
  else
    printf '%s=%s\n' "$key" "$value" >>"$file"
  fi
}

ensure_line() {
  local file="$1" line="$2"
  [[ -f $file ]] && grep -qxF -- "$line" "$file" && return 0
  printf '%s\n' "$line" >>"$file"
}

remove_line() {
  local file="$1" line="$2" tmp
  [[ -f $file ]] && grep -qxF -- "$line" "$file" || return 0
  tmp="$(mktemp)" || return 1
  grep -vxF -- "$line" "$file" >"$tmp"
  cat "$tmp" >"$file"
  rm -f "$tmp"
}
