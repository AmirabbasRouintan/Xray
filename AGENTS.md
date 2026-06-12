# Agents Guide: Xray Manager

## Project summary
This repository contains a Go-based terminal UI (TUI) for managing Xray proxy configurations and a Tor-based IP changer utility. The main entry point (`main.go`) provides the interactive UI for creating, saving, and selecting Xray configurations. The `ipchanger.go` module implements a Tor setup and IP rotation workflow that can be launched from the TUI.

## Key components
- `main.go`: Bubble Tea TUI for managing Xray configs, launching Tor IP changer, and controlling the Xray process.
- `ipchanger.go`: Tor installer/configurer and IP rotation tool, used by the TUI when running in IP changer mode.
- `go run .`: Run the app from source.
- `xray-manager`: Built Go binary of the TUI (ELF executable).
- `go.mod` / `go.sum`: Go module metadata and dependency lock files.

## Runtime behavior
- The TUI starts with `main()` in `main.go` and uses Bubble Tea for state handling.
- Configs are stored in `~/.config/xray-configs` as JSON files.
- Active config is expected at `~/.config/xray/config.json` (symlink or file contents).
- The TUI can launch the Tor IP changer; when run with `--ipchanger-mode`, it executes the Tor workflow directly.

## Main TUI flows
- **Connect to Server**: Lists stored configs and starts Xray with the selected config by running `xray -config <path>`.
- **Add New Config**:
  - Paste config: accepts `vless://` URL or JSON, validates, generates a config name, and saves to `~/.config/xray-configs`.
  - Manual SOCKS: collects SOCKS host/port/auth, creates a config summary, and saves on request.
- **Manage Configs**: Displays configs and active status (future expansion point).
- **IP Changer**: Exits the TUI and launches the Tor IP changer (requires root privileges).

## Tor IP changer workflow (`ipchanger.go`)
- Detects distro, installs Tor if missing, configures `torrc`, and starts/restarts the Tor service.
- Uses a SOCKS5 proxy to query current IP from `https://checkip.amazonaws.com`.
- Supports fixed or infinite IP rotation with interval prompts.

## Notes for agents
- Use `go run .` for local execution.
- Root privileges are required for Tor setup and IP changer operations.
- The file `fix_analyze.patch` includes a potential refactor for parsing config inputs; it is not applied by default.
- The `xray-manager` binary is derived from this source; prefer editing Go files instead of the binary.

## Common commands
- `go run .`: Run the TUI from source.
- `./xray-manager`: Run the compiled binary.

## TODOs / extension points
- Implement active config management and config details view in `main.go` (`updateManageConfigs`).
- Expand `convertVlessToJSON` to support more VLESS parameters.
- Add validation/serialization for SOCKS configs into valid Xray JSON.
