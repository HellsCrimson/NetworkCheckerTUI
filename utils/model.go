package utils

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Init() tea.Cmd {
	return Tick()
}

type Model struct {
	AvailableChoices []AvailableChoice
	Choice           int
	Chosen           bool
	Ticks            int
	Frames           int
	Progress         float64
	Loaded           bool
	Quitting         bool

	Logging bool

	// ping-specific fields
	PingIP           string
	PingTotal        int
	PingCount        int
	PingSuccessCount int
	PingChan         chan PingResult
	PingLog          []string // collect per-ping results

	// mtu-specific fields
	MTUTargets      []int
	MTUIndex        int
	MTUSuccessCount int
	MTUChan         chan MtuResult
	MTULog          []string // collect per-mtu results

	// dns-specific fields
	DNSTargets      []string
	DNSIndex        int
	DNSSuccessCount int
	DNSChan         chan DnsResult
	DNSLog          []string

	// full-check orchestration
	FullStage     int // 0=not started, 1=ip,2=mtu,3=dns,4=done
	FullTotal     int // total number of individual checks
	FullCompleted int // how many checks completed

	// DHCP-specific fields
	DHCPTimeout int
	DHCPLog     []string
	DHCPChan    chan string
	DHCPFound   bool
	DHCPInfo    string

	// ARP-specific fields
	ARPChan chan string
	ARPLog  []string

	// Routing-specific fields
	RouteChan chan string
	RouteLog  []string

	// Firewall-specific fields
	FirewallChan chan string
	FirewallLog  []string

	// Open ports-specific fields
	OpenPortsChan chan string
	OpenPortsLog  []string

	// Traceroute-specific fields
	TraceChan   chan string
	TraceLog    []string
	TraceTarget string

	// Bandwidth-specific fields
	BandwidthChan chan string
	BandwidthLog  []string

	// Latency-specific fields
	LatencyChan chan string
	LatencyLog  []string

	// Packet loss-specific fields
	PacketLossChan chan string
	PacketLossLog  []string

	// VPN-specific fields
	VPNChan chan string
	VPNLog  []string

	// Wi-Fi-specific fields
	WiFiChan chan string
	WiFiLog  []string

	// Network interfaces-specific fields
	NetIfChan chan string
	NetIfLog  []string

	// Proxy-specific fields
	ProxyChan chan string
	ProxyLog  []string

	// NAT-specific fields
	NATChan chan string
	NATLog  []string

	// QoS-specific fields
	QoSChan chan string
	QoSLog  []string
}

type AvailableChoice struct {
	Name       string
	UpdateFunc func(tea.Msg, Model) (tea.Model, tea.Cmd)
	ViewFunc   func(Model) string
}

// Main update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// global key handling (quit)
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		// toggle logging with "v"
		if k == "v" {
			if m.Logging {
				// disable logging: close file if open
				if LoggingFile != nil {
					_ = LoggingFile.Close()
					LoggingFile = nil
				}
				m.Logging = false
			} else {
				// enable logging -> create/overwrite debug.log
				f, err := tea.LogToFile("debug.log", "debug")
				if err == nil {
					LoggingFile = f
					m.Logging = true
				}
				// if err, ignore — UI will continue without logging
			}
			return m, nil
		}

		if k == "q" || k == "esc" || k == "ctrl+c" {
			m.Quitting = true
			// ensure log file closed on quit
			if LoggingFile != nil {
				_ = LoggingFile.Close()
				LoggingFile = nil
			}
			return m, tea.Quit
		}

		// go back to the choices view when a test has finished
		if k == "b" && m.Chosen && m.Loaded {
			// return to the menu and reset running state so tests can be rerun
			m.Chosen = false
			m.Loaded = false

			// clear any worker channels so goroutines can be restarted later
			m.PingChan = nil
			m.MTUChan = nil
			m.DNSChan = nil
			m.ARPChan = nil
			m.FirewallChan = nil
			m.OpenPortsChan = nil

			// reset progress counters
			m.PingCount = 0
			m.PingSuccessCount = 0
			m.MTUIndex = 0
			m.MTUSuccessCount = 0
			m.DNSIndex = 0
			m.DNSSuccessCount = 0

			return m, nil
		}
	}

	// Hand off the message and model to the appropriate update function for the
	// appropriate view based on the current state.
	if !m.Chosen {
		return updateChoices(msg, m)
	}
	return updateChosen(msg, m)
}

// The main view, which just calls the appropriate sub-view
func (m Model) View() string {
	var s string
	if m.Quitting {
		return "\n  See you later!\n\n"
	}
	if !m.Chosen {
		s = choicesView(m)
	} else {
		s = chosenView(m)
	}
	return MainStyle.Render("\n" + s + "\n\n")
}

// Sub-update functions

// Update loop for the first view where you're choosing a task.
func updateChoices(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.Choice++
			if m.Choice > len(m.AvailableChoices)-1 {
				m.Choice = len(m.AvailableChoices) - 1
			}
		case "k", "up":
			m.Choice--
			if m.Choice < 0 {
				m.Choice = 0
			}
		case "enter", " ":
			m.Chosen = true
			return m, Frame()
		}
	}

	return m, nil
}

func updateChosen(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	if m.Choice < 0 || m.Choice >= len(m.AvailableChoices) {
		return m, nil
	}
	return m.AvailableChoices[m.Choice].UpdateFunc(msg, m)
}

func chosenView(m Model) string {
	if m.Choice < 0 || m.Choice >= len(m.AvailableChoices) {
		return "Invalid choice"
	}
	return m.AvailableChoices[m.Choice].ViewFunc(m)
}

func choicesView(m Model) string {
	c := m.Choice

	tpl := "What to do today?\n\n"
	tpl += "%s\n\n"
	tpl += SubtleStyle.Render("j/k, up/down: select") + DotStyle +
		SubtleStyle.Render("enter: choose") + DotStyle +
		SubtleStyle.Render("q, esc: quit") + DotStyle +
		SubtleStyle.Render(fmt.Sprintf("v: toggle logging (%s)", map[bool]string{true: "on", false: "off"}[m.Logging]))

	choices := ""
	for idx, choice := range m.AvailableChoices {
		choices += fmt.Sprintf("%s\n", Checkbox(choice.Name, idx == c))
	}

	return fmt.Sprintf(tpl, choices)
}
