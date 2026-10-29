export ZSH=/usr/share/hyprtrixie/oh-my-zsh
export ZSH_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/oh-my-zsh"
ZSH_THEME=""
DISABLE_AUTO_UPDATE=true
DISABLE_UPDATE_PROMPT=true
plugins=(git sudo colored-man-pages extract)
for plugin in docker kubectl helm fzf; do
  (( $+commands[$plugin] )) && plugins+=($plugin)
done
unset plugin
mkdir -p "$ZSH_CACHE_DIR/completions"
source "$ZSH/oh-my-zsh.sh"

HISTFILE="$HOME/.zsh_history"
HISTSIZE=50000
SAVEHIST=50000
setopt hist_ignore_all_dups hist_ignore_space share_history

typeset -U path
path=("$HOME/.local/bin" "$HOME/bin" "$HOME/.cargo/bin" $path)

(( $+commands[mise] )) && eval "$(mise activate zsh)"
if (( $+commands[pyenv] )); then
  export PYENV_ROOT="$HOME/.pyenv"
  eval "$(pyenv init - zsh)"
fi

if (( $+commands[eza] )); then
  alias ls='eza --group-directories-first --icons=auto'
  alias ll='eza -l --git --group-directories-first --icons=auto'
  alias la='eza -la --git --group-directories-first --icons=auto'
  alias tree='eza --tree --icons=auto'
fi
(( $+commands[bat] )) && export BAT_THEME=ansi
(( $+commands[fd] )) && export FZF_DEFAULT_COMMAND='fd --type f --hidden --exclude .git'

if [[ "$(cat /var/lib/hyprtrixie/container-engine 2>/dev/null)" == podman ]]; then
  export DOCKER_HOST="unix://${XDG_RUNTIME_DIR:-/run/user/$UID}/podman/podman.sock"
fi

[[ -f /usr/share/hyprtrixie/zsh/prompt.zsh ]] && source /usr/share/hyprtrixie/zsh/prompt.zsh

for script in /usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh /usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh; do
  [[ -f $script ]] && source "$script"
done
unset script

[[ -o interactive ]] && (( $+commands[hyprtrixie-tip] )) && hyprtrixie-tip terminal
