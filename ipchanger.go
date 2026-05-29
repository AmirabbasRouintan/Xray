package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/proxy"
)

const (
	torrcFile      = "/etc/tor/torrc"
	socksPort      = "9050"
	listenAddress  = "127.0.0.1"
	checkIPURL     = "https://checkip.amazonaws.com"
)

// IPChanger handles Tor-based IP rotation
type IPChanger struct {
	isRunning bool
	currentIP string
}

// NewIPChanger creates a new IP changer instance
func NewIPChanger() *IPChanger {
	return &IPChanger{
		isRunning: false,
		currentIP: "",
	}
}

// CheckTorInstalled checks if Tor is installed
func (ic *IPChanger) CheckTorInstalled() bool {
	_, err := exec.LookPath("tor")
	return err == nil
}

// InstallTor installs Tor based on the Linux distribution
func (ic *IPChanger) InstallTor() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root privileges required. Please run with sudo")
	}

	// Detect distribution
	distro, err := ic.detectDistribution()
	if err != nil {
		return fmt.Errorf("failed to detect distribution: %v", err)
	}

	fmt.Printf("Installing Tor on %s...\n", distro)

	var cmd *exec.Cmd
	switch {
	case strings.Contains(distro, "Ubuntu"), strings.Contains(distro, "Debian"):
		cmd = exec.Command("apt-get", "update")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update package list: %v", err)
		}
		cmd = exec.Command("apt-get", "install", "-y", "tor", "tor-geoipdb", "curl")
	case strings.Contains(distro, "Fedora"), strings.Contains(distro, "CentOS"), 
		 strings.Contains(distro, "Red Hat"), strings.Contains(distro, "Amazon Linux"):
		cmd = exec.Command("yum", "update", "-y")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to update package list: %v", err)
		}
		cmd = exec.Command("yum", "install", "-y", "tor", "curl")
	case strings.Contains(distro, "Arch"), strings.Contains(distro, "Manjaro"):
		cmd = exec.Command("pacman", "-Sy", "--noconfirm", "tor", "curl")
	default:
		return fmt.Errorf("unsupported distribution: %s. Please install tor manually", distro)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("installation failed: %v", err)
	}

	fmt.Println("Installation complete.")
	return nil
}

// detectDistribution detects the Linux distribution
func (ic *IPChanger) detectDistribution() (string, error) {
	data, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "NAME=") {
			distro := strings.TrimPrefix(line, "NAME=")
			distro = strings.Trim(distro, "\"")
			return distro, nil
		}
	}

	return "Unknown", nil
}

// ConfigureTor configures Tor with SOCKS proxy settings
func (ic *IPChanger) ConfigureTor() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("root privileges required. Please run with sudo")
	}

	fmt.Println("Configuring Tor...")

	// Backup original torrc if not already backed up
	backupFile := torrcFile + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		input, err := ioutil.ReadFile(torrcFile)
		if err == nil {
			ioutil.WriteFile(backupFile, input, 0644)
		}
	}

	// Read current torrc
	data, err := ioutil.ReadFile(torrcFile)
	if err != nil {
		return fmt.Errorf("failed to read torrc: %v", err)
	}

	// Remove existing SocksPort lines
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "SocksPort") && !strings.HasPrefix(line, "#SocksPort") {
			newLines = append(newLines, line)
		}
	}

	// Add our SocksPort configuration
	newLines = append(newLines, fmt.Sprintf("SocksPort %s:%s", listenAddress, socksPort))

	// Add ExitNodes if not present
	hasExitNodes := false
	for _, line := range newLines {
		if strings.HasPrefix(line, "ExitNodes") {
			hasExitNodes = true
			break
		}
	}
	if !hasExitNodes {
		newLines = append(newLines, "# ExitNodes {us}, {gb}, {fr}, {de}")
	}

	// Write back to torrc
	newContent := strings.Join(newLines, "\n")
	if err := ioutil.WriteFile(torrcFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write torrc: %v", err)
	}

	fmt.Println("Tor configuration updated.")
	return nil
}

// StartTorService starts the Tor service
func (ic *IPChanger) StartTorService() error {
	fmt.Println("Starting Tor service...")

	// Enable Tor service
	exec.Command("systemctl", "enable", "tor").Run()
	exec.Command("systemctl", "daemon-reload").Run()

	// Start Tor
	cmd := exec.Command("systemctl", "start", "tor")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Tor: %v", err)
	}

	// Restart to apply changes
	cmd = exec.Command("systemctl", "restart", "tor")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart Tor: %v", err)
	}

	// Wait for Tor to be ready
	time.Sleep(3 * time.Second)

	// Check if Tor is running
	cmd = exec.Command("systemctl", "is-active", "tor")
	if err := cmd.Run(); err != nil {
		fmt.Println("Warning: Tor service may not be fully active, attempting to continue...")
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Tor service started.")
	return nil
}

// GetIP retrieves the current IP address through Tor
func (ic *IPChanger) GetIP() (string, error) {
	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%s", listenAddress, socksPort), nil, proxy.Direct)
	if err != nil {
		return "Unable to retrieve IP", err
	}

	// Create HTTP client with SOCKS5 proxy
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dialer.Dial,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(checkIPURL)
	if err != nil {
		return "Unable to retrieve IP", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "Unable to retrieve IP", err
	}

	ip := strings.TrimSpace(string(body))
	ic.currentIP = ip
	return ip, nil
}

// ChangeIP changes the IP address by reloading Tor
func (ic *IPChanger) ChangeIP() (string, error) {
	fmt.Println("Changing IP address...")

	// Try reload first, fall back to restart
	cmd := exec.Command("systemctl", "reload", "tor.service")
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("systemctl", "restart", "tor.service")
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to reload/restart Tor: %v", err)
		}
	}

	time.Sleep(2 * time.Second)

	newIP, err := ic.GetIP()
	if err != nil {
		return "", err
	}

	return newIP, nil
}

// VerifyTorConnection verifies that Tor is working
func (ic *IPChanger) VerifyTorConnection() error {
	fmt.Println("Verifying Tor connection...")

	for i := 0; i < 5; i++ {
		_, err := ic.GetIP()
		if err == nil {
			fmt.Println("Tor is working correctly.")
			return nil
		}
		if i == 4 {
			return fmt.Errorf("could not verify Tor connection after 5 attempts")
		}
		time.Sleep(2 * time.Second)
	}

	return nil
}

// Setup performs the complete setup (install, configure, start)
func (ic *IPChanger) Setup() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("script must be run as root. Please run with sudo")
	}

	fmt.Println("Checking system requirements...")

	// Install Tor if not present
	if !ic.CheckTorInstalled() {
		fmt.Println("Tor is not installed. Installing Tor and dependencies...")
		if err := ic.InstallTor(); err != nil {
			return err
		}
	} else {
		fmt.Println("Tor is already installed.")
	}

	// Configure Tor
	if err := ic.ConfigureTor(); err != nil {
		return err
	}

	// Start Tor service
	if err := ic.StartTorService(); err != nil {
		return err
	}

	// Verify connection
	if err := ic.VerifyTorConnection(); err != nil {
		fmt.Printf("Warning: %v. The script will continue but IP changes may not work.\n", err)
	}

	return nil
}

// RunIPChanger runs the interactive IP changer
func (ic *IPChanger) RunIPChanger() error {
	// Display ASCII art
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccinGreen)).Render(`
██╗██╗  ██╗██╗        ███████╗██╗      ██████╗ ██╗    ██╗███████╗██████╗ 
██║╚██╗██╔╝██║        ██╔════╝██║     ██╔═══██╗██║    ██║██╔════╝██╔══██╗
██║ ╚███╔╝ ██║        █████╗  ██║     ██║   ██║██║ █╗ ██║█████╗  ██████╔╝
██║ ██╔██╗ ██║        ██╔══╝  ██║     ██║   ██║██║███╗██║██╔══╝  ██╔══██╗
██║██╔╝ ██╗██║███████╗██║     ███████╗╚██████╔╝╚███╔███╔╝███████╗██║  ██║
╚═╝╚═╝  ╚═╝╚═╝╚══════╝╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝ ╚══════╝╚═╝  ╚═╝
	`))

	// Get initial IP
	initialIP, err := ic.GetIP()
	if err != nil {
		fmt.Printf("Warning: Could not get initial IP: %v\n", err)
		initialIP = "Unknown"
	}
	fmt.Printf("\n%s\n\n", successStyle.Render(fmt.Sprintf("Initial Tor IP address: %s", initialIP)))

	reader := bufio.NewReader(os.Stdin)

	for {
		// Get interval
		fmt.Print(infoStyle.Render("Time Interval? (type 0 for infinite): "))
		intervalStr, _ := reader.ReadString('\n')
		intervalStr = strings.TrimSpace(intervalStr)
		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			fmt.Println(errorStyle.Render("Invalid input. Please enter a number."))
			continue
		}

		// Get number of times
		fmt.Print(infoStyle.Render("How many IP? (0 for infinite): "))
		timesStr, _ := reader.ReadString('\n')
		timesStr = strings.TrimSpace(timesStr)
		times, err := strconv.Atoi(timesStr)
		if err != nil {
			fmt.Println(errorStyle.Render("Invalid input. Please enter a number."))
			continue
		}

		if times == 0 {
			fmt.Println("Starting infinite IP changes (interval range: 10-20s)")
			for {
				newIP, err := ic.ChangeIP()
				if err != nil {
					fmt.Printf(errorStyle.Render("Error changing IP: %v\n"), err)
				} else {
					fmt.Printf("%s\n", infoStyle.Render(fmt.Sprintf("New IP address: %s", newIP)))
				}

				// Random interval between 10-20 seconds
				randomInterval := rand.Intn(11) + 10
				time.Sleep(time.Duration(randomInterval) * time.Second)
			}
		} else {
			for i := 0; i < times; i++ {
				newIP, err := ic.ChangeIP()
				if err != nil {
					fmt.Printf(errorStyle.Render("Error changing IP: %v\n"), err)
				} else {
					fmt.Printf("%s\n", infoStyle.Render(fmt.Sprintf("New IP address: %s", newIP)))
				}
				time.Sleep(time.Duration(interval) * time.Second)
			}
			fmt.Printf("\n%s\n", successStyle.Render(fmt.Sprintf("Finished cycling IP address %d times.", times)))
			break
		}
	}

	// Get final IP
	finalIP, err := ic.GetIP()
	if err != nil {
		finalIP = "Unknown"
	}
	fmt.Printf("%s\n", successStyle.Render(fmt.Sprintf("Final IP address: %s", finalIP)))

	return nil
}

