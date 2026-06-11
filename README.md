<div align="center">
  <h2>
    <span style="color: #d3869b; font-family: monospace; font-size: 1.6em;">❝ Xray Manager ❞</span>
  </h2>
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

<div align="center">

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

</div>

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

### Add Config

<div align="center">

| Option | Description |
|--------|-------------|
| **Paste Config** | Paste `vless://`, `vmess://`, `trojan://`, `ss://`, `hysteria2://` URLs or JSON |
| **Manual SOCKS** | Step-by-step SOCKS proxy creation |
| **Load JSON File** | Import from a local `.json` file |

</div>

### Settings

<div align="center">

| Setting | Default | Description |
|---------|---------|-------------|
| HTTP Port | `10808` | Local HTTP inbound port when connecting |
| SOCKS Port | `10809` | Local SOCKS inbound port when connecting |
| Ping Timeout | `8s` | Timeout per ping request |

</div>

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

<div align="center">

| Protocol | URL Format | Status |
|----------|-----------|--------|
| **VLESS** | `vless://` | ✅ Full support |
| **VMess** | `vmess://` | ✅ Full support |
| **Trojan** | `trojan://` | ✅ Full support |
| **Shadowsocks** | `ss://` | ✅ Full support |
| **Hysteria2** | `hysteria2://` / `hy2://` | ✅ Full support |
| **JSON Config** | Raw JSON | ✅ Full support |
| **SOCKS** | Manual entry | ✅ Full support |

</div>

---

## 🛠️ Tech Stack

- **[Bubble Tea](https://github.com/charmbracelet/bubbletea)** — Go framework for terminal user interfaces
- **[Lipgloss](https://github.com/charmbracelet/lipgloss)** — Style definitions for terminal apps
- **[Bubbles](https://github.com/charmbracelet/bubbles)** — TUI components (spinner, textarea, textinput)
- **[Catppuccin Jade](https://catppuccin.com/)** — Beautiful color palette
- **[Xray-core](https://github.com/XTLS/Xray-core)** — The proxy core


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
  
</div>
