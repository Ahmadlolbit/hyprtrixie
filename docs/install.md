# Installing Debian 13 for hyprtrixie

This takes a laptop from empty disk to a Debian 13 base that hyprtrixie can turn into the full desktop. The disk is wiped.

---

## What you need

- A 64-bit PC (UEFI). Tested target: ThinkPad, Core Ultra 7 255H, 16 GB.
- A USB stick of at least 1 GB, and a second one only if the installer can't download the layout helper (see step 5).
- Internet during the install (Ethernet or Wi-Fi).
- About 64 GB of disk at minimum; 128 GB or more is comfortable with snapshots.
- Your data backed up elsewhere.

## 1. Download Debian

1. Go to https://www.debian.org/CD/netinst/ and download `debian-13.x.y-amd64-netinst.iso` (amd64).
2. From the same folder, download `SHA512SUMS` and `SHA512SUMS.sign`.
3. Check it on Linux:

   ```bash
   sha512sum -c SHA512SUMS --ignore-missing
   ```

   It must print `debian-13.x.y-amd64-netinst.iso: OK`.

The official image already contains the non-free firmware most laptops need for Wi-Fi.

## 2. Write the USB stick

- Linux: `sudo cp debian-13.*-amd64-netinst.iso /dev/sdX && sync` (replace `sdX` with the stick, check with `lsblk`; everything on it is erased).
- Windows: Rufus, pick the ISO, and when asked choose **DD image mode**.
- macOS: balenaEtcher.

## 3. ThinkPad firmware settings

Power on and press **F1** for the setup.

- Security → Secure Boot → **Off**. (Drivers built on the machine, such as NVIDIA's, are not signed.)
- Startup → UEFI/Legacy Boot → **UEFI Only**.
- Save with F10.

Power on again with the stick in, press **F12** and pick the USB drive.

## 4. Start the expert installer

In the boot menu: **Advanced options → Graphical expert install**. Expert mode returns to a main menu between steps; that pause is what step 5 needs.

Go through the menu items in order:

| Screen | Choice |
| --- | --- |
| Choose language / location / locale | your language and country, locale `en_US.UTF-8` (or yours) |
| Configure the keyboard | your layout (hyprtrixie copies it to the desktop later) |
| Detect and mount installation media | Continue |
| Load installer components | nothing extra, Continue |
| Detect network hardware | Continue |
| Configure the network | Wi-Fi: pick your network, WPA passphrase. Hostname: e.g. `thinkpad`. Domain: empty |
| Set up users and passwords | Shadow passwords: Yes. **Allow login as root: No** (so your account gets sudo). Then your full name, user name and password |
| Configure the clock | NTP: Yes, then your time zone |
| Detect disks | Continue |

### Partition disks

Choose **Manual**, then:

1. Select the whole disk (e.g. `nvme0n1`) → create a new empty partition table → **GPT**.
2. Free space → Create a new partition → `1 GB` → Beginning → Use as: **EFI System Partition** → Done.
3. Free space → Create → `1 GB` → Use as: **Ext4** → Mount point: **/boot** → Done.
4. Free space → Create → the rest of the disk → Use as: **physical volume for encryption** → Erase data: No (faster on an SSD) → Done.
5. **Configure encrypted volumes** → Yes → Create encrypted volumes → select the partition from step 4 → Finish → type the disk passphrase twice. You'll type it at every boot.
6. Select the new encrypted volume (`nvme0n1p3_crypt`) → its partition → Use as: **btrfs journaling file system** → Mount point: **/** → Mount options: **noatime** → Done.
7. **Finish partitioning and write changes to disk**. It warns that there's no swap: answer **No** (don't go back; hyprtrixie makes swap later). Write the changes: **Yes**.

You are back at the main menu. **Do not start "Install the base system" yet.**

## 5. Create the btrfs layout

The installer put everything in one subvolume called `@rootfs`. This step turns it into `@`, `@home`, `@snapshots`, `@var_log` and `@swap`, so a snapshot rollback never touches your files or logs.

1. Press **Ctrl+Alt+F2**, then Enter for a shell.
2. Get the helper, whichever works:
   - Download it:

     ```sh
     wget -O /tmp/layout.sh https://raw.githubusercontent.com/Ahmadlolbit/hyprtrixie/main/installer/btrfs-layout.sh
     sh /tmp/layout.sh
     ```

   - Or from a second USB stick (FAT) with `btrfs-layout.sh` on it:

     ```sh
     mkdir /mnt/usb && mount /dev/sdb1 /mnt/usb
     sh /mnt/usb/btrfs-layout.sh
     ```

3. It prints the new mounts and fstab. Check that `/`, `/home`, `/.snapshots`, `/var/log`, `/swap` and `/boot` are all listed.
4. Press **Ctrl+Alt+F5** to get back to the graphical installer.

If you skip this step, hyprtrixie still works and still takes snapshots, but `/home` lives inside them, so a rollback would also roll back your files.

## 6. Finish the install

| Screen | Choice |
| --- | --- |
| Install the base system | Kernel: `linux-image-amd64`. Drivers in the initrd: generic |
| Configure the package manager | Network mirror: Yes. **Protocol: https**. Mirror: `deb.debian.org`. Proxy: empty. Non-free firmware: Yes. Non-free software: Yes. Contrib: Yes. Source repositories: No. Services: security updates and release updates |
| Select and install software | Popularity contest: your call. Software: **only "standard system utilities"** (untick the desktop, GNOME and SSH server) |
| Install the GRUB boot loader | Yes. Force installation to the EFI removable media path: No (see troubleshooting if it doesn't boot). Update NVRAM: Yes |
| Finish the installation | System clock is UTC: Yes. Remove the stick, Continue |

HTTPS matters on some networks: plain HTTP mirrors can hand out corrupted indexes behind caching proxies.

## 7. First boot

1. GRUB starts Debian, then asks for the disk passphrase.
2. Log in on the text console with your account.
3. Check the network: `ping -c 3 deb.debian.org`.

## 8. Run hyprtrixie

```bash
sudo apt install git
git clone https://github.com/Ahmadlolbit/hyprtrixie.git
cd hyprtrixie
sudo ./hyprtrixie
```

If you gave root a password in step 4, your account has no sudo yet. Run `su -`, go to the clone and start `./hyprtrixie` as root; it sets up sudo for your account.

The menu opens. Look through the sections, change what you like, then choose **Install**. The first run takes a while (downloads, kernel, desktop, apps). If a step fails you'll see its last lines and can Retry, Skip, Continue or Halt.

When it's done: `sudo reboot`, log in on the login screen, open a terminal and run

```bash
cd ~/hyprtrixie && sudo ./hyprtrixie verify
```

## Where things end up

| What | Where |
| --- | --- |
| Your choices | `/etc/hyprtrixie/hyprtrixie.conf` |
| Logs | `/var/log/hyprtrixie/` and `~/runbook-logs/runbook.log` |
| Report | `~/runbook-report.md` |

## Troubleshooting

- **Doesn't boot after the install (goes straight to the firmware)**: some firmware ignores the boot entry the installer made. Boot the stick, Advanced options → Rescue mode, unlock the disk when asked, then choose "Reinstall GRUB boot loader". If that still doesn't help, install again and answer **Yes** to "Force installation to the EFI removable media path".
- **`wget` fails in step 5**: the installer's `wget` may not speak HTTPS. Use the second USB stick.
- **No Wi-Fi in the installer**: use a phone (USB tethering) or Ethernet for the install; hyprtrixie installs newer firmware later.
