<div align="center">
  <pre style="font-size: 1.2em; line-height: 1.4; color: #8ec07c; font-weight: bold;">
██╗  ██╗██████╗  █████╗ ██╗   ██╗
╚██╗██╔╝██╔══██╗██╔══██╗╚██╗ ██╔╝
 ╚███╔╝ ██████╔╝███████║ ╚████╔╝
 ██╔██╗ ██╔══██╗██╔══██║  ╚██╔╝
██╔╝ ██╗██║  ██║██║  ██║   ██║
╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝
  </pre>
  <h1 style="color: #d3869b; font-size: 2.2em; margin: 0;">Xray Manager</h1>
  <p style="font-size: 1.1em; color: #83a598; margin-top: 5px;">
    <strong>A beautiful TUI for managing Xray proxy configurations with a built-in Tor IP changer</strong>
  </p>
  <br>
  <p>
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go" alt="Go Version">
    <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge&logo=license" alt="License">
    <img src="https://img.shields.io/badge/Platform-Linux-red?style=for-the-badge&logo=linux" alt="Platform">
    <img src="https://img.shields.io/badge/TUI-Bubble%20Tea-ff69b4?style=for-the-badge" alt="TUI">
  </p>
</div>

---

## ✨ Features

| Feature | Description |
|---------|-------------|
| 🖥️ **Interactive TUI** | Navigate, manage, and connect to Xray proxy configs using keyboard and mouse |
| 🔗 **Config Import** | Paste `vless://`, `vmess://`, `trojan://`, `ss://`, `hysteria2://` URLs or raw JSON |
| 🧦 **Manual SOCKS** | Create SOCKS proxy configs interactively step-by-step |
| 📡 **Subscription Manager** | Import, save, and manage subscription links |
| 📊 **Latency Testing** | Ping configs with TCP or real proxy latency measurement |
| 🌍 **Server Regions** | Auto-detect geographic regions via GeoIP and browse configs grouped by country |
| 🔄 **IP Changer** | Launch a Tor-based IP rotation tool with custom intervals |
| 🔍 **Config Details** | View full JSON, copy to clipboard, or export as shareable link |
| 🔎 **Search & Filter** | Search by name/server/protocol, filter by working/errored/unpinged |
| 🖱️ **Mouse Support** | Full mouse support for navigation and scrolling |

---

## 🚀 Installation

### Prerequisites

- **Go 1.25+** (for building from source)
- **Xray core binary** (`xray`) installed in your PATH ([download here](https://github.com/XTLS/Xray-core/releases))
- **Tor** (optional, for IP Changer — auto-installed if missing)

### Build from Source

```bash
git clone https://github.com/AmirabbasRouintan/Xray.git
cd Xray
go build -o xray-app .
```

### Run

```bash
./xray-app
```

Or directly from source:

```bash
go run .
```

---

## 🎮 Usage

### Main Menu

```
1  Connect        — Browse and connect to saved configs
2  Add Config     — Import new configs (paste URL/JSON, manual SOCKS, file)
3  Subscriptions  — Manage subscription URLs
4  Settings       — Configure HTTP/SOCKS ports and ping timeout
5  IP Changer     — Launch Tor IP rotation tool
6  Xray Info      — View Xray core status and version
7  Sorter         — Browse configs grouped by geographic region
8  Quit           — Exit the application
```

### Connect Screen

| Key | Action |
|-----|--------|
| `↑/↓` or `k/j` | Navigate configs |
| `Enter` | Connect to selected config |
| `Space` | Toggle multi-select |
| `Ctrl+A` | Select all |
| `s` | Set as active config |
| `i` | View config details |
| `c` | Copy config to clipboard |
| `p` | Ping selected config |
| `P` | Ping all configs |
| `t` | Open shell with proxy env |
| `d` | Delete selected |
| `a` | Delete all |
| `D` | Delete duplicates |
| `E` | Delete errored configs |
| `/` | Search configs |
| `f` | Filter (all/working/errored/unpinged) |
| `[` / `]` | Page up / down |
| `Mouse wheel` | Scroll through configs |

### Sorter Screen (Regions)

| Key | Action |
|-----|--------|
| `←/→` or `Tab/Shift+Tab` | Switch between regions |
| `↑/↓` or `k/j` | Navigate configs |
| `Space` | Toggle selection |
| `Ctrl+A` | Select all in region |
| `Enter` | Connect to selected config |
| `c` | Copy config to clipboard |
| `r` | Re-detect regions |
| `/` | Search/filter regions |

### Add Config Screen

| Option | Description |
|--------|-------------|
| **Paste Config** | Supports `vless://`, `vmess://`, `trojan://`, `ss://`, `hysteria2://` URLs and JSON (press `Ctrl+Enter` to analyze) |
| **Manual SOCKS** | Step-by-step SOCKS proxy creation (IP, port, optional auth) |
| **Load JSON File** | Import from a local `.json` file |

### Subscription Manager

| Key | Action |
|-----|--------|
| `Enter` | Import and save subscription |
| `Tab` | Toggle between URL input and saved subscriptions |
| `d` | Delete selected subscription |
| `Ctrl+V` | Paste URL |
| `↑/↓` | Navigate saved subscriptions |

### Settings

| Setting | Default | Description |
|---------|---------|-------------|
| HTTP Port | `10808` | Local HTTP inbound port when connecting |
| SOCKS Port | `10809` | Local SOCKS inbound port when connecting |
| Ping Timeout | `8s` | Timeout per ping request |

---

## 📁 Configuration

Configs are stored in `~/.config/xray-configs/` as individual JSON files.

The active config is symlinked at `~/.config/xray/config.json`.

Settings, ping cache, and region cache are saved to:
- `~/.config/xray-configs/settings.json`
- `~/.config/xray-configs/ping_cache.json`
- `~/.config/xray-configs/region_cache.json`

---

## 🔄 IP Changer (Tor)

The Tor-based IP changer requires root privileges and supports:

- **Automatic Tor installation** — Detects your Linux distro and installs Tor
- **Custom intervals** — Set time between IP changes (in seconds)
- **Fixed or infinite rotation** — Choose how many times to change IP
- **Current IP display** — View your Tor exit node IP address

### Usage

```bash
./xray-app         # Select "IP Changer" from menu
sudo ./xray-app --ipchanger-mode    # Run in IP changer mode directly
```

**Supported Distributions:**
- Ubuntu, Debian (apt-get)
- Fedora, CentOS, RHEL, Amazon Linux (yum)
- Arch, Manjaro (pacman)

---

## 🧠 Supported Protocols

| Protocol | URL Format | Status |
|----------|-----------|--------|
| **VLESS** | `vless://` | ✅ Full support |
| **VMess** | `vmess://` | ✅ Full support |
| **Trojan** | `trojan://` | ✅ Full support |
| **Shadowsocks** | `ss://` | ✅ Full support |
| **Hysteria2** | `hysteria2://` / `hy2://` | ✅ Full support |
| **JSON Config** | Raw JSON | ✅ Full support |
| **SOCKS** | Manual entry | ✅ Full support |

---

## 🛠️ Tech Stack

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — Go framework for terminal user interfaces
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** — Style definitions for terminal apps
- **[Bubbles](https://github.com/charmbracelet/bubbles)** — TUI components (spinner, textarea, textinput)
- **[Catppuccin Jade](https://catppuccin.com/)** — Beautiful color palette
- **[Xray-core](https://github.com/XTLS/Xray-core)** — The proxy core

---

## 📸 Screenshots

```
┌──────────────────────────────────────────────────────────────┐
│        ██╗  ██╗██████╗  █████╗ ██╗   ██╗                    │
│        ╚██╗██╔╝██╔══██╗██╔══██╗╚██╗ ██╔╝                    │
│         ╚███╔╝ ██████╔╝███████║ ╚████╔╝                     │
│         ██╔██╗ ██╔══██╗██╔══██║  ╚██╔╝                      │
│        ██╔╝ ██╗██║  ██║██║  ██║   ██║                       │
│        ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝                       │
│ Core: RUNNING  Active: my_config  |  14:30:25                │
├──────────────────────────────────────────────────────────────┤
│                     Connect to Server                        │
│                                                              │
│  > 1 ● vless_US_443_143025  (192.168.1.1:443)  TCP:45ms      │
│    2 ● vmess_DE_443_143025  (10.0.0.1:443)     TCP:89ms      │
│    3   trojan_JP_443_143025 (203.0.113.1:443)  TCP:ERR       │
│    4 ● ss_HK_8388_143025    (198.51.100.1:8388) TCP:120ms    │
│                                                              │
│ ─────────────────────────●──────────────────── 4/4 (100%)    │
│ Filter: all (4/4)  [f: change]                               │
│                                                              │
│ ↑/↓: Navigate | Enter: Connect | p: Ping | /: Search | ...  │
└──────────────────────────────────────────────────────────────┘
```

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! Feel free to check the [issues page](https://github.com/AmirabbasRouintan/Xray/issues).

---

## 📄 License

This project is [MIT](LICENSE) licensed.

---

<div align="center">
  <br>
  <p style="font-size: 1.3em; color: #d3869b;">
    ⭐ If you found this project useful, please give it a star!
  </p>
  <p style="color: #83a598;">
    It took a lot of time and effort to build this project — your support means a lot ❤️
  </p>
  <br>
  <p>
    <a href="https://github.com/AmirabbasRouintan/Xray">
      <img src="https://img.shields.io/github/stars/AmirabbasRouintan/Xray?style=for-the-badge&logo=github&color=yellow" alt="Stars">
    </a>
    <a href="https://github.com/AmirabbasRouintan/Xray/issues">
      <img src="https://img.shields.io/github/issues/AmirabbasRouintan/Xray?style=for-the-badge&logo=github" alt="Issues">
    </a>
  </p>
  <br>
  <sub>Built with ❤️ using Go and Bubble Tea</sub>
</div>
