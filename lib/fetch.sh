MANIFEST_DIR="${HYPRTRIXIE_MANIFESTS:-${HYPRTRIXIE_ROOT:-.}/manifests}"
CACHE_DIR="${HYPRTRIXIE_CACHE_DIR:-/var/cache/hyprtrixie}"
RELEASE_NAME=""
RELEASE_VERSION=""
RELEASE_URL=""
RELEASE_SHA256=""
FETCHED_PATH=""
SOURCE_URL=""
SOURCE_COMMIT=""

manifest_line() {
  local file="$MANIFEST_DIR/$1"
  [[ -f $file ]] || return 1
  awk -v name="$2" '$1 == name { print; found = 1; exit } END { exit !found }' "$file"
}

release_get() {
  local line
  line="$(manifest_line releases "$1")" || {
    log error "no pinned release named $1"
    return 1
  }
  read -r RELEASE_NAME RELEASE_VERSION RELEASE_URL RELEASE_SHA256 <<<"$line"
}

sha256_ok() {
  [[ -f $1 ]] && [[ "$(sha256sum "$1" | cut -d' ' -f1)" == "$2" ]]
}

download() {
  curl -fL --retry 3 --retry-delay 3 --connect-timeout 20 -o "$2" "$1"
}

fetch_release() {
  local dir="$CACHE_DIR/downloads" file
  release_get "$1" || return 1
  file="$dir/$RELEASE_NAME-$RELEASE_VERSION-${RELEASE_URL##*/}"
  mkdir -p "$dir" || return 1
  if sha256_ok "$file" "$RELEASE_SHA256"; then
    FETCHED_PATH="$file"
    return 0
  fi
  log info "downloading $RELEASE_URL"
  download "$RELEASE_URL" "$file.part" || return 1
  if ! sha256_ok "$file.part" "$RELEASE_SHA256"; then
    log error "checksum mismatch for $RELEASE_NAME $RELEASE_VERSION"
    rm -f "$file.part"
    return 1
  fi
  mv "$file.part" "$file"
  FETCHED_PATH="$file"
}

release_current() {
  release_get "$1" || return 1
  [[ "$(cat "${STATE_DIR:-/var/lib/hyprtrixie}/releases/$1" 2>/dev/null)" == "$RELEASE_VERSION" ]]
}

release_mark() {
  local dir="${STATE_DIR:-/var/lib/hyprtrixie}/releases"
  release_get "$1" || return 1
  mkdir -p "$dir" && printf '%s\n' "$RELEASE_VERSION" >"$dir/$1"
}

source_get() {
  local line
  line="$(manifest_line sources "$1")" || {
    log error "no pinned source named $1"
    return 1
  }
  read -r _ SOURCE_URL SOURCE_COMMIT <<<"$line"
}

git_head() {
  git -C "$1" rev-parse HEAD 2>/dev/null
}

fetch_git() {
  local name="$1" dest="$2" tmp
  shift 2
  source_get "$name" || return 1
  if [[ -d $dest/.git && "$(git_head "$dest")" == "$SOURCE_COMMIT" ]]; then
    return 0
  fi
  tmp="$dest.part"
  rm -rf "$tmp"
  mkdir -p "$(dirname "$dest")" || return 1
  git init -q "$tmp" || return 1
  git -C "$tmp" remote add origin "$SOURCE_URL" || return 1
  if (($#)); then
    git -C "$tmp" sparse-checkout set --no-cone "$@" || return 1
  fi
  log info "fetching $name at $SOURCE_COMMIT"
  git -C "$tmp" fetch -q --depth 1 --filter=blob:none origin "$SOURCE_COMMIT" || return 1
  git -C "$tmp" checkout -q FETCH_HEAD || return 1
  if [[ "$(git_head "$tmp")" != "$SOURCE_COMMIT" ]]; then
    log error "$name is not at the pinned commit"
    return 1
  fi
  rm -rf "$dest"
  mv "$tmp" "$dest"
}

fetch_git_cached() {
  local name="$1"
  shift
  FETCHED_PATH="$CACHE_DIR/sources/$name"
  fetch_git "$name" "$FETCHED_PATH" "$@"
}
