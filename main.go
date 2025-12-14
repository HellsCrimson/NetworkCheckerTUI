package main

// An example demonstrating an application with multiple views.
//
// Note that this example was produced before the Bubbles progress component
// was available (github.com/charmbracelet/bubbles/progress) and thus, we're
// implementing a progress bar from scratch here.

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	progressBarWidth  = 71
	progressFullChar  = "█"
	progressEmptyChar = "░"
	dotChar           = " • "
)

// General stuff for styling the view
var (
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	progressEmpty = subtleStyle.Render(progressEmptyChar)
	dotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(dotChar)
	mainStyle     = lipgloss.NewStyle().MarginLeft(2)

	// Gradient colors we'll use for the progress bar
	ramp = makeRampStyles("#B14FFF", "#00FFA3", progressBarWidth)

	loggingFile *os.File
)

func main() {
	// initialize with field names so struct changes are safe
	initialModel := model{
		AvailableChoices: []AvailableChoice{
			{
				Name:       "Full network check",
				UpdateFunc: updateFullNetwork,
				ViewFunc:   chosenFullNetworkView,
			},
			{
				Name:       "Check IP",
				UpdateFunc: updateIPRouting,
				ViewFunc:   chosenIPRoutingView,
			},
			{
				Name:       "Check DNS",
				UpdateFunc: updateDNS,
				ViewFunc:   chosenDNSView,
			},
			{
				Name:       "Check MTU",
				UpdateFunc: updateMTU,
				ViewFunc:   chosenMTUView,
			},
			{
				Name:       "Frame analyzer",
				UpdateFunc: updateFrameAnalyzer,
				ViewFunc:   chosenFrameAnalyzerView,
			},
			{
				Name:       "Check DHCP",
				UpdateFunc: updateDHCP,
				ViewFunc:   chosenDHCPView,
			},
			{
				Name:       "Check ARP tables",
				UpdateFunc: updateARP,
				ViewFunc:   chosenARPView,
			},
			{
				Name:       "Check routing tables",
				UpdateFunc: updateRouting,
				ViewFunc:   chosenRoutingView,
			},
			{
				Name:       "Check firewall rules",
				UpdateFunc: updateFirewall,
				ViewFunc:   chosenFirewallView,
			},
			{
				Name:       "Check open ports",
				UpdateFunc: updateOpenPorts,
				ViewFunc:   chosenOpenPortsView,
			},
			{
				Name:       "Check traceroute",
				UpdateFunc: updateTraceroute,
				ViewFunc:   chosenTracerouteView,
			},
			{
				Name:       "Check bandwidth",
				UpdateFunc: updateBandwidth,
				ViewFunc:   chosenBandwidthView,
			},
			{
				Name:       "Check latency",
				UpdateFunc: updateLatency,
				ViewFunc:   chosenLatencyView,
			},
			{
				Name:       "Check packet loss",
				UpdateFunc: updatePacketLoss,
				ViewFunc:   chosenPacketLossView,
			},
			{
				Name:       "Check VPN status",
				UpdateFunc: updateVPN,
				ViewFunc:   chosenVPNView,
			},
			{
				Name:       "Check Wi-Fi signal",
				UpdateFunc: updateWiFi,
				ViewFunc:   chosenWiFiView,
			},
			{
				Name:       "Check network interfaces",
				UpdateFunc: updateNetworkInterfaces,
				ViewFunc:   chosenNetworkInterfacesView,
			},
			{
				Name:       "Check proxy settings",
				UpdateFunc: updateProxy,
				ViewFunc:   chosenProxyView,
			},
			{
				Name:       "Check NAT configuration",
				UpdateFunc: updateNAT,
				ViewFunc:   chosenNATView,
			},
			{
				Name:       "Check QoS settings",
				UpdateFunc: updateQoS,
				ViewFunc:   chosenQoSView,
			},
		},
		Choice:           0,
		Chosen:           false,
		Ticks:            10,
		Frames:           0,
		Progress:         0,
		Loaded:           false,
		Quitting:         false,
		Logging:          false,
		PingIP:           "8.8.8.8",
		PingTotal:        4,
		PingCount:        0,
		PingSuccessCount: 0,
		PingChan:         nil,
		MTUTargets:       []int{500, 1000, 1400, 1500, 9000},
		MTUIndex:         0,
		MTUSuccessCount:  0,
		MTUChan:          nil,

		// result logs
		PingLog: []string{},
		MTULog:  []string{},

		// DNS defaults
		DNSTargets:      []string{"localhost", "example.com"},
		DNSIndex:        0,
		DNSSuccessCount: 0,
		DNSChan:         nil,
		DNSLog:          []string{},

		// DHCP defaults
		DHCPTimeout: 5,
		DHCPLog:     []string{},
		DHCPChan:    nil,
		DHCPFound:   false,
		DHCPInfo:    "",

		// ARP defaults
		ARPChan: nil,
		ARPLog:  []string{},

		// Routing defaults
		RouteChan: nil,
		RouteLog:  []string{},

		// Firewall defaults
		FirewallChan: nil,
		FirewallLog:  []string{},

		// Open ports defaults
		OpenPortsChan: nil,
		OpenPortsLog:  []string{},

		// Traceroute defaults
		TraceChan:   nil,
		TraceLog:    []string{},
		TraceTarget: "8.8.8.8",

		// Bandwidth defaults
		BandwidthChan: nil,
		BandwidthLog:  []string{},

		// Latency defaults
		LatencyChan: nil,
		LatencyLog:  []string{},

		// Packet loss defaults
		PacketLossChan: nil,
		PacketLossLog:  []string{},

		// VPN defaults
		VPNChan: nil,
		VPNLog:  []string{},

		// Wi-Fi defaults
		WiFiChan: nil,
		WiFiLog:  []string{},

		// Network interfaces defaults
		NetIfChan: nil,
		NetIfLog:  []string{},

		// Proxy defaults
		ProxyChan: nil,
		ProxyLog:  []string{},

		// NAT defaults
		NATChan: nil,
		NATLog:  []string{},

		// QoS defaults
		QoSChan: nil,
		QoSLog:  []string{},
	}
	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
	}
}

type (
	tickMsg  struct{}
	frameMsg struct{}
)

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func frame() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return frameMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tick()
}

type model struct {
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
	PingChan         chan pingResult
	PingLog          []string // collect per-ping results

	// mtu-specific fields
	MTUTargets      []int
	MTUIndex        int
	MTUSuccessCount int
	MTUChan         chan mtuResult
	MTULog          []string // collect per-mtu results

	// dns-specific fields
	DNSTargets      []string
	DNSIndex        int
	DNSSuccessCount int
	DNSChan         chan dnsResult
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
	UpdateFunc func(tea.Msg, model) (tea.Model, tea.Cmd)
	ViewFunc   func(model) string
}

type pingResult struct {
	Index   int
	Success bool
	Done    bool
}

type mtuResult struct {
	Size    int
	Success bool
	Done    bool
}

type dnsResult struct {
	Name    string
	Addrs   []string
	Success bool
	Done    bool
}

func choicesView(m model) string {
	c := m.Choice

	tpl := "What to do today?\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("q, esc: quit") + dotStyle +
		subtleStyle.Render(fmt.Sprintf("v: toggle logging (%s)", map[bool]string{true: "on", false: "off"}[m.Logging]))

	choices := ""
	for idx, choice := range m.AvailableChoices {
		choices += fmt.Sprintf("%s\n", checkbox(choice.Name, idx == c))
	}

	return fmt.Sprintf(tpl, choices)
}

// Main update function.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// global key handling (quit)
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		// toggle logging with "v"
		if k == "v" {
			if m.Logging {
				// disable logging: close file if open
				if loggingFile != nil {
					_ = loggingFile.Close()
					loggingFile = nil
				}
				m.Logging = false
			} else {
				// enable logging -> create/overwrite debug.log
				f, err := tea.LogToFile("debug.log", "debug")
				if err == nil {
					loggingFile = f
					m.Logging = true
				}
				// if err, ignore — UI will continue without logging
			}
			return m, nil
		}

		if k == "q" || k == "esc" || k == "ctrl+c" {
			m.Quitting = true
			// ensure log file closed on quit
			if loggingFile != nil {
				_ = loggingFile.Close()
				loggingFile = nil
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
func (m model) View() string {
	var s string
	if m.Quitting {
		return "\n  See you later!\n\n"
	}
	if !m.Chosen {
		s = choicesView(m)
	} else {
		s = chosenView(m)
	}
	return mainStyle.Render("\n" + s + "\n\n")
}

// Sub-update functions

// Update loop for the first view where you're choosing a task.
func updateChoices(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
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
			return m, frame()
		}
	}

	return m, nil
}

func updateChosen(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	if m.Choice < 0 || m.Choice >= len(m.AvailableChoices) {
		return m, nil
	}
	return m.AvailableChoices[m.Choice].UpdateFunc(msg, m)
}

func chosenView(m model) string {
	if m.Choice < 0 || m.Choice >= len(m.AvailableChoices) {
		return "Invalid choice"
	}
	return m.AvailableChoices[m.Choice].ViewFunc(m)
}
