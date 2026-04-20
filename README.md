<div align="center">

# expose-localhost

**Expose your localhost to the Internet with a single command.**

A tiny, cross-platform CLI written in Go — a friendly wrapper around `cloudflared` that gives you a public `*.trycloudflare.com` URL for any local service. No signup, no config, no Cloudflare account required.

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey)](#installation)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](#license)

</div>

---

## Features

- **Zero-config** — enter a host & port, hit Enter, get a public URL.
- **Beautiful TUI** — interactive form powered by [`huh`](https://github.com/charmbracelet/huh), styled result box with [`lipgloss`](https://github.com/charmbracelet/lipgloss), and a scroll-able log viewer.
- **Single binary** — one file, no Python, Node, or Docker runtime needed.
- **Auto-downloads `cloudflared`** — on first run it fetches the right binary for your OS/arch into a cache directory; subsequent runs start instantly.
- **Cross-platform** — Linux (amd64 / 386 / arm64 / arm), macOS (amd64 / arm64), Windows (amd64 / 386).
- **Shorthand mode** — pass `-h` and `-p` to skip the form; perfect for scripts.
- **One-key URL copy** — press `c` to copy the public URL to your clipboard.
- **Realtime log streaming** — press `tab` to toggle between the URL view and the live log view.

## Quick demo

```bash
$ expose-localhost
  expose-localhost  ·  cloudflared tunnel

? Local host  127.0.0.1
? Local port  3000
? Initial view  URL — show the public tunnel url

╭────────────────────────────────────────────╮
│  ✓ tunnel ready                            │
│                                            │
│  public   https://foo-bar-baz.trycloudflare.com
│  forward  127.0.0.1:3000                   │
╰────────────────────────────────────────────╯

  tab log   ·   c copy url   ·   q quit
```

## Installation

### Option 1 — Download a prebuilt release

Grab the binary for your OS from the [`dist/`](./dist) folder (or the Releases page if published).

#### Linux / macOS

```bash
# Linux amd64 (example)
curl -L -o expose-localhost \
  https://github.com/mwarevn/expose-localhost/releases/latest/download/expose-localhost-linux-amd64

chmod +x expose-localhost
sudo mv expose-localhost /usr/local/bin/

# run it
expose-localhost
```

Available targets:

| OS      | Arch    | File                                   |
| ------- | ------- | -------------------------------------- |
| Linux   | amd64   | `expose-localhost-linux-amd64`         |
| Linux   | 386     | `expose-localhost-linux-386`           |
| Linux   | arm64   | `expose-localhost-linux-arm64`         |
| Linux   | arm     | `expose-localhost-linux-arm`           |
| macOS   | amd64   | `expose-localhost-darwin-amd64`        |
| macOS   | arm64   | `expose-localhost-darwin-arm64`        |
| Windows | amd64   | `expose-localhost-windows-amd64.exe`   |
| Windows | 386     | `expose-localhost-windows-386.exe`     |

#### Windows (PowerShell)

```powershell
Invoke-WebRequest `
  -Uri https://github.com/mwarevn/expose-localhost/releases/latest/download/expose-localhost-windows-amd64.exe `
  -OutFile expose-localhost.exe

.\expose-localhost.exe
```

> **Note:** On first launch, the program downloads `cloudflared` (~30 MB) from Cloudflare's GitHub Releases into a cache directory (`$XDG_CACHE_HOME/expose-localhost/` on Linux, `~/Library/Caches/expose-localhost/` on macOS, `%LocalAppData%\expose-localhost\` on Windows). Make sure you have Internet access the first time you run it.

---

### Option 2 — Build from source

**Requirements:** Go 1.24 or newer.

```bash
# clone
git clone https://github.com/mwarevn/expose-localhost.git
cd expose-localhost

# pull dependencies
go mod tidy

# run directly (no build step)
go run .

# or build a local binary
go build -o expose-localhost .
./expose-localhost
```

#### Manual cross-compile

```bash
# Linux amd64
GOOS=linux   GOARCH=amd64 go build -o dist/expose-localhost-linux-amd64

# macOS arm64 (Apple Silicon)
GOOS=darwin  GOARCH=arm64 go build -o dist/expose-localhost-darwin-arm64

# Windows amd64
GOOS=windows GOARCH=amd64 go build -o dist/expose-localhost-windows-amd64.exe
```

#### Build every target at once

The included `build.sh` produces binaries for all 8 platforms into `dist/`:

```bash
./build.sh
```

Example output:

```
→ building dist/expose-localhost-linux-amd64          (6.8M)
→ building dist/expose-localhost-darwin-arm64         (6.6M)
→ building dist/expose-localhost-windows-amd64.exe    (7.0M)
...
✓ built 8 binaries in dist/
```

## Usage

### Interactive mode (default)

Run with no flags and the program will prompt for host, port, and initial view:

```bash
expose-localhost
```

Once the tunnel is up you'll see the public URL inside a bordered box.

**TUI keybindings:**

| Key          | Action                                         |
| ------------ | ---------------------------------------------- |
| `tab` / `l`  | Toggle between URL view and Log view           |
| `c` / `y`    | Copy the public URL to the clipboard           |
| `↑` `↓`      | Scroll the log up/down (Log view)              |
| `pgup/pgdn`  | Page through the log                           |
| `g` / `G`    | Jump to the top / bottom of the log            |
| `q` / `esc`  | Quit (or press `Ctrl+C` at any time)           |

### Shorthand (non-interactive) mode

Pass **both** `-h` and `-p` and the form is skipped — the tunnel starts immediately. Handy for scripts or systemd units:

```bash
# basic
expose-localhost -h 127.0.0.1 -p 3000

# long flags work too
expose-localhost --host 0.0.0.0 --port 8080

# also stream cloudflared logs to stdout
expose-localhost -h 127.0.0.1 -p 3000 --log
```

### Flags

| Flag               | Default        | Description                                         |
| ------------------ | -------------- | --------------------------------------------------- |
| `-h`, `--host`     | `127.0.0.1`    | Host or IP to expose                                |
| `-p`, `--port`     | `8080`         | Local port to forward (1 – 65535)                   |
| `--log`            | `false`        | Stream cloudflared logs to stdout (shorthand mode)  |

### Real-world examples

```bash
# expose a React dev server
npm run dev &
expose-localhost -h 127.0.0.1 -p 3000

# expose an API backend
expose-localhost -h 0.0.0.0 -p 8000

# expose PHP's built-in server
php -S 127.0.0.1:9000 &
expose-localhost -p 9000

# stop the tunnel: Ctrl+C
```

## How it works

```
┌─────────────┐      stdin form       ┌──────────────────┐
│    User     │─────────────────────▶│  expose-localhost │
└─────────────┘                       │   (Go binary)    │
                                      └──────┬───────────┘
                                             │ spawn
                                             ▼
                                      ┌──────────────────┐
                                      │   cloudflared    │
                                      │  tunnel --url    │
                                      └──────┬───────────┘
                                             │ pipe stdout/stderr
                                             ▼
                                      ┌──────────────────┐
                                      │  Scrape regex    │
                                      │ *.trycloudflare  │
                                      └──────┬───────────┘
                                             ▼
                                        🌐 Public URL
```

1. First run: download the right `cloudflared` binary for your OS/arch from Cloudflare's GitHub Releases.
2. Spawn `cloudflared tunnel --url <host>:<port>` as a child process.
3. Pipe the child's stdout & stderr, grep for `https://*.trycloudflare.com` with a regex.
4. Render the URL in a styled box; switch to a full TUI for log streaming and clipboard copy.
5. On `Ctrl+C` or TUI quit, kill the child process and clean up.

## Project layout

```
expose-localhost/
├── main.go          # entrypoint, flag parsing, input form, TUI orchestration
├── cloudflared.go   # cloudflared lifecycle: download, spawn, URL scraping
├── download.go      # HTTP download with progress reporting
├── tui.go           # Bubble Tea model — URL view, Log view, keybindings
├── build.sh         # cross-compile all 8 targets into dist/
├── go.mod / go.sum  # Go dependencies
└── dist/            # prebuilt binaries (git-ignored)
```

## System requirements

- **Runtime:** nothing — the binary is statically linked, no dynamic libc.
- **Build from source:** Go 1.24+.
- **Network:** Internet access on first run (to fetch `cloudflared`) and while the tunnel is active.

## FAQ

**Is the URL stable across runs?**
No. Cloudflare hands out a fresh random `*.trycloudflare.com` subdomain every run. If you need a stable URL, use a Named Tunnel with a Cloudflare account (outside the scope of this tool).

**Any bandwidth / traffic limits?**
Trycloudflare is Cloudflare's free tier and may be rate-limited if abused. Not recommended for production workloads.

**Can I skip the cloudflared download?**
Yes. If `cloudflared` already exists at `os.UserCacheDir()/expose-localhost/cloudflared`, the program uses it instead of re-downloading.

**Does the tool kill other `cloudflared` processes on my machine?**
No. It only manages the child process it spawned itself — your other `cloudflared` instances are left alone.

## License

MIT — see [LICENSE](./LICENSE) if provided.

## Acknowledgements

- [Cloudflare](https://github.com/cloudflare/cloudflared) — `cloudflared` powers the tunnel under the hood.
- [Charm](https://charm.sh) — `huh`, `bubbletea`, and `lipgloss` for the beautiful TUI.
