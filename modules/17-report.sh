register_module 17 "Final checks and report"

REPORT_NAME=runbook-report.md

report_steps() {
  local entry id kind title
  for entry in "$@"; do
    IFS='|' read -r id kind title <<<"$entry"
    printf -- '- %s %s: %s\n' "$id" "$kind" "$title"
  done
}

report_markdown() {
  local id status when
  printf '# hyprtrixie report\n\n'
  printf -- '- Date: %s\n- hyprtrixie: %s\n- Configuration: %s\n- Log: %s\n\n' \
    "$(date '+%F %T')" "$HYPRTRIXIE_VERSION" "$CONFIG_FILE" "${LOG_FILE:-none}"
  printf '## Machine\n\n'
  detect_summary | sed 's/^/    /'
  printf '\n'
  printf '## Modules\n\n| Module | Result | When |\n|---|---|---|\n'
  for id in "${MODULE_IDS[@]}"; do
    IFS='|' read -r status when <<<"$(status_of "$id")"
    printf '| %s %s | %s | %s |\n' "$id" "${MODULE_TITLES[$id]}" "${status:-not run}" "${when:-}"
  done
  if ((${#STEP_FAILURES[@]})); then
    printf '\n## Failed\n\n'
    report_steps "${STEP_FAILURES[@]}"
  fi
  if ((${#STEP_SKIPS[@]})); then
    printf '\n## Skipped\n\n'
    report_steps "${STEP_SKIPS[@]}"
  fi
  cat <<EOF

## Next

- Reboot to start the desktop.
- \`sudo hyprtrixie verify\` runs every check again.
EOF
}

write_report() {
  local file
  file="$(user_path "$REPORT_NAME")"
  report_markdown >"$file.part" || return 1
  mv "$file.part" "$file" && chown "$TARGET_USER:" "$file"
}

module_17() {
  local id
  for id in "${MODULE_IDS[@]}"; do
    [[ $id == 17 ]] || verify_module "$id"
  done
  run_step "Write ~/$REPORT_NAME" write_report
}
