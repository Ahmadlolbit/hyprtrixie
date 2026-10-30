# Keeping it healthy

## Updates

- Menu › Update › Debian packages, or `sudo apt update && sudo apt full-upgrade`.
- Menu › Update › hyprtrixie, or `hyprtrixie update`: the newest hyprtrixie, with a list of what changed.
- Menu › Update › Firmware for the laptop's firmware.

## Snapshots

Before every update, and every day, the system takes a snapshot of itself. If an update breaks something:

1. Restart, press Esc to show the boot menu, and pick a snapshot from before the update.
2. If everything works there, run `sudo snapper-rollback <number>` and restart.

Your files in your home folder are never rolled back.

## Backups

Snapshots live on the same disk. For real backups of your home folder:

```bash
restic-backup init /media/$USER/usb-drive/backups
restic-backup
```

## When something's wrong

`sudo hyprtrixie doctor`, or Menu › System › Check the system, looks for the usual problems, says what to do, and offers the safe fixes.
