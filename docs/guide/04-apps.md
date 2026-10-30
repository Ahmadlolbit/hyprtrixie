# Apps

Super + Space finds and opens apps. It also does sums (`2+2`) and searches with prefixes:

| Start with | To search |
| --- | --- |
| `/` | files |
| `:` | the clipboard history |
| `.` | symbols and emoji |
| `@` | the web |
| `>` | a command to run |

## Installing and removing

Menu › Install and Menu › Remove list the apps hyprtrixie knows, by name. Or from a terminal:

```bash
hyprtrixie app list
sudo hyprtrixie app install gimp
sudo hyprtrixie app remove gimp
```

**Any Debian package** on those pages searches all of Debian.

## Which app opens a file

Images open in imv, PDFs in Evince, videos and music in VLC, archives in Ark. To change one, right-click a file in Dolphin and use Open With.
