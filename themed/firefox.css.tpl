:root {
  color-scheme: {{ mode }} !important;
  --lwt-accent-color: {{ dark_background }} !important;
  --lwt-accent-color-inactive: {{ dark_background }} !important;
  --lwt-text-color: {{ foreground }} !important;
  --toolbox-bgcolor: {{ dark_background }} !important;
  --toolbox-bgcolor-inactive: {{ dark_background }} !important;
  --toolbox-textcolor: {{ foreground }} !important;
  --toolbox-textcolor-inactive: {{ subtext }} !important;
  --toolbar-bgcolor: {{ background }} !important;
  --toolbar-color: {{ foreground }} !important;
  --tab-selected-bgcolor: {{ background }} !important;
  --tab-selected-textcolor: {{ foreground }} !important;
  --tab-hover-background-color: {{ surface }} !important;
  --tab-loading-fill: {{ accent }} !important;
  --tab-attention-icon-color: {{ accent }} !important;
  --toolbar-field-background-color: {{ surface }} !important;
  --toolbar-field-color: {{ foreground }} !important;
  --toolbar-field-border-color: {{ raised }} !important;
  --toolbar-field-focus-background-color: {{ dark_background }} !important;
  --toolbar-field-focus-color: {{ foreground }} !important;
  --toolbar-field-focus-border-color: {{ accent }} !important;
  --urlbarView-highlight-background: {{ selection_background }} !important;
  --urlbarView-highlight-color: {{ foreground }} !important;
  --urlbarView-separator-color: {{ raised }} !important;
  --arrowpanel-background: {{ dark_background }} !important;
  --arrowpanel-color: {{ foreground }} !important;
  --arrowpanel-border-color: {{ raised }} !important;
  --panel-separator-color: {{ raised }} !important;
  --sidebar-background-color: {{ dark_background }} !important;
  --sidebar-text-color: {{ foreground }} !important;
  --sidebar-border-color: {{ raised }} !important;
  --focus-outline-color: {{ accent }} !important;
  --toolbarbutton-icon-fill: {{ foreground }} !important;
  --toolbarbutton-icon-fill-attention: {{ accent }} !important;
  --toolbarbutton-hover-background: {{ surface }} !important;
  --toolbarbutton-active-background: {{ raised }} !important;
  --chrome-content-separator-color: {{ dark_background }} !important;
  --button-primary-bgcolor: {{ accent }} !important;
  --button-primary-color: {{ background }} !important;
  --color-accent-primary: {{ accent }} !important;
}

#navigator-toolbox {
  background-color: {{ dark_background }} !important;
  color: {{ foreground }} !important;
}

menupopup,
panel {
  --panel-background: {{ dark_background }} !important;
  --panel-color: {{ foreground }} !important;
  --panel-border-color: {{ raised }} !important;
}

menupopup menu[_moz-menuactive],
menupopup menuitem[_moz-menuactive] {
  background-color: {{ selection_background }} !important;
  color: {{ foreground }} !important;
}
