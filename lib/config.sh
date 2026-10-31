CONFIG_FILE="${HYPRTRIXIE_CONFIG:-/etc/hyprtrixie/hyprtrixie.conf}"
CONFIG_CHARS='[A-Za-z0-9 ._:/@+,-]'
PROFILES=(minimal everyday developer)
CONFIG_VERSION=2
CONFIG_LOADED_VERSION=""
CONFIG_MIGRATIONS=()
OPTION_KEYS=()
declare -gA OPTION_SECTION=()
declare -gA OPTION_LABEL=()
declare -gA OPTION_TYPE=()
declare -gA OPTION_DEFAULT=()
declare -gA OPTION_VALUES=()
declare -gA OPTION_WHEN=()
declare -gA OPTION_HELP=()
declare -gA OPTION_GROUPS=()
declare -gA OPTION_GROUP_OF=()
declare -gA OPTION_NAMES=()

profile_level() {
  local i
  for i in "${!PROFILES[@]}"; do
    [[ ${PROFILES[$i]} == "$1" ]] && {
      printf '%s' "$i"
      return 0
    }
  done
  printf '%s' "$((${#PROFILES[@]} - 1))"
}

option() {
  local key="$1" section="" label="" type="choice" default="" values="" when="" help="" groups="" group_of="" names=""
  shift
  while (($#)); do
    case "$1" in
      --section) section="$2" ;;
      --label) label="$2" ;;
      --type) type="$2" ;;
      --default) default="$2" ;;
      --values) values="$2" ;;
      --when) when="$2" ;;
      --help) help="$2" ;;
      --groups) groups="$2" ;;
      --group-of) group_of="$2" ;;
      --names) names="$2" ;;
      *) return 1 ;;
    esac
    shift 2
  done
  [[ -n ${OPTION_TYPE[$key]+set} ]] || OPTION_KEYS+=("$key")
  OPTION_SECTION[$key]="$section"
  OPTION_LABEL[$key]="${label:-$key}"
  OPTION_TYPE[$key]="$type"
  OPTION_DEFAULT[$key]="$default"
  OPTION_VALUES[$key]="$values"
  OPTION_WHEN[$key]="$when"
  OPTION_HELP[$key]="$help"
  OPTION_GROUPS[$key]="$groups"
  OPTION_GROUP_OF[$key]="$group_of"
  OPTION_NAMES[$key]="$names"
  [[ -v $key ]] || printf -v "$key" '%s' "$(option_default "$key")"
}

option_items() {
  local item
  for item in ${OPTION_VALUES[$1]}; do
    printf '%s\n' "${item%@*}"
  done
}

option_has_item() {
  local item
  for item in ${OPTION_VALUES[$1]}; do
    [[ ${item%@*} == "$2" ]] && return 0
  done
  return 1
}

option_default() {
  local key="$1" item tag out=()
  if [[ ${OPTION_TYPE[$key]} == multi && ${OPTION_VALUES[$key]} == *@* ]]; then
    for item in ${OPTION_VALUES[$key]}; do
      tag="${item#*@}"
      [[ $tag == "$item" ]] && tag="${PROFILES[0]}"
      [[ $tag == offered ]] && continue
      if (($(profile_level "$tag") <= $(profile_level "${PROFILE:-developer}"))); then
        out+=("${item%@*}")
      fi
    done
    printf '%s' "${out[*]}"
    return 0
  fi
  printf '%s' "${OPTION_DEFAULT[$key]}"
}

option_valid() {
  local key="$1" value="$2" word
  case "${OPTION_TYPE[$key]}" in
    choice) option_has_item "$key" "$value" ;;
    bool) [[ $value == yes || $value == no ]] ;;
    number) [[ $value =~ ^(0|[1-9][0-9]*)$ ]] ;;
    multi)
      for word in $value; do
        option_has_item "$key" "$word" || return 1
      done
      ;;
    *) return 0 ;;
  esac
}

option_enabled() {
  local key="$1" item="$2"
  [[ " ${!key} " == *" $item "* ]]
}

option_visible() {
  local when="${OPTION_WHEN[$1]}"
  [[ -z $when ]] || "$when"
}

section_keys() {
  local key
  for key in "${OPTION_KEYS[@]}"; do
    [[ ${OPTION_SECTION[$key]} == "$1" ]] || continue
    option_visible "$key" && printf '%s\n' "$key"
  done
  return 0
}

config_reset_profile() {
  local key
  for key in "${OPTION_KEYS[@]}"; do
    [[ ${OPTION_TYPE[$key]} == multi && ${OPTION_VALUES[$key]} == *@* ]] || continue
    printf -v "$key" '%s' "$(option_default "$key")"
  done
}

config_validate() {
  local key value
  for key in "${OPTION_KEYS[@]}"; do
    value="${!key}"
    option_valid "$key" "$value" && continue
    log warn "invalid $key='$value' in the configuration, using '$(option_default "$key")'"
    printf -v "$key" '%s' "$(option_default "$key")"
  done
  return 0
}

config_load() {
  local file="${1:-$CONFIG_FILE}" line key
  [[ -f $file ]] || return 1
  CONFIG_LOADED_VERSION=""
  while IFS= read -r line || [[ -n $line ]]; do
    [[ -z $line ]] && continue
    if [[ ! $line =~ ^([A-Z][A-Z0-9_]*)=\"($CONFIG_CHARS*)\"$ ]]; then
      log warn "ignoring malformed line in $file: $line"
      continue
    fi
    key="${BASH_REMATCH[1]}"
    if [[ $key == HYPRTRIXIE_CONFIG_VERSION ]]; then
      CONFIG_LOADED_VERSION="${BASH_REMATCH[2]}"
      continue
    fi
    if [[ -z ${OPTION_TYPE[$key]+set} ]]; then
      log warn "ignoring unknown setting $key in $file"
      continue
    fi
    printf -v "$key" '%s' "${BASH_REMATCH[2]}"
  done <"$file"
  config_validate
  config_migrate "${CONFIG_LOADED_VERSION:-1}"
  return 0
}

config_migration() {
  CONFIG_MIGRATIONS+=("$1 $2")
}

config_migrate() {
  local from="$1" entry version fn
  [[ $from =~ ^[0-9]+$ ]] || from=1
  for entry in "${CONFIG_MIGRATIONS[@]}"; do
    read -r version fn <<<"$entry"
    ((version > from)) || continue
    log info "updating the configuration for version $version: $fn"
    "$fn"
  done
  return 0
}

option_name() {
  local key="$1" item="$2"
  if [[ -n ${OPTION_NAMES[$key]} ]]; then
    "${OPTION_NAMES[$key]}" "$item"
  else
    printf '%s' "$item"
  fi
}

config_save() {
  local file="${1:-$CONFIG_FILE}" key tmp
  mkdir -p "${file%/*}" || return 1
  tmp="$(mktemp "$file.XXXXXX")" || return 1
  {
    printf 'HYPRTRIXIE_CONFIG_VERSION="%s"\n' "$CONFIG_VERSION"
    for key in "${OPTION_KEYS[@]}"; do
      [[ ${!key} =~ ^$CONFIG_CHARS*$ ]] || continue
      printf '%s="%s"\n' "$key" "${!key}"
    done
  } >"$tmp"
  chmod 0644 "$tmp"
  mv "$tmp" "$file"
}

config_show() {
  local key
  for key in "${OPTION_KEYS[@]}"; do
    printf '%-22s %-28s %s\n' "$key" "${OPTION_LABEL[$key]}" "${!key}"
  done
}

option PROFILE --section profile --label "Profile" --type choice \
  --default developer --values "developer everyday minimal" \
  --help "Presets for apps and tools: minimal, everyday, or developer (everything)"
option FAILURE_MODE --section run --label "When a step fails" --type choice \
  --default ask --values "ask halt continue" \
  --help "Ask what to do, stop the run, or note the failure and carry on"
