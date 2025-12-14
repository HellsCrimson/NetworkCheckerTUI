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
		AvailableChoices: []string{
			"Full network check",
			"Check IP",
			"Check DNS",
			"Check MTU",
			"Frame analyzer",
			"Check DHCP",
			"Check ARP tables",
			"Check routing tables",
			"Check firewall rules",
			"Check open ports",
			"Check traceroute",
			"Check bandwidth",
			"Check latency",
			"Check packet loss",
			"Check VPN status",
			"Check Wi-Fi signal",
			"Check network interfaces",
			"Check proxy settings",
			"Check NAT configuration",
			"Check QoS settings",
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
	AvailableChoices []string
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
		choices += fmt.Sprintf("%s\n", checkbox(choice, idx == c))
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

// Update loop for the second view after a choice has been made
func updateChosen(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	// Route to a choice-specific update handler so each choice can have
	// completely different logic and lifecycle.
	switch m.Choice {
	case 0:
		return updateFullNetwork(msg, m)
	case 1:
		return updateIPRouting(msg, m)
	case 2:
		return updateDNS(msg, m)
	case 3:
		return updateMTU(msg, m)
	case 4:
		return updateFrameAnalyzer(msg, m)
	case 5:
		return updateDHCP(msg, m)
	case 6:
		return updateARP(msg, m)
	case 7:
		return updateRouting(msg, m)
	case 8:
		return updateFirewall(msg, m)
	case 9:
		return updateOpenPorts(msg, m)
	case 10:
		return updateTraceroute(msg, m)
	case 11:
		return updateBandwidth(msg, m)
	case 12:
		return updateLatency(msg, m)
	case 13:
		return updatePacketLoss(msg, m)
	case 14:
		return updateVPN(msg, m)
	case 15:
		return updateWiFi(msg, m)
	case 16:
		return updateNetworkInterfaces(msg, m)
	case 17:
		return updateProxy(msg, m)
	case 18:
		return updateNAT(msg, m)
	case 19:
		return updateQoS(msg, m)
	default:
		return m, nil
	}
}

func chosenView(m model) string {
	switch m.Choice {
	case 0:
		return chosenFullNetworkView(m)
	case 1:
		return chosenIPRoutingView(m)
	case 2:
		return chosenDNSView(m)
	case 3:
		return chosenMTUView(m)
	case 4:
		return chosenFrameAnalyzerView(m)
	case 5:
		return chosenDHCPView(m)
	case 6:
		return chosenARPView(m)
	case 7:
		return chosenRoutingView(m)
	case 8:
		return chosenFirewallView(m)
	case 9:
		return chosenOpenPortsView(m)
	case 10:
		return chosenTracerouteView(m)
	case 11:
		return chosenBandwidthView(m)
	case 12:
		return chosenLatencyView(m)
	case 13:
		return chosenPacketLossView(m)
	case 14:
		return chosenVPNView(m)
	case 15:
		return chosenWiFiView(m)
	case 16:
		return chosenNetworkInterfacesView(m)
	case 17:
		return chosenProxyView(m)
	case 18:
		return chosenNATView(m)
	case 19:
		return chosenQoSView(m)
	default:
		return "Unknown choice"
	}
}
