# The menu

Super + Alt + Space opens it, and Super + Escape opens its System page.

- Arrows or the mouse move, Enter opens, Backspace goes back, Escape closes.
- Typing searches every page at once.

| Page | What's on it |
| --- | --- |
| Apps | every installed app |
| Learn | this guide, the key sheet, the welcome screen |
| Capture | screenshots |
| Toggle | screensaver, stay awake, night light, do not disturb, Wi-Fi, Bluetooth |
| Style | theme, background |
| Setup | Wi-Fi, audio, brightness, Bluetooth, power |
| Install, Remove | apps, or any Debian package |
| Update | hyprtrixie, Debian, firmware, what changed |
| About | this machine and the hyprtrixie version |
| System | check the system, lock, suspend, log out, restart, shut down |

On the Toggle page, a filled dot means on.

## Your own entries

Put them in `~/.config/hyprtrixie/menu.json`:

```json
{
  "work": { "label": "Work" },
  "work.vpn": { "label": "Office VPN", "action": "nmcli connection up office" }
}
```

The dots in an id make the pages: `work.vpn` sits on the `work` page. `action` is any shell command. Changes show the next time the menu opens.
