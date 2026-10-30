# hyprtrixie

Turn a minimal Debian 13 install into a polished Hyprland desktop, with a guided setup, sensible defaults and real options. It's a tool, not a distro: run it once, and you're simply on Debian.

## What you get

- **Hyprland 0.55** from Debian backports, configured in Lua, with a sharp or soft window style.
- **A bar** with popups you can click: Wi-Fi, Bluetooth, sound for each app, brightness, battery and power modes, a calendar, media controls, weather, notifications, updates and snapshots. It can be full width, floating, minimal or down the left side.
- **22 themes** that change everything at once: windows, bar, terminal, launcher, notifications, lock screen, login screen, boot splash and boot menu. Press Super+Ctrl+T to pick one from previews. Accent colours, fonts, glass, automatic light and dark, and wallpaper rotation are all a setting away.
- **The everyday pieces, already wired up:** a launcher, a menu on Super+Alt+Space, a key sheet, a lock screen, idle dimming and suspend, screenshots, a screensaver, a focus timer, and notifications with quiet hours.
- **A terminal setup:** kitty, zsh, Starship or Powerlevel10k, tmux, and the JetBrains Mono Nerd Font, plus bat, eza, fd, fzf, ripgrep, btop, fastfetch, lazygit and lazydocker.
- **Apps and developer tools you pick from a catalogue:** browsers, chat, office, media, Docker and Podman, Node, Rust, Go, Kubernetes tools, Neovim, VS Code, Zed and more.
- **A system that can roll back:** btrfs snapshots with snapper, listed in the boot menu, and a snapshot before every system update.
- **Hardware support:** the backports kernel and firmware for what it finds, microcode, video decoding, and the NVIDIA driver when there's an NVIDIA card.
- **Sensible security:** a firewall, automatic security updates, and no SSH server unless you want one.

## Requirements

- Debian 13 (trixie), 64-bit PC, UEFI.
- Secure Boot off, because drivers built on the machine aren't signed.
- 30 GiB free on `/`, and an internet connection.
- Recommended: install Debian with an encrypted btrfs root, as described in [Installing Debian](docs/install.md). That layout keeps `/home` and logs out of the snapshots.

## Quick start

```bash
sudo apt install git
git clone https://github.com/Ahmadlolbit/hyprtrixie.git
cd hyprtrixie
sudo ./hyprtrixie
```

The setup menu opens. Change whatever you like, then choose **Install**. When it finishes, reboot and log in.

If you set a root password while installing Debian, your account has no `sudo` yet. Run `su -`, then `./hyprtrixie` from the clone, and it sets `sudo` up for you.

## Commands

| Command | What it does |
| --- | --- |
| `sudo ./hyprtrixie` | open the setup menu |
| `sudo ./hyprtrixie install [--only 04,05] [--skip 10] [--yes]` | run the setup, or only some modules |
| `sudo ./hyprtrixie verify` | run every check again |
| `./hyprtrixie modules` | list the modules and how they last ended |
| `sudo ./hyprtrixie config show\|edit\|reset` | look at or change the saved choices |
| `hyprtrixie guide` | read the guide in the terminal |
| `hyprtrixie update` | update everything, with a snapshot first |
| `sudo hyprtrixie doctor` | check the system and offer safe fixes |
| `sudo hyprtrixie collect --verbose` | pack everything for a bug report |

Running it again is safe. Finished steps are checked, not redone, and files you edited are left alone: if there's a newer version of one, it lands next to yours as `.hyprtrixie-new`.

## First keys

| Keys | Action |
| --- | --- |
| Super+Return | terminal |
| Super+Space | launcher |
| Super+Alt+Space | menu |
| Super+Q | close the window |
| Super+1…0 | switch workspace |
| Super+Shift+1…0 | move the window to a workspace |
| Super+L | lock |
| Super+Escape | lock, suspend, log out, restart, shut down |
| Super+Ctrl+T | pick a theme |
| Print | screenshot of a region |

## Documentation

- [Installing Debian](docs/install.md)
- [Using hyprtrixie](docs/usage.md): every option, key, widget and setting
- [Troubleshooting](docs/troubleshooting.md)
- `hyprtrixie guide`, the short guide, in the terminal

## Built on

Hyprland, Quickshell, Walker and Elephant, hyprlock, hypridle, hyprpaper, mako, snapper, grub-btrfs and snapper-rollback.

## License

GPL-3.0
