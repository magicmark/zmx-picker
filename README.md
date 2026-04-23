# zmx-picker

Browse and attach to [zmx](https://github.com/neurosnap/zmx) sessions with a split-pane TUI.

- 📋 Session list with client count badges
- 👀 Live scrollback preview with ANSI colors
- ▶️ Hit Enter to attach instantly

## Install

```bash
go install github.com/magicmark/zmx-picker@latest
```

...or download a binary from [GitHub Releases](https://github.com/magicmark/zmx-picker/releases):

```bash
# macOS (Apple Silicon)
curl -L -o zmx-picker "https://github.com/magicmark/zmx-picker/releases/latest/download/zmx-picker-darwin-arm64"
chmod +x zmx-picker && mv zmx-picker /usr/local/bin/

# Linux (x86_64)
curl -L -o zmx-picker "https://github.com/magicmark/zmx-picker/releases/latest/download/zmx-picker-linux-amd64"
chmod +x zmx-picker && mv zmx-picker /usr/local/bin/
```

## Usage

```
zmx-picker
```

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate sessions |
| `Enter` | Attach to selected session |
| `q` / `Esc` | Quit |

## License

MIT
