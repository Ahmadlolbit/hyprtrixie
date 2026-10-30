# Troubleshooting

## Before anything else

- `sudo hyprtrixie doctor` checks the usual suspects (the desktop's programs, failed services, snapshots, disk space, battery, firmware, updates, hyprtrixie's own files) and offers the safe fixes.
- `hyprtrixie modules` shows which module failed. (Before the first install, the command is `./hyprtrixie` in the folder.)
- The full log is `/var/log/hyprtrixie/` (and a copy in `~/runbook-logs/`).
- `~/runbook-report.md` lists every failed or skipped step.
- Redo one module: `sudo hyprtrixie install --only 12`.
- When a step fails, **Show log** opens its whole output.
- For a bug report: `sudo hyprtrixie collect --verbose`, then attach the file it prints.

## The setup refuses to start

| Message | Fix |
| --- | --- |
| Debian 13 (trixie) | Only Debian 13 is supported. Testing, Ubuntu and older Debian are not. |
| 64-bit PC (amd64) | ARM and 32-bit machines are not supported. |
| Secure Boot is off | Turn Secure Boot off in the firmware settings (ThinkPad: F1 → Security → Secure Boot). |
| Internet connection | It needs to reach `https://deb.debian.org`. Check `ping deb.debian.org`, and the Wi-Fi from the installer. |
| Enough free disk space | 30 GiB free on `/` is needed. |
| no sudo / which account | Run it with `sudo` from your account. As root with more than one account: `HYPRTRIXIE_USER=name ./hyprtrixie`. |

## Package downloads fail

- **"Hash Sum mismatch"**: something between you and the mirror (often a caching proxy) is changing the files. The default mirror uses HTTPS, which avoids this. If you changed **Debian mirror** in the menu, use an `https://` one.
- **A vendor download times out** (Helium, VS Code, Docker): some networks block those sites. Helium falls back to its GitHub release; for the others, Retry later or choose Skip and untick the app.

## After logging in

- **Black screen in a virtual machine**: Hyprland needs 3D acceleration. In virt-manager: Video Virtio with 3D acceleration, and Display Spice with OpenGL.
- **The bar is missing**: `sudo hyprtrixie doctor` offers to start it again. To see why it stopped, run `qs -p /usr/share/hyprtrixie/shell` in a terminal; the errors show which part is failing.
- **The menu, the key sheet or the welcome screen doesn't open**: they're part of the bar, so check the bar first. `qs -p /usr/share/hyprtrixie/shell ipc call menu open root` in a terminal says "No running instances" when the bar isn't running.
- **Your own menu entries don't show**: a JSON mistake makes the menu skip the whole file. `jq . ~/.config/hyprtrixie/menu.json` points at it.
- **The launcher doesn't open**:
  - Check that `elephant` is running: `pgrep -a elephant`.
  - Restart Walker with `walker --gapplication-service &`.
- **Hyprland shows a config error bar**: run `Hyprland --verify-config` to see the line. The problem is usually something you added to `~/.config/hypr/hyprland.lua`.
- **A shortcut does nothing**: Super+B opens Helium. If you unticked Helium, bind Super+B to your browser in `~/.config/hypr/hyprland.lua`.
- **Wrong keyboard layout**: `sudo dpkg-reconfigure keyboard-configuration`, then log out and in.
- **The screen is too big or too small**: change **Display scale** in the menu and run `sudo ./hyprtrixie install --only 12`. Or set it per monitor in your own `hyprland.lua`.
- **The screensaver never starts**:
  - The bar's **awake** toggle must be off.
  - `hyprtrixie-screensaver status` must print `on`; Menu › Toggle › Screensaver turns it back on.
  - It only starts when its minutes come before the lock's, and never while the lock screen is up.
  - `hyprtrixie-screensaver start` shows it straight away. If nothing appears, run `tte --input-file /usr/share/hyprtrixie/screensaver.txt print` in a terminal and read what it prints.

## Themes

- **A theme only half applied**: pick it again with Super+Ctrl+T, then run `sudo hyprtrixie doctor`.
- **The theme picker doesn't open**: run `qs -p /usr/share/hyprtrixie/shell ipc call picker themes` in a terminal. "No running instances" means the bar isn't running; see "The bar is missing".
- **"ignored hyprland.lua from …"**: a theme cloned from git can't run code or replace terminal configs. That's on purpose; its colours and wallpapers still apply.

## Apps

- **An app from a vendor fails to install** (Brave, Chrome, Slack, Spotify, Mullvad and others come from the vendor's own repository): run `sudo apt update` and read the error. Some networks block vendor sites; Retry later or choose Skip.
- **Files open in the wrong app**: hyprtrixie picks imv, Evince, VLC and Ark once, then leaves your choices alone. Right-click a file in Dolphin → Open With → pick the app and tick "Remember", or `xdg-mime default org.gnome.Evince.desktop application/pdf`.
- **An app you removed with apt comes back**: it's still ticked in the setup menu's Applications, so the next install puts it back. Untick it there, or remove it with `sudo hyprtrixie app remove <app>`, which does both.

## Updating hyprtrixie

- **"run it as yourself, without sudo"**: `hyprtrixie update` asks for your password when the install starts.
- **"git pull failed"**: the folder has changes of your own (`git -C <folder> status` lists them; `git -C <folder> stash` puts them aside), or it's on a branch with no upstream (`git -C <folder> switch main`).
- **Read the notes again**: `hyprtrixie changes`, or Menu › Update › What changed.

## Boot

- **Text instead of the splash**: Esc switches between the splash and the boot messages. If it's always text:
  - **Boot splash** must be on in the setup menu.
  - `grep -o splash /proc/cmdline` must print `splash`.
  - `plymouth-set-default-theme` must print `hyprtrixie`.
  - `sudo update-initramfs -u` rebuilds the boot image; restart after it.
- **No dots when typing the disk password**: press Esc and type it at the text prompt.
- **The splash or the boot menu kept the old colours**: after a theme switch, the boot image is rebuilt in the background, which takes about a minute. `systemctl status hyprtrixie-follow-theme.service` shows the last run, and `sudo hyprtrixie-follow-theme` runs it now.
- **The boot menu never shows**: it's hidden for 3 seconds. Press Esc (or hold Shift) right after the laptop's logo.

## Login screen

- **It still looks like Catppuccin or plain SDDM**: `grep Current /etc/sddm.conf.d/10-hyprtrixie.conf` must say `Current=hyprtrixie`. Redo the module with `sudo hyprtrixie install --only 07`.
- **It didn't follow a theme or wallpaper change**: see the boot entry above; the same service paints the login screen.

## Network

- **No Wi-Fi after the setup**: NetworkManager took over the Wi-Fi from the installer.
  - Connect again with `nmcli device wifi connect "<network>" --ask`, or from the tray icon.
  - The old config is in `/etc/network/interfaces.hyprtrixie-old`.
- **Something can't reach this machine**: the firewall blocks incoming connections. Allow a port with `sudo ufw allow 22/tcp`, or turn the firewall off in the menu.
- **SSH into this machine stopped working**: with the SSH option on "client", the SSH server doesn't start at boot. Choose "leave" in the menu, or run `sudo systemctl enable --now ssh`.

## Sound, camera, sleep

- **No sound on a new Intel laptop**: the audio firmware (SOF) comes from backports. Reboot once after the setup, then check that `aplay -l` lists a card. On Arrow Lake and Lunar Lake, support depends on the kernel; note what `dmesg | grep -i sof` says.
- **The webcam doesn't work**: many 2024–2025 laptops use MIPI (IPU6/IPU7) cameras that Linux doesn't fully support yet.
- **It doesn't wake from suspend**:
  - Update the firmware with `fwupdmgr refresh && fwupdmgr update`.
  - If the firmware has a sleep state setting (ThinkPad: Config → Power), try "Linux".
  - Note what `cat /sys/power/mem_sleep` shows.

## NVIDIA

- The driver is built for every installed kernel (DKMS). `nvidia-smi` should list the card after a reboot.
- If the build failed, the setup says so in module 03. Check `dkms status`, and that Secure Boot is off.
- To use the open-source driver instead, choose **NVIDIA driver → nouveau** in the menu.

## Something broke after an update

1. Reboot and pick the last good snapshot from the snapshots submenu in the boot menu.
2. If everything works there, make it permanent with `sudo snapper-rollback <number>`, then reboot.

Rollback only touches the system. Your home folder and logs stay as they are.

## The laptop doesn't boot after installing Debian

See "Troubleshooting" in the install guide: reinstall GRUB from rescue mode, or install GRUB to the removable media path.

## Files ending in .hyprtrixie-new or .hyprtrixie-old

- `.hyprtrixie-new`: you edited a file that hyprtrixie manages, and there's a newer version. Yours is untouched; compare the two and merge what you want, then delete the `.hyprtrixie-new` file.
- `.hyprtrixie-old`: a file hyprtrixie had to replace, kept so you can go back.
