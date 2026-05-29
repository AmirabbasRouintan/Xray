package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the application state
type model struct {
	screen           string
	cursor           int
	pasteArea        textarea.Model
	analysis         string
	configBuffer     string // Raw JSON to save
	currentName      string // Name for the config
	resolvedJSONPath string
	savePath         string
	configs          []ConfigInfo
	selectedItem     int
	selectedConfigs  map[int]bool
	configDetail     string
	configDetailNote string
	xrayBinaryPath   string
	xrayVersion      string
	xrayBinaryError  string

	// Manual SOCKS fields
	socksStep       int
	socksIP         textinput.Model
	socksPort       textinput.Model
	socksUsername   textinput.Model
	socksPassword   textinput.Model
	jsonPathInput   textinput.Model
	subscriptionURL   textinput.Model
	settingsHTTPPort     int
	settingsSOCKSPort    int
	settingsPingTimeout  int
	settingsCursor       int
	settingsInput        textinput.Model
	settingsEditing      bool
	savedSubs         []SubscriptionInfo
	selectedSubIndex  int
	showSubsList      bool
	spinner           spinner.Model
	isLoading       bool
	loadingText     string
	loadingDone     func(model) model
	pinging         bool
	latencyTotal    int
	latencyDone     int
	latencyCh       chan tea.Msg
	scrollOffset    int
	termWidth       int
	termHeight      int
	filter          string
	showFilter      bool
	searchInput     textinput.Model
	showSearch      bool
}

type ConfigInfo struct {
	Name     string
	Path     string
	Active   bool
	Protocol string
	Server   string
	Port     int
	Ping     string
}

type SubscriptionInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Date string `json:"date"`
}

type connectionResult struct {
	success bool
	message string
}

type vmessConfig struct {
	ADD  string `json:"add"`
	AID  int    `json:"aid,string"`
	Host string `json:"host"`
	ID   string `json:"id"`
	NET  string `json:"net"`
	Path string `json:"path"`
	Port int    `json:"port"`
	PS   string `json:"ps"`
	SCY  string `json:"scy"`
	TLS  string `json:"tls"`
	Type string `json:"type"`
	V    string `json:"v"`
}

// Additional config types for different proxy protocols
type trojanConfig struct {
	Password string `json:"password"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Type     string `json:"type,omitempty"`
	Security string `json:"security,omitempty"`
	SNI      string `json:"sni,omitempty"`
	Path     string `json:"path,omitempty"`
	Host     string `json:"host,omitempty"`
	Remark   string `json:"ps,omitempty"`
}

type shadowsocksConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Method   string `json:"method"`
	Password string `json:"password"`
	Plugin   string `json:"plugin,omitempty"`
	Remark   string `json:"ps,omitempty"`
}

// Catppuccin Jade Color Palette
var (
	// Base colors
	catppuccinBackground = "#1d1d1d"
	catppuccinForeground = "#fff4d2"
	catppuccinSelection  = "#8ec07c"
	catppuccinCursor     = "#fff4d2"
	catppuccinInactive   = "#393939"
	catppuccinGreen      = "#8ec07c"
	catppuccinYellow     = "#d8a657"
	catppuccinBlue       = "#83a598"
	catppuccinPink       = "#d3869b"
	catppuccinRed        = "#FF4A4A"
)

// Styles with Catppuccin Jade colors
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(catppuccinGreen)).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(catppuccinGreen))

	menuItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinForeground))

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(catppuccinForeground)).
				Background(lipgloss.Color(catppuccinInactive)).
				Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinInactive)).
			Padding(1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinGreen)).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinRed)).
			Bold(true)

	// Additional Catppuccin Jade styles
	accentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinPink))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinBlue))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinYellow)).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(catppuccinInactive))
)

// ASCII Art
const xrayArt = `██╗  ██╗██████╗  █████╗ ██╗   ██╗
╚██╗██╔╝██╔══██╗██╔══██╗╚██╗ ██╔╝
 ╚███╔╝ ██████╔╝███████║ ╚████╔╝ 
 ██╔██╗ ██╔══██╗██╔══██║  ╚██╔╝  
██╔╝ ██╗██║  ██║██║  ██║   ██║   
╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝`

func initialModel() model {
	// Initialize textarea for paste config (multi-line)
	ta := textarea.New()
	ta.Placeholder = "Paste your proxy URL or JSON config here...\n\nSupports:\n• vless:// URLs\n• vmess:// URLs\n• trojan:// URLs\n• ss:// URLs (Shadowsocks)\n• JSON configurations\n• Multi-line content"
	ta.Focus()
	ta.SetWidth(70)
	ta.SetHeight(12)
	ta.ShowLineNumbers = false

	// Style the textarea with Catppuccin colors
	ta.FocusedStyle.Base = ta.FocusedStyle.Base.BorderForeground(lipgloss.Color(catppuccinGreen))
	ta.BlurredStyle.Base = ta.BlurredStyle.Base.BorderForeground(lipgloss.Color(catppuccinInactive))

	// Initialize SOCKS textinputs
	socksIP := textinput.New()
	socksIP.Placeholder = "192.168.1.100"
	socksIP.CharLimit = 15
	socksIP.Width = 20

	socksPort := textinput.New()
	socksPort.Placeholder = "1080"
	socksPort.CharLimit = 5
	socksPort.Width = 10

	socksUsername := textinput.New()
	socksUsername.Placeholder = "username (optional)"
	socksUsername.CharLimit = 50
	socksUsername.Width = 30

	socksPassword := textinput.New()
	socksPassword.Placeholder = "password (optional)"
	socksPassword.EchoMode = textinput.EchoPassword
	socksPassword.CharLimit = 50
	socksPassword.Width = 30

	jsonPathInput := textinput.New()
	jsonPathInput.Placeholder = "/path/to/config.json"
	jsonPathInput.CharLimit = 512
	jsonPathInput.Width = 60

	subscriptionURL := textinput.New()
	subscriptionURL.Placeholder = "https://example.com/subscription"
	subscriptionURL.CharLimit = 512
	subscriptionURL.Width = 60

	searchInput := textinput.New()
	searchInput.Placeholder = "Search configs by name, server, or protocol..."
	searchInput.CharLimit = 100
	searchInput.Width = 60

	settingsInput := textinput.New()
	settingsInput.CharLimit = 10
	settingsInput.Width = 15

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = accentStyle

	return model{
		screen:          "main",
		pasteArea:       ta,
		socksIP:         socksIP,
		socksPort:       socksPort,
		socksUsername:   socksUsername,
		socksPassword:   socksPassword,
		jsonPathInput:   jsonPathInput,
		subscriptionURL: subscriptionURL,
		searchInput:       searchInput,
		settingsInput:     settingsInput,
		settingsHTTPPort:    loadSettingsInt("http_port", 10808),
		settingsSOCKSPort:   loadSettingsInt("socks_port", 10809),
		settingsPingTimeout: loadSettingsInt("ping_timeout", 8),
		savedSubs:         loadSubscriptions(),
		selectedSubIndex:  0,
		showSubsList:      false,
		spinner:           spin,
		savePath:        filepath.Join(os.Getenv("HOME"), ".config", "xray", "config.json"),
		configs:         loadConfigs(),
		selectedConfigs: map[int]bool{},
		filter:          "all",
	}
}

func (m model) Init() tea.Cmd { // Remove pointer receiver
	return tea.Batch(textarea.Blink, m.startLoading("Starting...", 350*time.Millisecond, func(next model) model {
		next = populateXrayInfo(next)
		next = next.applyResponsiveLayout()
		return next
	}, false))
}

type startLoadingMsg struct {
	text     string
	duration time.Duration
	apply    func(model) model
	quit     bool
}

type loadingFinished struct {
	apply func(model) model
	quit  bool
}

type latencyStarted struct {
	total   int
	configs []ConfigInfo
	indices []int // nil = ping all configs
	timeout time.Duration
}

type latencyResult struct {
	server string
	port   int
	ping   string
}

type latencyFinished struct{}

type shellSessionEnded struct {
	config string
}

func (m model) startLoading(text string, duration time.Duration, apply func(model) model, quit bool) tea.Cmd {
	return func() tea.Msg {
		return startLoadingMsg{text: text, duration: duration, apply: apply, quit: quit}
	}
}

func (m model) readLatencyResult() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-m.latencyCh
		if !ok {
			return latencyFinished{}
		}
		return msg
	}
}

func runLatencyWorkers(configs []ConfigInfo, ch chan<- tea.Msg, indices []int, timeout time.Duration) {
	defer close(ch)
	binary, err := findXrayBinary()
	if err != nil {
		pingFrom := indices
		if pingFrom == nil {
			pingFrom = make([]int, len(configs))
			for i := range configs {
				pingFrom[i] = i
			}
		}
		for _, idx := range pingFrom {
			ch <- latencyResult{server: configs[idx].Server, port: configs[idx].Port, ping: "ERR"}
		}
		return
	}

	const wCount = 10
	jobs := make(chan struct {
		idx    int
		server string
		port   int
	})
	var wg sync.WaitGroup
	for i := 0; i < wCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				latency, err := runXrayLatency(binary, configs[job.idx].Path, timeout, job.server, job.port)
				ping := "ERR"
				if err == nil {
					ping = latency
				}
				ch <- latencyResult{server: job.server, port: job.port, ping: ping}
			}
		}()
	}

	go func() {
		pingFrom := indices
		if pingFrom == nil {
			pingFrom = make([]int, len(configs))
			for i := range configs {
				pingFrom[i] = i
			}
		}
		for _, idx := range pingFrom {
			jobs <- struct {
				idx    int
				server string
				port   int
			}{idx: idx, server: configs[idx].Server, port: configs[idx].Port}
		}
		close(jobs)
	}()

	wg.Wait()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // Remove pointer receiver
	switch msg := msg.(type) {
	case connectionResult:
		if msg.success {
			m.analysis = successStyle.Render(msg.message)
		} else {
			m.analysis = errorStyle.Render(msg.message)
		}
		return m, nil
	case startLoadingMsg:
		m.isLoading = true
		m.loadingText = msg.text
		m.loadingDone = msg.apply
		return m, tea.Batch(m.spinner.Tick, tea.Tick(msg.duration, func(time.Time) tea.Msg {
			return loadingFinished{apply: msg.apply, quit: msg.quit}
		}))
	case loadingFinished:
		m.isLoading = false
		m.loadingText = ""
		if msg.apply != nil {
			m = msg.apply(m)
		}
		m.loadingDone = nil
		if msg.quit {
			return m, tea.Quit
		}
		return m, nil
	case latencyStarted:
		m.pinging = true
		m.latencyTotal = msg.total
		m.latencyDone = 0
		m.analysis = fmt.Sprintf("Pinging %d configs... [0/%d]", msg.total, msg.total)
		m.latencyCh = make(chan tea.Msg, msg.total)
		go runLatencyWorkers(msg.configs, m.latencyCh, msg.indices, msg.timeout)
		return m, m.readLatencyResult()
	case latencyResult:
		// Find config by server:port
		found := false
		for i := range m.configs {
			if m.configs[i].Server == msg.server && m.configs[i].Port == msg.port {
				m.configs[i].Ping = msg.ping
				found = true
				break
			}
		}
		if !found {
			return m, nil
		}
		if m.latencyCh != nil {
			m.latencyDone++
			m.analysis = fmt.Sprintf("Pinging... [%d/%d]", m.latencyDone, m.latencyTotal)
			return m, m.readLatencyResult()
		}
		m.analysis = fmt.Sprintf("✅ Ping: %s — %s:%d", msg.ping, msg.server, msg.port)
		// Save single ping result
		cache := loadPingCache()
		cache[fmt.Sprintf("%s:%d", msg.server, msg.port)] = msg.ping
		savePingCache(cache)
		return m, nil
	case latencyFinished:
		m.pinging = false
		cache := loadPingCache()
		for _, cfg := range m.configs {
			if cfg.Ping != "" {
				cache[fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)] = cfg.Ping
			}
		}
		savePingCache(cache)
		if m.latencyTotal > 0 {
			m.analysis = fmt.Sprintf("✅ Ping completed (%d configs)", m.latencyTotal)
		}
		m.latencyTotal = 0
		m.latencyDone = 0
		return m, nil
	case shellSessionEnded:
		m.analysis = successStyle.Render(fmt.Sprintf("✅ Shell proxy session ended. (%s)", msg.config))
		return m, nil
	case spinner.TickMsg:
		if m.isLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		m = m.applyResponsiveLayout()
		return m, nil
	case tea.MouseMsg:
		if m.isLoading || m.pinging || m.screen != "connectServer" {
			return m, nil
		}
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.selectedItem > 0 {
				m.selectedItem--
				m.ensureVisible()
			}
		case tea.MouseButtonWheelDown:
			flen := len(m.filteredConfigs())
			if m.selectedItem < flen-1 {
				m.selectedItem++
				m.ensureVisible()
			}
		case tea.MouseButtonLeft:
			visible, offset, total := m.visibleConfigs()
			if total == 0 {
				break
			}
			// Use bottom-up calculation for Y offsets
			// Bottom margin: ~7 lines (border bottom + help + scrollbar + empty + padding)
			bottomMargin := 7
			listStartY := m.termHeight - len(visible) - bottomMargin
			listEndY := listStartY + len(visible)
			if msg.Y >= listStartY && msg.Y < listEndY {
				idx := offset + (msg.Y - listStartY)
				if idx >= 0 && idx < total {
					m.selectedItem = idx
					m.ensureVisible()
				}
			} else if msg.Y == listEndY+1 {
				// Click on the scroll bar area — jump based on X position
				barWidth := m.termWidth - 30
				if barWidth < 10 {
					barWidth = 10
				}
				clickX := msg.X - 6
				if clickX < 0 {
					clickX = 0
				}
				if clickX > barWidth {
					clickX = barWidth
				}
				pct := float64(clickX) / float64(barWidth)
				target := int(pct * float64(total))
				if target >= total {
					target = total - 1
				}
				if target < 0 {
					target = 0
				}
				m.selectedItem = target
				m.ensureVisible()
			}
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.isLoading {
			return m, nil
		}
		switch m.screen {
		case "main":
			return m.updateMain(msg)
		case "connectServer":
			return m.updateConnectServer(msg)
		case "addConfig":
			return m.updateAddConfig(msg)
		case "pasteConfig":
			return m.updatePasteConfig(msg)
		case "manualSocks":
			return m.updateManualSocks(msg)
		case "jsonFile":
			return m.updateJSONFile(msg)
	case "configDetails":
			return m.updateConfigDetails(msg)
		case "subscriptions":
			return m.updateSubscriptions(msg)
		case "settings":
			return m.updateSettings(msg)
		case "confirmDeleteSelected":
			return m.updateConfirmDelete(msg, false)
		case "confirmDeleteAll":
			return m.updateConfirmDelete(msg, true)
		case "ipChanger":
			return m.updateIPChanger(msg)
		case "xrayInfo":
			return m.updateXrayInfo(msg)
		}
	}
	return m, nil
}

func (m model) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, m.startLoading("Closing...", 350*time.Millisecond, nil, true)
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 6 {
			m.cursor++
		}
	case "enter", " ":
		switch m.cursor {
		case 0:
			m.screen = "connectServer"
			m.cursor = 0
			m.selectedConfigs = map[int]bool{}
			m.configs = loadConfigs()
			m.selectedItem = len(m.configs) - 1
			if m.selectedItem < 0 {
				m.selectedItem = 0
			}
		case 1:
			m.screen = "addConfig"
			m.cursor = 0
		case 2:
			m.screen = "subscriptions"
			m.cursor = 0
			m.subscriptionURL.Reset()
			m.subscriptionURL.Focus()
			m.analysis = ""
		case 3:
			m.screen = "settings"
			m.cursor = 0
			m.analysis = ""
		case 4:
			m.screen = "ipChanger"
			m.cursor = 0
		case 5:
			m.screen = "xrayInfo"
			m.cursor = 0
		case 6:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) updateAddConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = "main"
		m.cursor = 1
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < 3 {
			m.cursor++
		}
	case "enter", " ":
		switch m.cursor {
		case 0:
			m.screen = "pasteConfig"
			m.pasteArea.SetValue("")
			m.pasteArea.Focus()
			m.analysis = ""
		case 1:
			m.screen = "manualSocks"
			m.socksStep = 0
			m.socksIP.Reset()
			m.socksIP.Focus()
			m.socksPort.Blur()
			m.socksUsername.Blur()
			m.socksPassword.Blur()
		case 2:
			m.screen = "jsonFile"
			m.jsonPathInput.Reset()
			m.jsonPathInput.Focus()
			m.analysis = ""
			m.configBuffer = ""
			m.currentName = ""
			m.resolvedJSONPath = ""
		case 3:
			m.screen = "main"
			m.cursor = 1
		}
	}
	return m, nil
}

func (m model) updatePasteConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.screen = "addConfig"
		m.cursor = 0
		return m, nil
	case "enter":
		// If analysis is done, save and go back to main menu
		if m.analysis != "" && !strings.Contains(m.analysis, "❌") {
			return m, m.startLoading("Saving config...", 450*time.Millisecond, func(next model) model {
				trimmed := strings.TrimSpace(next.pasteArea.Value())
				lines := splitConfigLines(trimmed)
				if len(lines) > 1 {
					saved := 0
					for _, line := range lines {
						if next.saveConfigLine(line) {
							saved++
						}
					}
					next.analysis = successStyle.Render(fmt.Sprintf("✅ Saved %d configurations", saved))
				} else {
					next.saveConfig()()
				}
				next.screen = "main"
				next.cursor = 0
				next.pasteArea.SetValue("")
				if len(lines) <= 1 {
					next.analysis = ""
				}
				next.currentName = ""
				next.configs = loadConfigs()
				return next
			}, false)
		}
		// Otherwise, analyze the config
		if strings.TrimSpace(m.pasteArea.Value()) != "" && m.analysis == "" {
			return m, m.startLoading("Analyzing config...", 450*time.Millisecond, func(next model) model {
				next.analysis = next.parseInput(next.pasteArea.Value())
				if strings.Contains(next.analysis, "❌") {
					next.currentName = ""
				}
				return next
			}, false)
		}
	case "ctrl+s":
		if m.analysis != "" {
			return m, m.startLoading("Saving config...", 450*time.Millisecond, func(next model) model {
				next.saveConfig()()
				next.configs = loadConfigs()
				return next
			}, false)
		}
	case "ctrl+enter", "ctrl+j", "alt+enter":
		// Use Ctrl+Enter instead of Enter for analysis (Enter adds new lines)
		if strings.TrimSpace(m.pasteArea.Value()) != "" {
			return m, m.startLoading("Analyzing config...", 450*time.Millisecond, func(next model) model {
				next.analysis = next.parseInput(next.pasteArea.Value())
				return next
			}, false)
		}
		return m, nil
	case "ctrl+v", "shift+insert":
		// Explicit clipboard paste support
		clipboardText, err := clipboard.ReadAll()
		if err == nil && clipboardText != "" {
			// Insert clipboard content at current cursor position
			m.pasteArea.InsertString(clipboardText)
		}
		return m, nil
	case "ctrl+a":
		// Select all text
		m.pasteArea.SetValue(m.pasteArea.Value())
		return m, nil
	case "ctrl+l":
		// Clear the textarea
		m.pasteArea.SetValue("")
		m.analysis = ""
		return m, nil
	}

	// Update textarea for all other input
	m.pasteArea, cmd = m.pasteArea.Update(msg)
	return m, cmd
}

func (m model) updateManualSocks(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		if m.socksStep > 0 {
			m.socksStep--
			m = m.focusSocksInput()
		} else {
			m.screen = "addConfig"
			m.cursor = 1
		}
		return m, nil
	case "enter":
		if m.socksStep < 3 {
			m.socksStep++
			m = m.focusSocksInput()
		} else if m.socksStep == 3 {
			// Generate and save SOCKS config
			return m, m.startLoading("Saving SOCKS config...", 500*time.Millisecond, func(next model) model {
				socksConfig := next.generateSocksConfig()
				if !strings.Contains(socksConfig, "❌") {
					// Set up config buffer and name for saving
					next.configBuffer = socksConfig
					next.currentName = fmt.Sprintf("SOCKS_%s_%s", next.socksIP.Value(), next.socksPort.Value())
					// Save the config automatically
					next.saveConfig()()
					next.analysis = successStyle.Render("✅ SOCKS config saved successfully!")
					next.screen = "main"
					next.cursor = 0
					next.socksStep = 0
					next.socksIP.Reset()
					next.socksPort.Reset()
					next.socksUsername.Reset()
					next.socksPassword.Reset()
					next.configBuffer = ""
					next.currentName = ""
					next.configs = loadConfigs()
				} else {
					next.analysis = socksConfig
					next.socksStep = 4 // Move to error state
				}
				return next
			}, false)
		}
		return m, nil
	case "ctrl+s":
		if m.analysis != "" {
			return m, m.saveConfig()
		}
	case "ctrl+v", "shift+insert":
		// Add clipboard paste support to SOCKS inputs too
		clipboardText, err := clipboard.ReadAll()
		if err == nil && clipboardText != "" {
			switch m.socksStep {
			case 0:
				m.socksIP.SetValue(clipboardText)
			case 1:
				m.socksPort.SetValue(clipboardText)
			case 2:
				m.socksUsername.SetValue(clipboardText)
			case 3:
				m.socksPassword.SetValue(clipboardText)
			}
		}
		return m, nil
	}

	// Update active input
	switch m.socksStep {
	case 0:
		m.socksIP, cmd = m.socksIP.Update(msg)
	case 1:
		m.socksPort, cmd = m.socksPort.Update(msg)
	case 2:
		m.socksUsername, cmd = m.socksUsername.Update(msg)
	case 3:
		m.socksPassword, cmd = m.socksPassword.Update(msg)
	}

	return m, cmd
}

func (m model) updateJSONFile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		m.screen = "addConfig"
		m.cursor = 2
		return m, nil
	case "enter":
		if m.analysis != "" && !strings.Contains(m.analysis, "❌") {
			return m, m.startLoading("Saving config...", 450*time.Millisecond, func(next model) model {
				next.saveConfig()()
				next.screen = "main"
				next.cursor = 0
				next.jsonPathInput.SetValue("")
				next.analysis = ""
				next.currentName = ""
				next.configBuffer = ""
				next.resolvedJSONPath = ""
				next.configs = loadConfigs()
				return next
			}, false)
		}
		if strings.TrimSpace(m.jsonPathInput.Value()) != "" && m.analysis == "" {
			return m, m.startLoading("Loading file...", 450*time.Millisecond, func(next model) model {
				next.analysis = next.loadJSONFromPath(next.jsonPathInput.Value())
				if strings.Contains(next.analysis, "❌") {
					next.currentName = ""
					next.resolvedJSONPath = ""
					next.configBuffer = ""
				}
				return next
			}, false)
		}
	case "ctrl+s":
		if m.analysis != "" {
			return m, m.startLoading("Saving config...", 450*time.Millisecond, func(next model) model {
				next.saveConfig()()
				next.configs = loadConfigs()
				return next
			}, false)
		}
	case "ctrl+v", "shift+insert":
		clipboardText, err := clipboard.ReadAll()
		if err == nil && clipboardText != "" {
			m.jsonPathInput.SetValue(clipboardText)
		}
		return m, nil
	case "ctrl+l":
		m.jsonPathInput.SetValue("")
		m.analysis = ""
		m.configBuffer = ""
		m.currentName = ""
		m.resolvedJSONPath = ""
		return m, nil
	}

	m.jsonPathInput, cmd = m.jsonPathInput.Update(msg)
	return m, cmd
}

func (m model) focusSocksInput() model {
	m.socksIP.Blur()
	m.socksPort.Blur()
	m.socksUsername.Blur()
	m.socksPassword.Blur()

	switch m.socksStep {
	case 0:
		m.socksIP.Focus()
	case 1:
		m.socksPort.Focus()
	case 2:
		m.socksUsername.Focus()
	case 3:
		m.socksPassword.Focus()
	}

	return m
}

func (m model) updateConnectServer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If search is active, redirect all input to search field
	if m.showSearch {
		var cmd tea.Cmd
		switch msg.String() {
		case "enter":
			m.showSearch = false
			m.searchInput.Blur()
			return m, nil
		case "esc":
			m.showSearch = false
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.searchInput.Value()) != "" {
			m.searchInput.SetValue("")
			m.selectedItem = 0
			m.scrollOffset = 0
			m.selectedConfigs = map[int]bool{}
			m.ensureVisible()
			return m, nil
		}
		m.screen = "main"
		m.cursor = 0
	case "/":
		m.showSearch = true
		m.searchInput.Focus()
		m.searchInput.SetValue("")
		return m, nil
	case "up", "k":
		if m.selectedItem > 0 {
			m.selectedItem--
			m.ensureVisible()
		}
	case "down", "j":
		flen := len(m.filteredConfigs())
		if m.selectedItem < flen-1 {
			m.selectedItem++
			m.ensureVisible()
		}
	case "shift+up":
		if m.selectedItem > 0 {
			if len(m.selectedConfigs) == 0 {
				m.selectedConfigs[m.selectedItem] = true
			}
			m.selectedItem--
			m.selectedConfigs[m.selectedItem] = true
			m.ensureVisible()
		}
	case "shift+down":
		flen := len(m.filteredConfigs())
		if m.selectedItem < flen-1 {
			if len(m.selectedConfigs) == 0 {
				m.selectedConfigs[m.selectedItem] = true
			}
			m.selectedItem++
			m.selectedConfigs[m.selectedItem] = true
			m.ensureVisible()
		}
	case "ctrl+a":
		flen := len(m.filteredConfigs())
		for i := 0; i < flen; i++ {
			m.selectedConfigs[i] = true
		}
	case "enter":
		filtered := m.filteredConfigs()
		if len(filtered) > 0 && m.selectedItem < len(filtered) {
			selectedConfig := filtered[m.selectedItem]
			setActiveConfig(selectedConfig.Path)
			m.configs = loadConfigs()
			m.analysis = infoStyle.Render("🔄 Attempting to connect to " + selectedConfig.Name + "...")
			return m, m.connectToServer(selectedConfig)
		} else {
			m.analysis = errorStyle.Render("❌ No configs available or invalid selection")
		}
	case " ":
		flen := len(m.filteredConfigs())
		if m.selectedItem >= 0 && m.selectedItem < flen {
			m.selectedConfigs[m.selectedItem] = !m.selectedConfigs[m.selectedItem]
		}
		return m, nil
	case "i":
		filtered := m.filteredConfigs()
		if len(filtered) > 0 && m.selectedItem >= 0 && m.selectedItem < len(filtered) {
			cfg := filtered[m.selectedItem]
			return m, m.startLoading("Loading details...", 300*time.Millisecond, func(next model) model {
				next.configDetail = loadConfigDetails(cfg.Path)
				next.screen = "configDetails"
				return next
			}, false)
		}
	case "s":
		filtered := m.filteredConfigs()
		if len(filtered) > 0 && m.selectedItem >= 0 && m.selectedItem < len(filtered) {
			cfg := filtered[m.selectedItem]
			return m, m.startLoading("Setting active...", 400*time.Millisecond, func(next model) model {
				err := setActiveConfig(cfg.Path)
				if err != nil {
					next.analysis = errorStyle.Render(fmt.Sprintf("❌ %v", err))
				} else {
					next.analysis = successStyle.Render("✅ Active config set")
					next.configs = loadConfigs()
				}
				return next
			}, false)
		}
	case "d":
		filtered := m.filteredConfigs()
		if len(filtered) > 0 {
			if len(m.selectedConfigs) == 0 {
				m.selectedConfigs[m.selectedItem] = true
			}
			m.screen = "confirmDeleteSelected"
			return m, nil
		}
	case "a":
		if len(m.configs) > 0 {
			m.screen = "confirmDeleteAll"
			return m, nil
		}
	case "D":
		if len(m.configs) > 0 {
			deleted := deleteDuplicateConfigs(m.configs)
			m.configs = loadConfigs()
			m.selectedConfigs = map[int]bool{}
			m.selectedItem = maxInt(0, len(m.configs)-1)
			m.analysis = successStyle.Render(fmt.Sprintf("✅ Deleted %d duplicate configs", deleted))
			return m, nil
		}
	case "E":
		if len(m.configs) > 0 {
			deleted := deleteInvalidConfigs(m.configs)
			m.configs = loadConfigs()
			m.selectedConfigs = map[int]bool{}
			m.selectedItem = maxInt(0, len(m.configs)-1)
			m.analysis = successStyle.Render(fmt.Sprintf("✅ Deleted %d invalid configs", deleted))
			return m, nil
		}
	case "p":
		filtered := m.filteredConfigs()
		if !m.pinging && m.selectedItem >= 0 && m.selectedItem < len(filtered) {
			if len(m.selectedConfigs) > 0 {
				indices := make([]int, 0, len(m.selectedConfigs))
				for idx := range m.selectedConfigs {
					if idx >= 0 && idx < len(filtered) {
						indices = append(indices, idx)
					}
				}
				cfgs := make([]ConfigInfo, len(indices))
				for i, idx := range indices {
					cfgs[i] = filtered[idx]
				}
				return m, func() tea.Msg {
					return latencyStarted{total: len(cfgs), configs: cfgs, indices: nil, timeout: time.Duration(m.settingsPingTimeout) * time.Second}
				}
			}
			idx := m.selectedItem
			cfg := filtered[idx]
			m.analysis = fmt.Sprintf("Pinging %s:%d...", cfg.Server, cfg.Port)
			return m, func() tea.Msg {
				binary, err := findXrayBinary()
				if err != nil {
					return latencyResult{server: cfg.Server, port: cfg.Port, ping: "ERR"}
				}
				latency, err := runXrayLatency(binary, cfg.Path, time.Duration(m.settingsPingTimeout)*time.Second, cfg.Server, cfg.Port)
				ping := "ERR"
				if err == nil {
					ping = latency
				}
				return latencyResult{server: cfg.Server, port: cfg.Port, ping: ping}
			}
		}
	case "c":
		filtered := m.filteredConfigs()
		if len(filtered) > 0 {
			if len(m.selectedConfigs) > 0 {
				var lines []string
				for idx := range m.selectedConfigs {
					if idx >= 0 && idx < len(filtered) {
						lines = append(lines, readConfigForCopy(filtered[idx].Path))
					}
				}
				if len(lines) > 0 {
					text := strings.Join(lines, "\n")
					return m, func() tea.Msg {
						clipboard.WriteAll(text)
						return nil
					}
				}
			}
			if m.selectedItem >= 0 && m.selectedItem < len(filtered) {
				cfg := filtered[m.selectedItem]
				text := readConfigForCopy(cfg.Path)
				if text != "" {
					return m, func() tea.Msg {
						clipboard.WriteAll(text)
						return nil
					}
				}
			}
		}
	case "t":
		if !isXrayRunning() {
			m.analysis = warningStyle.Render("⚠️ Xray is not running. Connect to a config first.")
			return m, nil
		}
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
		cmd := exec.Command(shell)
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("HTTP_PROXY=http://127.0.0.1:%d", m.settingsHTTPPort),
			fmt.Sprintf("HTTPS_PROXY=http://127.0.0.1:%d", m.settingsHTTPPort),
			fmt.Sprintf("http_proxy=http://127.0.0.1:%d", m.settingsHTTPPort),
			fmt.Sprintf("https_proxy=http://127.0.0.1:%d", m.settingsHTTPPort),
			fmt.Sprintf("ALL_PROXY=http://127.0.0.1:%d", m.settingsHTTPPort),
			fmt.Sprintf("all_proxy=http://127.0.0.1:%d", m.settingsHTTPPort),
		)
		filtered := m.filteredConfigs()
		configName := ""
		if m.selectedItem >= 0 && m.selectedItem < len(filtered) {
			configName = filtered[m.selectedItem].Name
		}
		return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
			return shellSessionEnded{config: configName}
		})
	case "[":
		maxLines := configListMaxLines(m.termHeight)
		m.selectedItem -= maxLines / 2
		if m.selectedItem < 0 {
			m.selectedItem = 0
		}
		m.ensureVisible()
	case "]":
		flen := len(m.filteredConfigs())
		maxLines := configListMaxLines(m.termHeight)
		m.selectedItem += maxLines / 2
		if m.selectedItem >= flen {
			m.selectedItem = flen - 1
		}
		m.ensureVisible()
	case "f":
		m.showFilter = !m.showFilter
	case "1":
		if m.showFilter {
			m.filter = "all"
			m.showFilter = false
			m.selectedItem = 0
			m.scrollOffset = 0
			m.selectedConfigs = map[int]bool{}
			m.ensureVisible()
		}
	case "2":
		if m.showFilter {
			m.filter = "working"
			m.showFilter = false
			m.selectedItem = 0
			m.scrollOffset = 0
			m.selectedConfigs = map[int]bool{}
			m.ensureVisible()
		}
	case "3":
		if m.showFilter {
			m.filter = "errored"
			m.showFilter = false
			m.selectedItem = 0
			m.scrollOffset = 0
			m.selectedConfigs = map[int]bool{}
			m.ensureVisible()
		}
	case "4":
		if m.showFilter {
			m.filter = "unpinged"
			m.showFilter = false
			m.selectedItem = 0
			m.scrollOffset = 0
			m.selectedConfigs = map[int]bool{}
			m.ensureVisible()
		}
	case "q":
		if m.showFilter {
			m.showFilter = false
		}
	}
	return m, nil
}


func (m model) updateIPChanger(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = "main"
		m.cursor = 5
	case "enter", " ":
		// Exit TUI and launch IP Changer
		return m, tea.Sequence(
			tea.ExitAltScreen,
			func() tea.Msg {
				// Check if running as root
				if os.Geteuid() != 0 {
					runIPChangerWithSudo()
				} else {
					runIPChangerAsRoot()
				}
				return tea.KeyMsg{Type: tea.KeyEnter}
			},
		)
	}
	return m, nil
}

func (m model) updateXrayInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = "main"
		m.cursor = 6
	case "r":
		return m, m.startLoading("Refreshing...", 350*time.Millisecond, func(next model) model {
			next = populateXrayInfo(next)
			return next
		}, false)
	}
	return m, nil
}

func (m model) updateSubscriptions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.screen = "main"
		m.cursor = 3
		m.subscriptionURL.Blur()
		m.showSubsList = false
		return m, nil
	case "tab":
		m.showSubsList = !m.showSubsList
		if m.showSubsList {
			m.subscriptionURL.Blur()
		} else {
			m.subscriptionURL.Focus()
		}
		return m, nil
	case "up":
		if m.showSubsList && len(m.savedSubs) > 0 {
			if m.selectedSubIndex > 0 {
				m.selectedSubIndex--
			}
		}
		return m, nil
	case "down":
		if m.showSubsList && len(m.savedSubs) > 0 {
			if m.selectedSubIndex < len(m.savedSubs)-1 {
				m.selectedSubIndex++
			}
		}
		return m, nil
	case "enter":
		if m.showSubsList && len(m.savedSubs) > 0 {
			selectedSub := m.savedSubs[m.selectedSubIndex]
			return m, m.startLoading("Loading subscription...", 600*time.Millisecond, func(next model) model {
				saved, err := importSubscription(selectedSub.URL)
				if err != nil {
					next.analysis = errorStyle.Render(fmt.Sprintf("❌ %v", err))
				} else {
					next.analysis = successStyle.Render(fmt.Sprintf("✅ Loaded %d configs from %s", saved, selectedSub.Name))
					next.configs = loadConfigs()
				}
				return next
			}, false)
		} else if strings.TrimSpace(m.subscriptionURL.Value()) != "" {
			// Save the subscription first
			url := strings.TrimSpace(m.subscriptionURL.Value())
			subName := fmt.Sprintf("Sub_%s", time.Now().Format("0102_1504"))
			newSub := SubscriptionInfo{
				Name: subName,
				URL:  url,
				Date: time.Now().Format("2006-01-02 15:04"),
			}
			m.savedSubs = append(m.savedSubs, newSub)
			saveSubscriptions(m.savedSubs)
			
			return m, m.startLoading("Importing...", 600*time.Millisecond, func(next model) model {
				saved, err := importSubscription(url)
				if err != nil {
					next.analysis = errorStyle.Render(fmt.Sprintf("❌ %v", err))
				} else {
					next.analysis = successStyle.Render(fmt.Sprintf("✅ Imported %d configs and saved subscription", saved))
					next.configs = loadConfigs()
				}
				return next
			}, false)
		}
	case "d":
		if m.showSubsList && len(m.savedSubs) > 0 {
			// Delete selected subscription
			m.savedSubs = append(m.savedSubs[:m.selectedSubIndex], m.savedSubs[m.selectedSubIndex+1:]...)
			if m.selectedSubIndex >= len(m.savedSubs) && len(m.savedSubs) > 0 {
				m.selectedSubIndex = len(m.savedSubs) - 1
			}
			saveSubscriptions(m.savedSubs)
			m.analysis = successStyle.Render("✅ Subscription deleted")
		}
		return m, nil
	case "ctrl+v", "shift+insert":
		if !m.showSubsList {
			clipboardText, err := clipboard.ReadAll()
			if err == nil && clipboardText != "" {
				m.subscriptionURL.SetValue(clipboardText)
			}
		}
		return m, nil
	case "ctrl+l":
		m.subscriptionURL.SetValue("")
		m.analysis = ""
		return m, nil
	}

	if !m.showSubsList {
		m.subscriptionURL, cmd = m.subscriptionURL.Update(msg)
	}
	return m, cmd
}

func (m model) updateConfirmDelete(msg tea.KeyMsg, deleteAll bool) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		return m, m.startLoading("Deleting...", 400*time.Millisecond, func(next model) model {
			deleted := 0
			if deleteAll {
				deleted = deleteAllConfigs(next)
			} else {
				deleted = deleteSelectedConfigs(next)
			}
			next.configs = loadConfigs()
			next.selectedConfigs = map[int]bool{}
			next.selectedItem = maxInt(0, len(next.configs)-1)
			next.analysis = successStyle.Render(fmt.Sprintf("✅ Deleted %d configs", deleted))
		next.screen = "connectServer"
		return next
	}, false)
case "n", "esc":
	m.screen = "connectServer"
		return m, nil
	}
	return m, nil
}

func (m model) updateConfigDetails(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = "connectServer"
		return m, nil
	case "c":
		if m.configDetail != "" {
			return m, func() tea.Msg {
				clipboard.WriteAll(m.configDetail)
				return nil
			}
		}
	case "e":
		if m.configDetail != "" {
			link := exportConfigLink(m.configDetail)
			m.configDetailNote = link
			return m, func() tea.Msg {
				clipboard.WriteAll(link)
				return nil
			}
		}
	}
	return m, nil
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		if m.settingsEditing {
			m.settingsEditing = false
			m.settingsInput.Blur()
		} else {
			m.screen = "main"
			m.cursor = 4
		}
		return m, nil
	case "up", "k":
		if !m.settingsEditing && m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if !m.settingsEditing && m.settingsCursor < 2 {
			m.settingsCursor++
		}
	case "enter":
		if !m.settingsEditing {
			m.settingsEditing = true
			switch m.settingsCursor {
			case 0:
				m.settingsInput.SetValue(fmt.Sprintf("%d", m.settingsHTTPPort))
			case 1:
				m.settingsInput.SetValue(fmt.Sprintf("%d", m.settingsSOCKSPort))
			case 2:
				m.settingsInput.SetValue(fmt.Sprintf("%d", m.settingsPingTimeout))
			}
			m.settingsInput.Focus()
		} else {
			val := strings.TrimSpace(m.settingsInput.Value())
			num, err := strconv.Atoi(val)
			if err == nil && num > 0 && num <= 65535 {
				switch m.settingsCursor {
				case 0:
					m.settingsHTTPPort = num
				case 1:
					m.settingsSOCKSPort = num
				case 2:
					if num > 0 && num <= 120 {
						m.settingsPingTimeout = num
					}
				}
			}
			saveSettings(m.settingsHTTPPort, m.settingsSOCKSPort, m.settingsPingTimeout)
			m.settingsEditing = false
			m.settingsInput.Blur()
			m.settingsInput.SetValue("")
		}
		return m, nil
	}

	if m.settingsEditing {
		m.settingsInput, cmd = m.settingsInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) viewSettings() string {
	width := responsiveWidth(m.termWidth, 80)

	s := "Settings\n\n"

	type settingItem struct {
		name  string
		value string
		desc  string
	}
	settings := []settingItem{
		{name: "HTTP Port", value: fmt.Sprintf("%d", m.settingsHTTPPort), desc: "Local HTTP inbound port when connecting"},
		{name: "SOCKS Port", value: fmt.Sprintf("%d", m.settingsSOCKSPort), desc: "Local SOCKS inbound port when connecting"},
		{name: "Ping Timeout", value: fmt.Sprintf("%ds", m.settingsPingTimeout), desc: "Timeout per ping request"},
	}

	maxName := 0
	for _, set := range settings {
		if len(set.name) > maxName {
			maxName = len(set.name)
		}
	}

	for i, set := range settings {
		cursor := "  "
		if m.settingsCursor == i {
			cursor = accentStyle.Render("> ")
		}
		label := fmt.Sprintf("%-*s", maxName, set.name)
		if m.settingsEditing && m.settingsCursor == i {
			line := fmt.Sprintf("%s%s : %s", cursor, label, inputStyle.Render(m.settingsInput.View()))
			s += line + "\n"
		} else {
			line := fmt.Sprintf("%s%s : %s", cursor, label, set.value)
			s += line + "\n"
		}
		if m.settingsCursor == i {
			s += dimStyle.Render("   "+set.desc) + "\n"
		}
	}

	if m.analysis != "" {
		s += "\n" + m.analysis + "\n"
	}

	s += "\n" + dimStyle.Render("↑/↓: Navigate | Enter: Edit | Esc: Back")

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) View() string {
	var s strings.Builder
	compact := m.termHeight > 0 && m.termHeight < 25

	if !compact {
		s.WriteString("\n")
		// ASCII Art Header
		s.WriteString(titleStyle.Render(xrayArt))
		s.WriteString("\n")
	}

	// Status Bar
	s.WriteString(m.renderStatusBar())
	s.WriteString("\n")

	if m.isLoading {
		return m.renderLoading()
	}

	switch m.screen {
	case "main":
		s.WriteString(m.viewMain())
	case "connectServer":
		s.WriteString(m.viewConnectServer())
	case "addConfig":
		s.WriteString(m.viewAddConfig())
	case "pasteConfig":
		s.WriteString(m.viewPasteConfig())
	case "manualSocks":
		s.WriteString(m.viewManualSocks())
	case "jsonFile":
		s.WriteString(m.viewJSONFile())
	case "ipChanger":
		s.WriteString(m.viewIPChanger())
	case "xrayInfo":
		s.WriteString(m.viewXrayInfo())
	case "configDetails":
		s.WriteString(m.viewConfigDetails())
	case "subscriptions":
		s.WriteString(m.viewSubscriptions())
	case "settings":
		s.WriteString(m.viewSettings())
	case "confirmDeleteSelected":
		s.WriteString(m.viewDeleteConfirm(false))
	case "confirmDeleteAll":
		s.WriteString(m.viewDeleteConfirm(true))
	}

	return s.String()
}

func (m model) renderStatusBar() string {
	compact := m.termHeight > 0 && m.termHeight < 25

	coreStatus := "STOPPED"
	coreColor := catppuccinRed
	if isXrayRunning() {
		coreStatus = "RUNNING"
		coreColor = catppuccinGreen
	}

	activeConfig := "None"
	activeColor := catppuccinInactive
	for _, config := range m.configs {
		if config.Active {
			activeConfig = config.Name
			activeColor = catppuccinYellow
			break
		}
	}

	statusLine := fmt.Sprintf("%s %s | %s %s",
		accentStyle.Render("Core:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color(coreColor)).Render(coreStatus),
		accentStyle.Render("Active:"),
		lipgloss.NewStyle().Foreground(lipgloss.Color(activeColor)).Render(activeConfig))

	if !compact {
		timestamp := time.Now().Format("15:04:05")
		statusLine += " | " + infoStyle.Render(timestamp)
	}

	width := responsiveWidth(m.termWidth, 80)
	if compact {
		return lipgloss.NewStyle().
			Padding(0, 1).
			Width(width).
			Render(statusLine)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(0, 1).
		Width(width).
		Render(statusLine)
}

func (m model) renderLoading() string {
	line := fmt.Sprintf("%s %s", m.spinner.View(), dimStyle.Render(m.loadingText))
	block := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 3).
		Render(line)

	width := m.termWidth
	height := m.termHeight
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

func renderPing(ping string) string {
	trimmed := strings.TrimSpace(ping)
	if trimmed == "" || trimmed == "N/A" {
		return dimStyle.Render("N/A")
	}
	if strings.Contains(strings.ToLower(trimmed), "err") || strings.Contains(strings.ToLower(trimmed), "not found") {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinPink)).Bold(true).Render(trimmed)
	}

	value := parsePingMs(trimmed)
	switch {
	case value == 0:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinPink)).Bold(true).Render(trimmed)
	case value >= 1 && value <= 100:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true).Render(trimmed)
	case value > 100 && value <= 200:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#90EE90")).Bold(true).Render(trimmed)
	case value > 200 && value <= 350:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")).Bold(true).Render(trimmed)
	case value > 350 && value <= 500:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6347")).Bold(true).Render(trimmed)
	case value > 500:
		return errorStyle.Render(trimmed)
	default:
		return infoStyle.Render(trimmed)
	}
}

func (m model) renderXrayInfo() string {
	status := dimStyle.Render("stopped")
	if isXrayRunning() {
		status = successStyle.Render("running")
	}

	path := dimStyle.Render("not found")
	if m.xrayBinaryPath != "" {
		path = infoStyle.Render(m.xrayBinaryPath)
	} else if m.xrayBinaryError != "" {
		path = errorStyle.Render(m.xrayBinaryError)
	}

	version := dimStyle.Render("unknown")
	if m.xrayVersion != "" {
		version = infoStyle.Render(m.xrayVersion)
	}

	infoWidth := responsiveWidth(m.termWidth, 80)
	infoBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(infoWidth).
		Render(
			fmt.Sprintf("Xray core: %s\nPath: %s\nVersion: %s", status, path, version),
		)

	return infoBox
}

func responsiveWidth(termWidth int, fallback int) int {
	if termWidth <= 0 {
		return fallback
	}
	width := termWidth - 6
	if width < 40 {
		return 40
	}
	return width
}

func clamp(min, value, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m model) applyResponsiveLayout() model {
	width := responsiveWidth(m.termWidth, 80)
	// Menu box: Border + Padding(1,2) = 4 chars overhead
	// inputStyle wrapper: Border + Padding(1) = 4 chars overhead
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}

	m.jsonPathInput.Width = contentWidth - 4
	if m.jsonPathInput.Width < 20 {
		m.jsonPathInput.Width = 20
	}
	m.socksIP.Width = clamp(12, contentWidth/3, 30)
	m.socksPort.Width = clamp(6, contentWidth/6, 12)
	m.socksUsername.Width = clamp(16, contentWidth/2, 40)
	m.socksPassword.Width = clamp(16, contentWidth/2, 40)
	m.pasteArea.SetWidth(contentWidth)
	m.pasteArea.SetHeight(clamp(3, m.termHeight/4, 12))
	return m
}

func (m model) filteredConfigs() []ConfigInfo {
	var result []ConfigInfo
	for _, cfg := range m.configs {
		switch m.filter {
		case "working":
			if cfg.Ping != "" && cfg.Ping != "ERR" {
				result = append(result, cfg)
			}
		case "errored":
			if cfg.Ping == "ERR" {
				result = append(result, cfg)
			}
		case "unpinged":
			if cfg.Ping == "" {
				result = append(result, cfg)
			}
		default:
			result = append(result, cfg)
		}
	}
	if m.filter == "working" {
		sort.SliceStable(result, func(i, j int) bool {
			return parsePingMs(result[i].Ping) > parsePingMs(result[j].Ping)
		})
	} else {
		sort.SliceStable(result, func(i, j int) bool {
			return parsePingMs(result[i].Ping) < parsePingMs(result[j].Ping)
		})
	}
	// Apply text search filter
	if q := strings.TrimSpace(m.searchInput.Value()); q != "" {
		lowerQ := strings.ToLower(q)
		var searched []ConfigInfo
		for _, cfg := range result {
			if strings.Contains(strings.ToLower(cfg.Name), lowerQ) ||
				strings.Contains(strings.ToLower(cfg.Server), lowerQ) ||
				strings.Contains(strings.ToLower(cfg.Protocol), lowerQ) {
				searched = append(searched, cfg)
			}
		}
		return searched
	}
	return result
}

func configListMaxLines(termHeight int) int {
	compact := termHeight > 0 && termHeight < 25
	if compact {
		oh := 11 // overhead on compact: status(1) + \n(1) + border(1) + title(2) + search(1) + \n(1) + help(1) + search2(1) + filter(1) + border(1)
		if n := termHeight - oh; n > 3 {
			return n
		}
		return 3
	}
	oh := 18 // overhead on normal: \n(1) + art(6) + \n(1) + status(3) + \n(1) + border(1) + title(2) + search(1) + \n(1) + help(1) + filter(1) + border(1)
	if n := termHeight - oh; n > 3 {
		return n
	}
	return 3
}

func (m *model) ensureVisible() {
	configs := m.filteredConfigs()
	total := len(configs)
	if total == 0 {
		return
	}
	maxLines := configListMaxLines(m.termHeight)
	if maxLines > total {
		maxLines = total
	}
	if m.selectedItem < m.scrollOffset {
		m.scrollOffset = m.selectedItem
	}
	if m.selectedItem >= m.scrollOffset+maxLines {
		m.scrollOffset = m.selectedItem - maxLines + 1
	}
	if m.scrollOffset+maxLines > total {
		m.scrollOffset = total - maxLines
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m model) visibleConfigs() ([]ConfigInfo, int, int) {
	configs := m.filteredConfigs()
	total := len(configs)
	if total == 0 {
		return nil, 0, 0
	}
	maxLines := configListMaxLines(m.termHeight)
	if maxLines > total {
		maxLines = total
	}
	start := m.scrollOffset
	if start+maxLines > total {
		start = total - maxLines
	}
	if start < 0 {
		start = 0
	}
	return configs[start : start+maxLines], start, total
}

func (m model) renderConfigList(showSelection bool) string {
	visible, offset, total := m.visibleConfigs()
	if total == 0 {
		return ""
	}
	var out strings.Builder
	for i, cfg := range visible {
		idx := offset + i
		cursor := "  "
		marker := accentStyle.Render(fmt.Sprintf("%d", idx+1))
		status := dimStyle.Render("●")
		if cfg.Active {
			status = successStyle.Render("●")
		}

		if m.selectedItem == idx {
			cursor = accentStyle.Render("> ")
		}

		sel := ""
		if showSelection {
			s := "[ ]"
			if m.selectedConfigs[idx] {
				s = "[x]"
			}
			sel = s + " "
		}

		name := cfg.Name
		if m.selectedItem == idx {
			name = selectedItemStyle.Render(name)
		}

		ping := renderPing(cfg.Ping)
		out.WriteString(fmt.Sprintf("%s%s %s%s %s (%s:%d) %s\n", cursor, marker, sel, status, name, cfg.Server, cfg.Port, ping))
	}

	// Scroll indicator
	if total > len(visible) {
		pct := float64(offset+len(visible)) / float64(total) * 100
		barLen := 20
		pos := int(float64(barLen) * float64(offset+len(visible)/2) / float64(total))
		if pos < 0 {
			pos = 0
		}
		if pos >= barLen {
			pos = barLen - 1
		}
		bar := strings.Repeat("─", pos) + "●" + strings.Repeat("─", barLen-pos-1)
		selCount := len(m.selectedConfigs)
		selInfo := ""
		if selCount > 0 {
			selInfo = infoStyle.Render(fmt.Sprintf(" [%d selected]", selCount))
		}
		out.WriteString(dimStyle.Render(fmt.Sprintf("\n  %s  %d/%d (%.0f%%)%s", bar, total, total, pct, selInfo)))
	}
	return out.String()
}

func parsePingMs(ping string) int {
	value := strings.ToLower(strings.TrimSpace(ping))
	value = strings.TrimSuffix(value, "ms")
	var ms int
	fmt.Sscanf(value, "%d", &ms)
	return ms
}

func (m model) viewMain() string {
	body := ""

	choices := []string{
		"Connect",
		"Add Config",
		"Subscriptions",
		"Settings",
		"IP Changer",
		"Xray Info",
		"Quit",
	}
	markers := []string{
		accentStyle.Render("1"),
		accentStyle.Render("2"),
		accentStyle.Render("3"),
		accentStyle.Render("4"),
		accentStyle.Render("5"),
		accentStyle.Render("6"),
		accentStyle.Render("7"),
	}

	for i, choice := range choices {
		cursor := "  "
		if m.cursor == i {
			cursor = accentStyle.Render("> ")
			choice = selectedItemStyle.Render(choice)
		} else {
			choice = menuItemStyle.Render(choice)
		}
		body += fmt.Sprintf("%s%s %s\n", cursor, markers[i], choice)
	}

	body += "\n" + dimStyle.Render("↑/↓: Navigate | Enter: Select | q: Quit")

	menuWidth := responsiveWidth(m.termWidth, 80)

	if m.analysis != "" && (m.pinging || strings.Contains(m.analysis, "Ping")) {
		body += "\n\n"
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinBlue)).
			Padding(1, 2).
			Width(clamp(40, menuWidth-6, 120)).
			Render(m.analysis)
		body += resultBox
	}

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(menuWidth).
		Render("Main Menu\n\n" + body)

	return menuBox
}

func (m model) viewAddConfig() string {
	body := ""

	choices := []string{
		"Paste Config (JSON/Proxy URLs)",
		"Manual SOCKS Proxy",
		"Load JSON File",
		"Back to Main Menu",
	}
	markers := []string{
		accentStyle.Render("1"),
		accentStyle.Render("2"),
		accentStyle.Render("3"),
		accentStyle.Render("4"),
	}

	for i, choice := range choices {
		cursor := "  "
		if m.cursor == i {
			cursor = accentStyle.Render("> ")
			choice = selectedItemStyle.Render(choice)
		} else {
			choice = menuItemStyle.Render(choice)
		}
		body += fmt.Sprintf("%s%s %s\n", cursor, markers[i], choice)
	}

	body += "\n" + dimStyle.Render("↑/↓: Navigate | Enter: Select | Esc: Back")

	menuWidth := responsiveWidth(m.termWidth, 80)
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(menuWidth).
		Render("Add New Configuration\n\n" + body)

	return menuBox
}

func (m model) viewPasteConfig() string {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}

	m.pasteArea.SetWidth(contentWidth)
	m.pasteArea.SetHeight(clamp(3, m.termHeight/4, 12))

	s := "Paste Configuration\n\n"
	s += "Paste your Xray JSON config or proxy URL:\n\n"

	s += inputStyle.Render(m.pasteArea.View()) + "\n\n"

	if m.analysis != "" {
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinBlue)).
			Padding(1, 2).
			Width(contentWidth).
			Render(m.analysis)

		s += successStyle.Render("🔍 Analysis Result:") + "\n"
		s += resultBox + "\n\n"

		// Show config name if available
		if m.currentName != "" {
			s += fmt.Sprintf("📝 Config Name: %s\n", successStyle.Render(m.currentName))
		}
		s += fmt.Sprintf("📂 Save to: %s\n\n", m.savePath)
	}

	if m.analysis != "" && !strings.Contains(m.analysis, "❌") {
		s += successStyle.Render("✅ Press Enter to save and return to Main Menu\n\n")
	}

	s += accentStyle.Render("📋 Ctrl+V/Shift+Insert:") + " Paste from clipboard\n"
	if m.analysis == "" {
		s += accentStyle.Render("⚡ Enter:") + " Analyze config\n"
	}
	s += accentStyle.Render("🗑️  Ctrl+L:") + " Clear\n"
	s += accentStyle.Render("💾 Ctrl+S:") + " Save config\n"
	s += dimStyle.Render("🔙 Esc: Go back")
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) viewManualSocks() string {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}

	m.socksIP.Width = clamp(12, contentWidth/3, 30)
	m.socksPort.Width = clamp(6, contentWidth/6, 12)
	m.socksUsername.Width = clamp(16, contentWidth/2, 40)
	m.socksPassword.Width = clamp(16, contentWidth/2, 40)

	s := "Manual SOCKS Proxy Setup\n\n"

	steps := []string{"Server IP", "Port", "Username", "Password"}

	for i, step := range steps {
		status := "⭕"
		if i < m.socksStep {
			status = "✅"
		} else if i == m.socksStep {
			status = "🔄"
		}
		s += fmt.Sprintf("%s %s\n", status, step)
	}
	s += "\n"

	if m.socksStep < 4 {
		switch m.socksStep {
		case 0:
			s += "Enter Server IP:\n" + inputStyle.Render(m.socksIP.View())
		case 1:
			s += "Enter Port (1-65535):\n" + inputStyle.Render(m.socksPort.View())
		case 2:
			s += "Enter Username (optional, press Enter to skip):\n" + inputStyle.Render(m.socksUsername.View())
		case 3:
			s += "Enter Password (optional, press Enter to skip):\n" + inputStyle.Render(m.socksPassword.View())
		}
	}

	if m.analysis != "" {
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinGreen)).
			Padding(1, 2).
			Width(contentWidth).
			Render(m.analysis)

		s += successStyle.Render("✨ Configuration Ready:") + "\n"
		s += resultBox + "\n\n"
		s += fmt.Sprintf("📂 Save to: %s\n", m.savePath)
		s += "\n" + successStyle.Render("Press Ctrl+S to save this configuration")
	}

	if m.socksStep < 4 {
		s += "\n\n" + accentStyle.Render("📋 Ctrl+V/Shift+Insert:") + " Paste | " +
			accentStyle.Render("Enter:") + " Next step | " +
			dimStyle.Render("Esc: Previous step/Back")
	} else {
		s += "\n\n" + accentStyle.Render("💾 Ctrl+S:") + " Save | " +
			dimStyle.Render("🔙 Esc: Back to menu")
	}
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) viewJSONFile() string {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}
	m.jsonPathInput.Width = contentWidth - 4
	if m.jsonPathInput.Width < 20 {
		m.jsonPathInput.Width = 20
	}

	s := "Load JSON Configuration\n\n"
	s += "Enter the path to your JSON config file:\n"
	s += dimStyle.Render("Supports ~, relative paths, and quoted strings") + "\n\n"
	s += inputStyle.Render(m.jsonPathInput.View()) + "\n\n"

	if m.analysis != "" {
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinBlue)).
			Padding(1, 2).
			Width(contentWidth).
			Render(m.analysis)

		s += successStyle.Render("🔍 Analysis Result:") + "\n"
		s += resultBox + "\n\n"
		if m.currentName != "" {
			s += fmt.Sprintf("📝 Config Name: %s\n", successStyle.Render(m.currentName))
		}
		if m.resolvedJSONPath != "" {
			s += fmt.Sprintf("📁 Resolved Path: %s\n", m.resolvedJSONPath)
		}
		s += fmt.Sprintf("📂 Save to: %s\n\n", m.savePath)
	}

	if m.analysis != "" && !strings.Contains(m.analysis, "❌") {
		s += successStyle.Render("✅ Press Enter to save and return to Main Menu\n\n")
	}

	s += accentStyle.Render("📋 Ctrl+V/Shift+Insert:") + " Paste path\n"
	if m.analysis == "" {
		s += accentStyle.Render("⚡ Enter:") + " Load and validate file\n"
	}
	s += accentStyle.Render("🗑️  Ctrl+L:") + " Clear\n"
	s += accentStyle.Render("💾 Ctrl+S:") + " Save config\n"
	s += dimStyle.Render("🔙 Esc: Go back")
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) renderFilterBar() string {
	flen := len(m.filteredConfigs())
	total := len(m.configs)
	filterInfo := accentStyle.Render(m.filter) + dimStyle.Render(fmt.Sprintf(" (%d/%d)", flen, total))
	if m.showFilter {
		return "\n\n" + dimStyle.Render("Filter:  [1] All") + dimStyle.Render(fmt.Sprintf(" (%d)", total)) +
			dimStyle.Render("  [2] Working  [3] Errored  [4] Unpinged") +
			dimStyle.Render("  [q] Close") +
			"\n" + dimStyle.Render("Current: ") + filterInfo
	}
	return "\n" + dimStyle.Render("Filter: ") + filterInfo + dimStyle.Render("  [f: change]")
}

func (m model) viewConnectServer() string {
	compact := m.termHeight > 0 && m.termHeight < 25
	s := "Connect to Server\n\n"

	searchBar, _ := m.renderSearchBar()
	if !compact {
		s += searchBar + "\n\n"
	}

	if len(m.configs) == 0 {
		s += "No server configurations found.\n"
		s += "Please add configurations first.\n\n"
	} else {
		s += m.renderConfigList(true)
	}

	if compact {
		s += "\n" + searchBar + "\n"
	}

	s += "\n↑/↓: Navigate | S+↑/↓: Multi-select | Space: Select | [ ]: Page | f: Filter | /: Search | i: Details | s: Set Active | Enter: Connect | p: Ping | t: Shell | c: Copy | d: Delete | a: All | D: Dupes | E: Errored | Esc: Back"
	s += m.renderFilterBar()

	if m.analysis != "" {
		s += "\n\n"
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinBlue)).
			Padding(1, 2).
			Width(clamp(40, responsiveWidth(m.termWidth, 80)-6, 120)).
			Render(m.analysis)
		s += resultBox
	}

	if len(m.configs) == 0 {
		s += "Esc: Back to Main Menu"
	}

	width := responsiveWidth(m.termWidth, 80)
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) renderSearchBar() (string, int) {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}

	var bar string
	if m.showSearch {
		m.searchInput.Width = contentWidth - 4
		searchLabel := accentStyle.Render("🔍 Search:")
		bar = searchLabel + " " + inputStyle.Render(m.searchInput.View())
	} else if q := strings.TrimSpace(m.searchInput.Value()); q != "" {
		bar = infoStyle.Render(fmt.Sprintf("🔍 Filtering: \"%s\"  ", q)) +
			dimStyle.Render("[/: change  Esc: clear]")
	} else {
		bar = dimStyle.Render("[/: Search]")
	}
	return bar, contentWidth
}


func (m model) viewIPChanger() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("🔄 IP Changer (Tor)"))
	s.WriteString("\n\n")

	s.WriteString("This will launch the IP Changer tool that uses Tor to rotate your IP address.\n\n")

	// Check if running as root
	isRoot := os.Geteuid() == 0

	if isRoot {
		s.WriteString(successStyle.Render("✅ Running with root privileges"))
		s.WriteString("\n\n")
	} else {
		s.WriteString(warningStyle.Render("⚠️  Root privileges required"))
		s.WriteString("\n\n")
	}

	// Features section with better styling
	featuresTitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(catppuccinRed)).
		Bold(true).
		Render("Features:")

	s.WriteString(featuresTitle)
	s.WriteString("\n")
	s.WriteString("• Automatic Tor installation and configuration\n")
	s.WriteString("• Change IP address at custom intervals\n")
	s.WriteString("• Infinite or fixed number of IP rotations\n")
	s.WriteString("• View current Tor IP address\n\n")

	s.WriteString(successStyle.Render("Press Enter to start IP Changer"))
	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("Esc: Back to Main Menu"))

	width := responsiveWidth(m.termWidth, 80)
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s.String())

	return menuBox
}

func (m model) viewXrayInfo() string {
	s := "Xray Core Info\n\n"
	s += m.renderXrayInfo()
	s += "\n\n"
	s += dimStyle.Render("r: Refresh | Esc: Back to Main Menu")
	width := responsiveWidth(m.termWidth, 80)
	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) viewConfigDetails() string {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := clamp(40, width-6, 120)
	lines := strings.Split(m.configDetail, "\n")
	if len(lines) > 120 {
		lines = append(lines[:120], "... (truncated)")
	}
	body := strings.Join(lines, "\n")

	s := "Config Details\n\n"
	s += lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinBlue)).
		Padding(1, 2).
		Width(contentWidth).
		Render(body)

	if m.configDetailNote != "" {
		s += "\n\n" + infoStyle.Render("Exported to clipboard")
	}

	s += "\n\n" + dimStyle.Render("c: Copy JSON | e: Export link | Esc: Back")

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) viewSubscriptions() string {
	width := responsiveWidth(m.termWidth, 80)
	contentWidth := width - 8
	if contentWidth < 30 {
		contentWidth = 30
	}
	m.subscriptionURL.Width = contentWidth - 4
	if m.subscriptionURL.Width < 20 {
		m.subscriptionURL.Width = 20
	}

	s := "Subscription Manager\n\n"

	if m.showSubsList {
		s += "📋 Saved Subscriptions:\n\n"
		if len(m.savedSubs) == 0 {
			s += dimStyle.Render("No saved subscriptions yet") + "\n\n"
		} else {
			for i, sub := range m.savedSubs {
				style := lipgloss.NewStyle()
				if i == m.selectedSubIndex {
					style = style.Background(lipgloss.Color(catppuccinBlue)).Foreground(lipgloss.Color("#ffffff"))
				}
				s += style.Render(fmt.Sprintf("  %s (%s)", sub.Name, sub.Date)) + "\n"
			}
			s += "\n"
		}

		s += accentStyle.Render("Enter:") + " Load selected subscription\n"
		s += accentStyle.Render("d:") + " Delete selected subscription\n"
		s += accentStyle.Render("↑↓:") + " Navigate\n"
		s += accentStyle.Render("Tab:") + " Switch to URL input\n"
	} else {
		s += "📥 Import New Subscription:\n\n"
		s += "Paste subscription URL (supports both plain text and base64 encoded):\n\n"
		s += inputStyle.Render(m.subscriptionURL.View()) + "\n\n"

		s += accentStyle.Render("Enter:") + " Import & save subscription\n"
		s += accentStyle.Render("Ctrl+V:") + " Paste URL\n"
		s += accentStyle.Render("Ctrl+L:") + " Clear\n"
		s += accentStyle.Render("Tab:") + " View saved subscriptions\n"
	}

	if m.analysis != "" {
		resultBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(catppuccinBlue)).
			Padding(1, 2).
			Width(contentWidth).
			Render(m.analysis)

		s += "\n" + resultBox + "\n\n"
	}

	s += dimStyle.Render("Esc: Back to Main Menu")

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) viewDeleteConfirm(deleteAll bool) string {
	width := responsiveWidth(m.termWidth, 80)
	message := "Delete selected configs? (y/n)"
	if deleteAll {
		message = "Delete ALL configs? (y/n)"
	}

	s := "Confirm Delete\n\n"
	s += warningStyle.Render(message)
	s += "\n\n" + dimStyle.Render("y: Yes | n/Esc: Cancel")

	menuBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(catppuccinInactive)).
		Padding(1, 2).
		Width(width).
		Render(s)

	return menuBox
}

func (m model) launchIPChanger() tea.Cmd {
	return tea.ExitAltScreen
}

func loadSubscriptions() []SubscriptionInfo {
	subsPath := filepath.Join(os.Getenv("HOME"), ".config", "xray-subscriptions.json")
	data, err := os.ReadFile(subsPath)
	if err != nil {
		return []SubscriptionInfo{}
	}
	
	var subs []SubscriptionInfo
	if err := json.Unmarshal(data, &subs); err != nil {
		return []SubscriptionInfo{}
	}
	
	return subs
}

func saveSubscriptions(subs []SubscriptionInfo) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	
	subsPath := filepath.Join(configDir, "xray-subscriptions.json")
	data, err := json.MarshalIndent(subs, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(subsPath, data, 0644)
}

func runIPChangerWithSudo() {
	// Get current executable path
	exe, err := os.Executable()
	if err != nil {
		fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Cannot determine executable path: %v", err)))
		fmt.Println("\nPress Enter to continue...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	// Clear screen and show nice prompt
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to top
	fmt.Println(titleStyle.Render(xrayArt))
	fmt.Println()
	fmt.Println(warningStyle.Render("⚠️  Root privileges required for IP Changer"))
	fmt.Println()

	// Use /dev/tty to ensure proper password input
	cmd := exec.Command("sudo", exe, "--ipchanger-mode")

	// Open /dev/tty for stdin to properly read sudo password
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = tty
		defer tty.Close()
	} else {
		cmd.Stdin = os.Stdin
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Failed to run with sudo: %v", err)))
	}

	fmt.Println()
	fmt.Println(dimStyle.Render("Press Enter to return to menu..."))
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func runIPChangerAsRoot() {
	// Clear screen
	fmt.Print("\033[2J\033[H")

	// Create new IP changer instance
	ic := NewIPChanger()

	// Perform setup
	if err := ic.Setup(); err != nil {
		fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Setup failed: %v", err)))
		fmt.Println()
		fmt.Println(dimStyle.Render("Press Enter to continue..."))
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	// Run the IP changer
	if err := ic.RunIPChanger(); err != nil {
		fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Error: %v", err)))
	}

	fmt.Println()
	fmt.Println(dimStyle.Render("Press Enter to return to menu..."))
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// Helper functions
func (m model) parseInput(input string) string {
	input = strings.TrimSpace(input)

	lines := splitConfigLines(input)
	if len(lines) > 1 {
		return m.parseMultipleConfigs(lines)
	}

	// Detect proxy protocol type and parse accordingly
	if strings.HasPrefix(input, "vless://") {
		return m.parseVlessURL(input)
	} else if strings.HasPrefix(input, "vmess://") {
		return m.parseVmessURL(input)
	} else if strings.HasPrefix(input, "trojan://") {
		return m.parseTrojanURL(input)
	} else if strings.HasPrefix(input, "ss://") {
		return m.parseShadowsocksURL(input)
	} else if strings.HasPrefix(input, "{") {
		// Try to parse as JSON
		return m.parseJSONConfig(input)
	}

	return errorStyle.Render("❌ Unsupported format. Please paste a valid vless://, vmess://, trojan://, ss:// URL or JSON config.")
}

// Parse VLESS URL
func (m model) parseVlessURL(vlessURL string) string {
	configJSON := m.convertVlessToJSON(vlessURL)
	if strings.Contains(configJSON, "❌") {
		return configJSON
	}

	// Extract server info for display
	parsedURL, err := url.Parse(vlessURL)
	if err != nil {
		return errorStyle.Render("❌ Invalid VLESS URL format")
	}

	serverName := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	// Generate config name
	m.currentName = generateConfigName("vless", serverName, port)
	m.configBuffer = configJSON

	return fmt.Sprintf("✅ %s\n📡 Protocol: VLESS\n🖥️  Server: %s\n🔌 Port: %s", 
		successStyle.Render("Valid VLESS configuration"), serverName, port)
}

// Parse VMess URL
func (m model) parseVmessURL(vmessURL string) string {
	configJSON := m.convertVmessToJSON(vmessURL)
	if strings.Contains(configJSON, "❌") {
		return configJSON
	}

	// Extract server info for display
	base64Part := strings.TrimPrefix(vmessURL, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(base64Part)
	if err != nil {
		return errorStyle.Render("❌ Invalid VMess URL encoding")
	}

	var vmess vmessConfig
	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return errorStyle.Render("❌ Invalid VMess configuration format")
	}

	// Generate config name
	m.currentName = generateConfigName("vmess", vmess.ADD, fmt.Sprintf("%d", vmess.Port))
	m.configBuffer = configJSON

	return fmt.Sprintf("✅ %s\n📡 Protocol: VMess\n🖥️  Server: %s\n🔌 Port: %d\n🔐 Security: %s", 
		successStyle.Render("Valid VMess configuration"), vmess.ADD, vmess.Port, vmess.SCY)
}

// Parse Trojan URL
func (m model) parseTrojanURL(trojanURL string) string {
	configJSON := m.convertTrojanToJSON(trojanURL)
	if strings.Contains(configJSON, "❌") {
		return configJSON
	}

	// Extract server info for display
	parsedURL, err := url.Parse(trojanURL)
	if err != nil {
		return errorStyle.Render("❌ Invalid Trojan URL format")
	}

	serverName := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	// Generate config name
	m.currentName = generateConfigName("trojan", serverName, port)
	m.configBuffer = configJSON

	return fmt.Sprintf("✅ %s\n📡 Protocol: Trojan\n🖥️  Server: %s\n🔌 Port: %s", 
		successStyle.Render("Valid Trojan configuration"), serverName, port)
}

// Parse Shadowsocks URL
func (m model) parseShadowsocksURL(ssURL string) string {
	configJSON := m.convertShadowsocksToJSON(ssURL)
	if strings.Contains(configJSON, "❌") {
		return configJSON
	}

	// Extract server info for display
	parsedURL, err := url.Parse(ssURL)
	if err != nil {
		return errorStyle.Render("❌ Invalid Shadowsocks URL format")
	}

	serverName := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "8388"
	}

	// Generate config name
	m.currentName = generateConfigName("shadowsocks", serverName, port)
	m.configBuffer = configJSON

	return fmt.Sprintf("✅ %s\n📡 Protocol: Shadowsocks\n🖥️  Server: %s\n🔌 Port: %s", 
		successStyle.Render("Valid Shadowsocks configuration"), serverName, port)
}

// Parse JSON config
func (m model) parseJSONConfig(jsonStr string) string {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return errorStyle.Render("❌ Invalid JSON format")
	}

	// Try to determine protocol from outbounds
	protocol := "unknown"
	serverAddr := "unknown"
	serverPort := "unknown"

	if outbounds, ok := config["outbounds"].([]interface{}); ok && len(outbounds) > 0 {
		if outbound, ok := outbounds[0].(map[string]interface{}); ok {
			if proto, ok := outbound["protocol"].(string); ok {
				protocol = proto
			}
			if settings, ok := outbound["settings"].(map[string]interface{}); ok {
				if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
					if server, ok := vnext[0].(map[string]interface{}); ok {
						if addr, ok := server["address"].(string); ok {
							serverAddr = addr
						}
						if port, ok := server["port"].(float64); ok {
							serverPort = fmt.Sprintf("%.0f", port)
						}
					}
				}
			}
		}
	}

	// Generate config name
	m.currentName = generateConfigName(protocol, serverAddr, serverPort)
	m.configBuffer = jsonStr

	return fmt.Sprintf("✅ %s\n📡 Protocol: %s\n🖥️  Server: %s\n🔌 Port: %s", 
		successStyle.Render("Valid JSON configuration"), protocol, serverAddr, serverPort)
}

// Helper function to generate config names
func generateConfigName(protocol, server, port string) string {
	// Clean server name
	serverPart := server
	if len(serverPart) > 20 {
		serverPart = serverPart[:17] + "..."
	}
	
	timestamp := time.Now().Format("150405")
	return fmt.Sprintf("%s_%s_%s_%s", protocol, serverPart, port, timestamp)
}

// Now I need to add the missing conversion functions after the end of the file
func (m model) convertVmessToJSON(vmessURL string) string {
	base64Part := strings.TrimPrefix(vmessURL, "vmess://")
	decoded, err := base64.StdEncoding.DecodeString(base64Part)
	if err != nil {
		return errorStyle.Render("❌ Invalid VMess URL encoding")
	}

	var vmess vmessConfig
	if err := json.Unmarshal(decoded, &vmess); err != nil {
		return errorStyle.Render("❌ Invalid VMess configuration format")
	}

	// Convert VMess config to Xray format
	xrayConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks",
				"port":     1080,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth":      "noauth",
					"udp":       true,
					"userLevel": 8,
				},
			},
			map[string]interface{}{
				"tag":      "http",
				"port":     1087,
				"protocol": "http",
				"settings": map[string]interface{}{
					"userLevel": 8,
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"tag":      "proxy",
				"protocol": "vmess",
				"settings": map[string]interface{}{
					"vnext": []interface{}{
						map[string]interface{}{
							"address": vmess.ADD,
							"port":    vmess.Port,
							"users": []interface{}{
								map[string]interface{}{
									"id":       vmess.ID,
									"alterId":  vmess.AID,
									"email":    "t@t.tt",
									"security": vmess.SCY,
								},
							},
						},
					},
				},
				"streamSettings": createStreamSettings(vmess.NET, vmess.Type, vmess.Host, vmess.Path, vmess.TLS),
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return errorStyle.Render("❌ Failed to generate JSON configuration")
	}

	return string(jsonBytes)
}

func (m model) convertTrojanToJSON(trojanURL string) string {
	parsedURL, err := url.Parse(trojanURL)
	if err != nil {
		return errorStyle.Render("❌ Invalid Trojan URL format")
	}

	password := parsedURL.User.Username()
	serverName := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "443"
	}

	portInt := 443
	fmt.Sscanf(port, "%d", &portInt)

	// Parse query parameters
	query := parsedURL.Query()
	security := query.Get("security")
	if security == "" {
		security = "tls"
	}
	sni := query.Get("sni")
	if sni == "" {
		sni = serverName
	}
	path := query.Get("path")
	host := query.Get("host")

	// Convert Trojan config to Xray format
	xrayConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks",
				"port":     1080,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth":      "noauth",
					"udp":       true,
					"userLevel": 8,
				},
			},
			map[string]interface{}{
				"tag":      "http",
				"port":     1087,
				"protocol": "http",
				"settings": map[string]interface{}{
					"userLevel": 8,
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"tag":      "proxy",
				"protocol": "trojan",
				"settings": map[string]interface{}{
					"servers": []interface{}{
						map[string]interface{}{
							"address":  serverName,
							"port":     portInt,
							"password": password,
						},
					},
				},
				"streamSettings": map[string]interface{}{
					"network":  "tcp",
					"security": security,
					"tlsSettings": map[string]interface{}{
						"allowInsecure": false,
						"serverName":    sni,
					},
				},
			},
		},
	}

	// Add websocket settings if path is provided
	if path != "" {
		streamSettings := xrayConfig["outbounds"].([]interface{})[0].(map[string]interface{})["streamSettings"].(map[string]interface{})
		streamSettings["network"] = "ws"
		streamSettings["wsSettings"] = map[string]interface{}{
			"path": path,
		}
		if host != "" {
			streamSettings["wsSettings"].(map[string]interface{})["headers"] = map[string]interface{}{
				"Host": host,
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return errorStyle.Render("❌ Failed to generate JSON configuration")
	}

	return string(jsonBytes)
}

func (m model) convertShadowsocksToJSON(ssURL string) string {
	parsedURL, err := url.Parse(ssURL)
	if err != nil {
		return errorStyle.Render("❌ Invalid Shadowsocks URL format")
	}

	// Decode method and password
	userInfo := parsedURL.User.String()
	decoded, err := base64.StdEncoding.DecodeString(userInfo)
	if err != nil {
		// Try URL decoding if base64 fails
		if userInfo == "" {
			return errorStyle.Render("❌ Missing authentication info in Shadowsocks URL")
		}
		decoded = []byte(userInfo)
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 {
		return errorStyle.Render("❌ Invalid Shadowsocks authentication format")
	}

	method := parts[0]
	password := parts[1]
	serverName := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		port = "8388"
	}

	portInt := 8388
	fmt.Sscanf(port, "%d", &portInt)

	// Convert Shadowsocks config to Xray format
	xrayConfig := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"tag":      "socks",
				"port":     1080,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth":      "noauth",
					"udp":       true,
					"userLevel": 8,
				},
			},
			map[string]interface{}{
				"tag":      "http",
				"port":     1087,
				"protocol": "http",
				"settings": map[string]interface{}{
					"userLevel": 8,
				},
			},
		},
		"outbounds": []interface{}{
			map[string]interface{}{
				"tag":      "proxy",
				"protocol": "shadowsocks",
				"settings": map[string]interface{}{
					"servers": []interface{}{
						map[string]interface{}{
							"address":  serverName,
							"port":     portInt,
							"method":   method,
							"password": password,
						},
					},
				},
			},
		},
	}

	jsonBytes, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		return errorStyle.Render("❌ Failed to generate JSON configuration")
	}

	return string(jsonBytes)
}

// Helper function to create stream settings for VMess
func createStreamSettings(network, streamType, host, path, tls string) map[string]interface{} {
	streamSettings := map[string]interface{}{
		"network": network,
	}

	if tls == "tls" {
		streamSettings["security"] = "tls"
		streamSettings["tlsSettings"] = map[string]interface{}{
			"allowInsecure": false,
		}
		if host != "" {
			streamSettings["tlsSettings"].(map[string]interface{})["serverName"] = host
		}
	}

	switch network {
	case "ws":
		wsSettings := map[string]interface{}{}
		if path != "" {
			wsSettings["path"] = path
		}
		if host != "" {
			wsSettings["headers"] = map[string]interface{}{
				"Host": host,
			}
		}
		streamSettings["wsSettings"] = wsSettings
	case "tcp":
		if streamType == "http" {
			streamSettings["tcpSettings"] = map[string]interface{}{
				"header": map[string]interface{}{
					"type": "http",
					"request": map[string]interface{}{
						"path": []string{"/"},
					},
				},
			}
			if path != "" {
				streamSettings["tcpSettings"].(map[string]interface{})["header"].(map[string]interface{})["request"].(map[string]interface{})["path"] = []string{path}
			}
		}
	}

	return streamSettings
}

func (m model) loadJSONFromPath(path string) string {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errorStyle.Render("❌ Please provide a JSON file path")
	}

	expandedPath, err := expandConfigPath(cleanPath)
	if err != nil {
		return errorStyle.Render(fmt.Sprintf("❌ Invalid path: %v", err))
	}

	if !strings.HasSuffix(strings.ToLower(expandedPath), ".json") {
		return errorStyle.Render("❌ File must have a .json extension")
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return errorStyle.Render(fmt.Sprintf("❌ Unable to read file: %v", err))
	}

	analysis := m.parseJSONConfig(string(data))
	if strings.Contains(analysis, "❌") {
		return analysis
	}

	m.configBuffer = string(data)
	m.currentName = strings.TrimSuffix(filepath.Base(expandedPath), filepath.Ext(expandedPath))
	m.resolvedJSONPath = expandedPath
	return successStyle.Render("✅ JSON file loaded and validated\n") +
		fmt.Sprintf("File: %s", expandedPath)
}

func expandConfigPath(path string) (string, error) {
	expanded := strings.TrimSpace(path)
	expanded = strings.Trim(expanded, "\"'")
	if strings.HasPrefix(expanded, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		if expanded == "~" {
			expanded = homeDir
		} else if strings.HasPrefix(expanded, "~/") {
			expanded = filepath.Join(homeDir, strings.TrimPrefix(expanded, "~/"))
		}
	}

	if !filepath.IsAbs(expanded) {
		absPath, err := filepath.Abs(expanded)
		if err != nil {
			return "", err
		}
		expanded = absPath
	}

	return expanded, nil
}

// Load configurations from the xray-configs directory
func loadConfigs() []ConfigInfo {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "xray-configs")

	// Create directory if it doesn't exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
		return []ConfigInfo{}
	}

	var configs []ConfigInfo

	// Read all JSON files in the directory
	files, err := filepath.Glob(filepath.Join(configDir, "*.json"))
	if err != nil {
		return []ConfigInfo{}
	}

	// Get the currently active config path
	activeConfigPath := filepath.Join(os.Getenv("HOME"), ".config", "xray", "config.json")

	for _, file := range files {
		base := filepath.Base(file)
		if base == "ping_cache.json" || base == "settings.json" {
			continue
		}
		// Read the config file
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Try to parse as JSON to extract server info
		var configData map[string]interface{}
		if err := json.Unmarshal(data, &configData); err != nil {
			continue
		}

		// Extract server and port from config
		server := "Unknown"
		port := 0
		protocol := "Unknown"

		// Try to extract outbound info
		if outbounds, ok := configData["outbounds"].([]interface{}); ok && len(outbounds) > 0 {
			if outbound, ok := outbounds[0].(map[string]interface{}); ok {
				if proto, ok := outbound["protocol"].(string); ok {
					protocol = proto
				}
				if settings, ok := outbound["settings"].(map[string]interface{}); ok {
					if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
						if v, ok := vnext[0].(map[string]interface{}); ok {
							if addr, ok := v["address"].(string); ok {
								server = addr
							}
							if p, ok := v["port"].(float64); ok {
								port = int(p)
							}
						}
					} else if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
						if s, ok := servers[0].(map[string]interface{}); ok {
							if addr, ok := s["address"].(string); ok {
								server = addr
							}
							if p, ok := s["port"].(float64); ok {
								port = int(p)
							}
						}
					}
				}
			}
		}

		// Check if this is the active config
		isActive := false
		if activeLink, err := os.Readlink(activeConfigPath); err == nil {
			isActive = (activeLink == file)
		} else {
			// If not a symlink, compare contents
			if activeData, err := os.ReadFile(activeConfigPath); err == nil {
				isActive = string(data) == string(activeData)
			}
		}

		// Create config info
		name := filepath.Base(file)
		name = strings.TrimSuffix(name, ".json")

		configs = append(configs, ConfigInfo{
			Name:     name,
			Path:     file,
			Active:   isActive,
			Protocol: protocol,
			Server:   server,
			Port:     port,
			Ping:     "",
		})
	}

	configs = applyPingCache(configs, loadPingCache())
	sort.SliceStable(configs, func(i, j int) bool {
		return parsePingMs(configs[i].Ping) < parsePingMs(configs[j].Ping)
	})
	return configs
}

func pingCachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "xray-configs", "ping_cache.json")
}

func loadPingCache() map[string]string {
	cache := map[string]string{}
	data, err := os.ReadFile(pingCachePath())
	if err != nil {
		return cache
	}
	json.Unmarshal(data, &cache)
	return cache
}

func savePingCache(cache map[string]string) {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(pingCachePath())
	os.MkdirAll(dir, 0755)
	os.WriteFile(pingCachePath(), data, 0644)
}

func applyPingCache(configs []ConfigInfo, cache map[string]string) []ConfigInfo {
	for i, cfg := range configs {
		key := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
		if val, ok := cache[key]; ok {
			configs[i].Ping = val
		}
	}
	return configs
}

func readConfigForCopy(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfgMap map[string]interface{}
	if json.Unmarshal(data, &cfgMap) == nil {
		if origin, ok := cfgMap["_origin_url"].(string); ok && origin != "" {
			return origin
		}
		if obs, ok := cfgMap["outbounds"].([]interface{}); ok && len(obs) > 0 {
			if ob, ok := obs[0].(map[string]interface{}); ok {
				proto, _ := ob["protocol"].(string)
				settings, _ := ob["settings"].(map[string]interface{})
				if settings == nil {
					return string(data)
				}
				name := strings.TrimSuffix(filepath.Base(path), ".json")

				switch proto {
				case "vless", "vmess":
					if vnext, ok := settings["vnext"].([]interface{}); ok && len(vnext) > 0 {
						if v, ok := vnext[0].(map[string]interface{}); ok {
							addr, _ := v["address"].(string)
							p, _ := v["port"].(float64)
							if users, ok := v["users"].([]interface{}); ok && len(users) > 0 {
								if user, ok := users[0].(map[string]interface{}); ok {
									id, _ := user["id"].(string)
									if addr != "" && id != "" {
										return fmt.Sprintf("%s://%s@%s:%d#%s", proto, id, addr, int(p), name)
									}
								}
							}
						}
					}

				case "trojan":
					if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
						if s, ok := servers[0].(map[string]interface{}); ok {
							addr, _ := s["address"].(string)
							p, _ := s["port"].(float64)
							pass, _ := s["password"].(string)
							if addr != "" && pass != "" {
								return fmt.Sprintf("trojan://%s@%s:%d#%s", pass, addr, int(p), name)
							}
						}
					}

				case "shadowsocks":
					if servers, ok := settings["servers"].([]interface{}); ok && len(servers) > 0 {
						if s, ok := servers[0].(map[string]interface{}); ok {
							addr, _ := s["address"].(string)
							p, _ := s["port"].(float64)
							method, _ := s["method"].(string)
							pass, _ := s["password"].(string)
							if addr != "" && method != "" && pass != "" {
								auth := base64.StdEncoding.EncodeToString([]byte(method + ":" + pass))
								return fmt.Sprintf("ss://%s@%s:%d#%s", auth, addr, int(p), name)
							}
						}
					}
				}
			}
		}
	}
	return string(data)
}

func isXrayRunning() bool {
	// Check if Xray process is running
	cmd := exec.Command("pgrep", "xray")
	err := cmd.Run()
	return err == nil
}

func findXrayBinary() (string, error) {
	localPath := filepath.Join(".", "xray")
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
		return localPath, nil
	}

	systemPath, err := exec.LookPath("xray")
	if err != nil {
		return "", fmt.Errorf("xray binary not found")
	}
	return systemPath, nil
}

func populateXrayInfo(m model) model {
	binary, err := findXrayBinary()
	if err != nil {
		m.xrayBinaryError = err.Error()
		m.xrayBinaryPath = ""
		m.xrayVersion = ""
		return m
	}

	m.xrayBinaryPath = binary
	m.xrayBinaryError = ""
	version, err := getXrayVersion(binary)
	if err != nil {
		m.xrayVersion = "unknown"
		return m
	}
	m.xrayVersion = version
	return m
}

func getXrayVersion(binary string) (string, error) {
	cmd := exec.Command(binary, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(trimmed), "xray") && strings.Contains(strings.ToLower(trimmed), "version") {
			return trimmed, nil
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}
	return "", fmt.Errorf("version not found")
}

func deleteSelectedConfigs(m model) int {
	deleted := 0
	filtered := m.filteredConfigs()
	for idx := range m.selectedConfigs {
		if idx >= 0 && idx < len(filtered) {
			if err := os.Remove(filtered[idx].Path); err == nil {
				deleted++
			}
		}
	}
	return deleted
}

func deleteAllConfigs(m model) int {
	deleted := 0
	for _, cfg := range m.configs {
		if err := os.Remove(cfg.Path); err == nil {
			deleted++
		}
	}
	return deleted
}

func deleteDuplicateConfigs(configs []ConfigInfo) int {
	seen := make(map[string]bool)
	deleted := 0
	for _, cfg := range configs {
		key := fmt.Sprintf("%s:%d", cfg.Server, cfg.Port)
		if seen[key] {
			if err := os.Remove(cfg.Path); err == nil {
				deleted++
			}
		} else {
			seen[key] = true
		}
	}
	return deleted
}

func deleteInvalidConfigs(configs []ConfigInfo) int {
	deleted := 0
	for _, cfg := range configs {
		if cfg.Ping == "ERR" {
			if err := os.Remove(cfg.Path); err == nil {
				deleted++
			}
		}
	}
	return deleted
}

func setActiveConfig(configPath string) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "xray")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	activePath := filepath.Join(configDir, "config.json")
	_ = os.Remove(activePath)
	return os.Symlink(configPath, activePath)
}

func loadConfigDetails(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return errorStyle.Render(fmt.Sprintf("❌ Unable to read config: %v", err))
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err == nil {
		return pretty.String()
	}
	return string(data)
}

func importSubscription(urlStr string) (int, error) {
	resp, err := http.Get(strings.TrimSpace(urlStr))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("subscription returned %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	decoded, err := decodeSubscriptionPayload(string(data))
	if err != nil {
		return 0, err
	}

	lines := splitConfigLines(string(decoded))
	if len(lines) == 0 {
		return 0, fmt.Errorf("no configs found")
	}

	saved := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "vmess://") || 
		   strings.HasPrefix(line, "vless://") || 
		   strings.HasPrefix(line, "trojan://") || 
		   strings.HasPrefix(line, "ss://") {
			if saveConfigLineStandalone(line) {
				saved++
			}
		}
	}

	return saved, nil
}

func decodeSubscriptionPayload(payload string) ([]byte, error) {
	trimmed := strings.TrimSpace(payload)
	
	// Check if the content looks like plain text proxy URLs
	if strings.Contains(trimmed, "://") {
		// This is likely a plain text subscription, return as-is
		return []byte(trimmed), nil
	}
	
	// Remove all whitespace before base64 decoding (raw data often has newlines)
	cleaned := strings.Join(strings.Fields(trimmed), "")
	
	// Try to decode as base64 subscription
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		// Try URL-safe base64
		decoded, err = base64.URLEncoding.DecodeString(cleaned)
		if err != nil {
			// Try RawStdEncoding
			decoded, err = base64.RawStdEncoding.DecodeString(cleaned)
			if err != nil {
				// Try RawURLEncoding
				decoded, err = base64.RawURLEncoding.DecodeString(cleaned)
				if err != nil {
					// If all base64 decoding fails, treat as plain text
					return []byte(trimmed), nil
				}
			}
		}
	}
	return decoded, nil
}

func saveConfigLineStandalone(line string) bool {
	m := model{}
	return m.saveConfigLine(line)
}

func exportConfigLink(configJSON string) string {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return ""
	}

	outbounds, ok := config["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		return ""
	}

	outbound, ok := outbounds[0].(map[string]interface{})
	if !ok {
		return ""
	}

	protocol, _ := outbound["protocol"].(string)
	settings, _ := outbound["settings"].(map[string]interface{})
	vnext, _ := settings["vnext"].([]interface{})
	if len(vnext) == 0 {
		return ""
	}

	first, ok := vnext[0].(map[string]interface{})
	if !ok {
		return ""
	}

	address, _ := first["address"].(string)
	port := intFromAny(first["port"])
	users, _ := first["users"].([]interface{})
	if len(users) == 0 {
		return ""
	}

	user, ok := users[0].(map[string]interface{})
	if !ok {
		return ""
	}

	id, _ := user["id"].(string)
	if protocol == "vless" {
		return fmt.Sprintf("vless://%s@%s:%d?security=none", id, address, port)
	}

	if protocol == "vmess" {
		vmess := map[string]string{
			"v":    "2",
			"ps":   "exported",
			"add":  address,
			"port": fmt.Sprintf("%d", port),
			"id":   id,
			"aid":  fmt.Sprintf("%d", intFromAny(user["alterId"])),
			"net":  "tcp",
			"type": "",
			"host": "",
			"path": "",
			"tls":  "",
			"scy":  "auto",
		}
		encoded, _ := json.Marshal(vmess)
		return "vmess://" + base64.StdEncoding.EncodeToString(encoded)
	}

	return ""
}

func intFromAny(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var parsed int
		fmt.Sscanf(v, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func runXrayLatency(binary, configPath string, timeout time.Duration, server string, port int) (string, error) {
	if server == "" || port <= 0 {
		return "", fmt.Errorf("missing server or port")
	}

	// Find a random available port for HTTP proxy
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("no available port: %v", err)
	}
	httpPort := httpListener.Addr().(*net.TCPAddr).Port
	httpListener.Close()

	// Read config and inject HTTP inbound on the random port
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("cannot read config: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("invalid config: %v", err)
	}
	// Replace inbounds with a single HTTP on our random port
	cfg["inbounds"] = []map[string]interface{}{
		{
			"listen":   "127.0.0.1",
			"port":     httpPort,
			"protocol": "http",
			"tag":      "ping-in",
		},
	}

	tmpDir := filepath.Join(os.TempDir(), "xray-latency")
	os.MkdirAll(tmpDir, 0755)
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("latency_%d.json", httpPort))
	modData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(tmpPath, modData, 0644)

	// Kill leftover xray processes from previous pings
	exec.Command("pkill", "-f", "xray-latency").Run()
	time.Sleep(50 * time.Millisecond)

	// Start xray
	cmd := exec.Command(binary, "-config", tmpPath)
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("xray start failed: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
		os.Remove(tmpPath)
	}()

	// Wait for xray to start listening
	deadline := time.Now().Add(2 * time.Second)
	var connected bool
	for time.Now().Before(deadline) {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", httpPort), 100*time.Millisecond); err == nil {
			conn.Close()
			connected = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !connected {
		return "", fmt.Errorf("xray did not start in time")
	}

	// Measure real latency through the HTTP proxy
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: timeout,
	}

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.google.com/generate_204", nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start)

	ms := elapsed.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	if ms > 10000 {
		ms = 10000
	}

	return fmt.Sprintf("%dms", ms), nil
}

func estimateProxyOverhead(configPath string) int64 {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 80 // default overhead
	}

	content := strings.ToLower(string(data))
	overhead := int64(40) // base overhead

	// Protocol overhead
	if strings.Contains(content, "vmess") {
		overhead += 30
	} else if strings.Contains(content, "vless") {
		overhead += 20
	} else if strings.Contains(content, "trojan") {
		overhead += 25
	} else if strings.Contains(content, "shadowsocks") {
		overhead += 15
	}

	// Transport overhead
	if strings.Contains(content, "ws") || strings.Contains(content, "websocket") {
		overhead += 35
	}
	if strings.Contains(content, "grpc") {
		overhead += 45
	}
	if strings.Contains(content, "h2") || strings.Contains(content, "http2") {
		overhead += 40
	}

	// Security overhead
	if strings.Contains(content, "tls") || strings.Contains(content, "reality") {
		overhead += 25
	}

	return overhead
}

func (m model) saveConfig() tea.Cmd {
	return func() tea.Msg {
		// Create config directory
		configDir := filepath.Join(os.Getenv("HOME"), ".config", "xray-configs")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil
		}

		// Generate filename from current name or timestamp
		var filename string
		if m.currentName != "" {
			filename = m.currentName + ".json"
		} else {
			filename = fmt.Sprintf("config_%s.json", time.Now().Format("20060102_150405"))
		}

		savePath := filepath.Join(configDir, filename)

		// Save the configuration
		var configData []byte
		var originURL string
		if m.configBuffer != "" {
			configData = []byte(m.configBuffer)
		} else if strings.TrimSpace(m.pasteArea.Value()) != "" {
			// If pasting a vless/vmess URL, convert to JSON config
			input := strings.TrimSpace(m.pasteArea.Value())
			if strings.HasPrefix(input, "vless://") {
				originURL = input
				configData = []byte(m.convertVlessToJSON(input))
			} else if strings.HasPrefix(input, "vmess://") {
				originURL = input
				configData = []byte(m.convertVmessToJSON(input))
			} else {
				configData = []byte(input)
			}
		}

		if originURL != "" {
			var cfgMap map[string]interface{}
			if json.Unmarshal(configData, &cfgMap) == nil {
				cfgMap["_origin_url"] = originURL
				configData, _ = json.MarshalIndent(cfgMap, "", "  ")
			}
		}

		if len(configData) > 0 {
			os.WriteFile(savePath, configData, 0644)
		}

		return nil
	}
}

func (m model) saveConfigLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	configDir := filepath.Join(os.Getenv("HOME"), ".config", "xray-configs")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return false
	}

	var filename string
	if strings.HasPrefix(line, "vless://") {
		configName := extractNameFromLine(line)
		if configName != "" {
			filename = configName + ".json"
		}
	} else if strings.HasPrefix(line, "vmess://") {
		configName := extractNameFromLine(line)
		if configName != "" {
			filename = configName + ".json"
		}
	}

	if filename == "" {
		filename = fmt.Sprintf("config_%s_%d.json", time.Now().Format("20060102_150405"), time.Now().UnixNano())
	}

	savePath := filepath.Join(configDir, filename)
	configData := []byte{}
	if strings.HasPrefix(line, "vless://") {
		configData = []byte(m.convertVlessToJSON(line))
	} else if strings.HasPrefix(line, "vmess://") {
		configData = []byte(m.convertVmessToJSON(line))
	} else if strings.HasPrefix(line, "trojan://") {
		configData = []byte(m.convertTrojanToJSON(line))
	} else if strings.HasPrefix(line, "ss://") {
		configData = []byte(m.convertShadowsocksToJSON(line))
	} else if strings.HasPrefix(line, "{") {
		configData = []byte(line)
	}

	if len(configData) > 0 {
		// Embed original URL for clipboard copy
		if strings.HasPrefix(line, "vless://") || strings.HasPrefix(line, "vmess://") ||
			strings.HasPrefix(line, "trojan://") || strings.HasPrefix(line, "ss://") {
			var cfgMap map[string]interface{}
			if json.Unmarshal(configData, &cfgMap) == nil {
				cfgMap["_origin_url"] = line
				configData, _ = json.MarshalIndent(cfgMap, "", "  ")
			}
		}
		if err := os.WriteFile(savePath, configData, 0644); err == nil {
			return true
		}
	}
	return false
}

func extractNameFromLine(line string) string {
	if strings.HasPrefix(line, "vless://") {
		urlStr := strings.TrimPrefix(line, "vless://")
		u, err := url.Parse("vless://" + urlStr)
		if err == nil && u.Fragment != "" {
			name, _ := url.QueryUnescape(u.Fragment)
			return name
		}
	}
	if strings.HasPrefix(line, "vmess://") {
		vmess, err := decodeVmessURL(line)
		if err == nil && vmess.PS != "" {
			return vmess.PS
		}
	}
	return ""
}

func (m model) convertVlessToJSON(vlessURL string) string {
	// Simple vless to JSON converter
	// This is a basic implementation - expand based on your needs
	urlStr := strings.TrimPrefix(vlessURL, "vless://")
	u, err := url.Parse("vless://" + urlStr)
	if err != nil {
		return "{}"
	}

	uuid := u.User.Username()
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}

	query := u.Query()
	security := query.Get("security")
	if security == "" {
		security = "none"
	}

	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			{
				"protocol": "vless",
				"settings": map[string]interface{}{
					"vnext": []map[string]interface{}{
						{
							"address": host,
							"port":    port,
							"users": []map[string]interface{}{
								{
									"id":         uuid,
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]interface{}{
					"security": security,
				},
			},
		},
	}

	jsonData, _ := json.MarshalIndent(config, "", "  ")
	return string(jsonData)
}

func decodeVmessURL(vmessURL string) (vmessConfig, error) {
	encoded := strings.TrimPrefix(strings.TrimSpace(vmessURL), "vmess://")
	if encoded == "" {
		return vmessConfig{}, fmt.Errorf("missing payload")
	}
	encoded = strings.Trim(encoded, "=\r\n\t ")

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			decoded, err = base64.RawURLEncoding.DecodeString(encoded)
			if err != nil {
				decoded, err = base64.RawURLEncoding.DecodeString(strings.ReplaceAll(encoded, "-", "+"))
				if err != nil {
					return vmessConfig{}, err
				}
			}
		}
	}

	var config vmessConfig
	if err := json.Unmarshal(decoded, &config); err != nil {
		return vmessConfig{}, err
	}

	if config.ADD == "" || config.ID == "" {
		return vmessConfig{}, fmt.Errorf("missing required fields")
	}

	if config.Port == 0 {
		return vmessConfig{}, fmt.Errorf("invalid port")
	}

	if config.SCY == "" {
		config.SCY = "auto"
	}
	if config.TLS == "" {
		config.TLS = "none"
	}
	if config.NET == "" {
		config.NET = "tcp"
	}

	return config, nil
}

func splitConfigLines(input string) []string {
	var lines []string
	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func (m model) parseMultipleConfigs(lines []string) string {
	var parsed int
	var names []string

	for _, line := range lines {
		var result string
		switch {
		case strings.HasPrefix(line, "vless://"):
			result = m.parseVlessURL(line)
		case strings.HasPrefix(line, "vmess://"):
			result = m.parseVmessURL(line)
		case strings.HasPrefix(line, "{"):
			result = m.parseJSONConfig(line)
		default:
			continue
		}

		if strings.Contains(result, "❌") {
			continue
		}
		parsed++
		if m.currentName != "" {
			names = append(names, m.currentName)
		}
	}

	if parsed == 0 {
		return errorStyle.Render("❌ No valid configs found in input")
	}

	m.currentName = ""
	return successStyle.Render(fmt.Sprintf("✅ %d configurations detected\n", parsed)) +
		"Press Enter to save all configurations."
}

func (m model) connectToServer(config ConfigInfo) tea.Cmd {
	return func() tea.Msg {
		// Stop any existing Xray process (exact name match only, not xray-app)
		_ = exec.Command("pkill", "-x", "xray").Run()

		configPath := config.Path
		if configPath == "" {
			return connectionResult{
				success: false,
				message: "❌ No config path provided",
			}
		}

		// Read the original config and inject inbounds if missing
		data, err := os.ReadFile(configPath)
		if err != nil {
			return connectionResult{
				success: false,
				message: fmt.Sprintf("❌ Failed to read config: %v", err),
			}
		}

		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return connectionResult{
				success: false,
				message: fmt.Sprintf("❌ Failed to parse config: %v", err),
			}
		}

		// Inject inbounds using settings ports
		cfg["inbounds"] = []map[string]interface{}{
			{
				"listen":   "127.0.0.1",
				"port":     m.settingsHTTPPort,
				"protocol": "http",
				"tag":      "http-in",
			},
			{
				"listen":   "127.0.0.1",
				"port":     m.settingsSOCKSPort,
				"protocol": "socks",
				"settings": map[string]interface{}{"udp": true},
				"tag":      "socks-in",
			},
		}

		// Write to a temp file
		tmpDir := filepath.Join(os.TempDir(), "xray-app")
		os.MkdirAll(tmpDir, 0755)
		tmpPath := filepath.Join(tmpDir, "runtime_config.json")
		modifiedData, _ := json.MarshalIndent(cfg, "", "  ")
		if err := os.WriteFile(tmpPath, modifiedData, 0644); err != nil {
			return connectionResult{
				success: false,
				message: fmt.Sprintf("❌ Failed to write runtime config: %v", err),
			}
		}

		cmd := exec.Command("xray", "-config", tmpPath)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err = cmd.Start()
		if err != nil {
			return connectionResult{
				success: false,
				message: fmt.Sprintf("❌ Failed to start Xray: %v", err),
			}
		}

		pid := 0
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}

		time.Sleep(500 * time.Millisecond)

		return connectionResult{
			success: true,
			message: fmt.Sprintf("✅ Connected to %s (%s:%d) [PID %d]\n🌐 HTTP:   http://127.0.0.1:%d\n🧦 SOCKS5: socks5://127.0.0.1:%d", config.Name, config.Server, config.Port, pid, m.settingsHTTPPort, m.settingsSOCKSPort),
		}
	}
}

func (m model) generateSocksConfig() string {
	// Generate SOCKS configuration based on input fields
	ip := strings.TrimSpace(m.socksIP.Value())
	port := strings.TrimSpace(m.socksPort.Value())
	username := strings.TrimSpace(m.socksUsername.Value())
	password := strings.TrimSpace(m.socksPassword.Value())

	// Basic validation
	if ip == "" || port == "" {
		return errorStyle.Render("❌ Server IP and Port are required")
	}

	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil || portNum < 1 || portNum > 65535 {
		return errorStyle.Render("❌ Invalid port number")
	}

	// Create Xray config JSON for SOCKS outbound
	config := map[string]interface{}{
		"outbounds": []map[string]interface{}{
			{
				"protocol": "socks",
				"settings": map[string]interface{}{
					"servers": []map[string]interface{}{
						{
							"address": ip,
							"port":    portNum,
						},
					},
				},
			},
		},
		"inbounds": []map[string]interface{}{
			{
				"listen":   "127.0.0.1",
				"port":     10808,
				"protocol": "http",
			},
		},
	}

	// Add auth if provided
	if username != "" || password != "" {
		sockServer := config["outbounds"].([]map[string]interface{})[0]["settings"].(map[string]interface{})["servers"].([]map[string]interface{})[0]
		sockServer["users"] = []map[string]interface{}{
			{
				"user": username,
				"pass": password,
			},
		}
	}

	jsonBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errorStyle.Render(fmt.Sprintf("❌ JSON generation failed: %v", err))
	}

	return string(jsonBytes)
}

func main() {
	// Check if we're being called in IP changer mode (from sudo)
	if len(os.Args) > 1 && os.Args[1] == "--ipchanger-mode" {
		// Clear screen
		fmt.Print("\033[2J\033[H")

		// Run IP changer directly
		ic := NewIPChanger()

		// Perform setup
		if err := ic.Setup(); err != nil {
			fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Setup failed: %v", err)))
			os.Exit(1)
		}

		// Run the IP changer
		if err := ic.RunIPChanger(); err != nil {
			fmt.Printf("\n%s\n", errorStyle.Render(fmt.Sprintf("❌ Error: %v", err)))
			os.Exit(1)
		}

		return
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

func (m model) attemptConnection(config ConfigInfo) tea.Cmd {
	return func() tea.Msg {
		return connectionResult{
			success: true,
			message: fmt.Sprintf("✅ Connected to %s (%s:%d)\n🔗 Xray would be running\n🌐 Local HTTP proxy: http://127.0.0.1:10809\n🧅 Local SOCKS proxy: socks5://127.0.0.1:10808", config.Name, config.Server, config.Port),
		}
	}
}

func settingsPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "xray-configs", "settings.json")
}

func loadSettingsInt(key string, defaultVal int) int {
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return defaultVal
	}
	var s map[string]int
	if json.Unmarshal(data, &s) != nil {
		return defaultVal
	}
	if v, ok := s[key]; ok {
		return v
	}
	return defaultVal
}

func saveSettings(httpPort, socksPort, pingTimeout int) {
	dir := filepath.Join(os.Getenv("HOME"), ".config", "xray-configs")
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(map[string]int{
		"http_port":    httpPort,
		"socks_port":   socksPort,
		"ping_timeout": pingTimeout,
	}, "", "  ")
	os.WriteFile(filepath.Join(dir, "settings.json"), data, 0644)
}
