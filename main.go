package main

// An example demonstrating an application with multiple views.
//
// Note that this example was produced before the Bubbles progress component
// was available (github.com/charmbracelet/bubbles/progress) and thus, we're
// implementing a progress bar from scratch here.

import (
	"fmt"

	"network-check/modules"
	"network-check/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// initialize with field names so struct changes are safe
	initialModel := utils.Model{
		AvailableChoices: []utils.AvailableChoice{
			{
				Name:       "Full network check",
				UpdateFunc: modules.UpdateFullNetwork,
				ViewFunc:   modules.ChosenFullNetworkView,
			},
			{
				Name:       "Check IP",
				UpdateFunc: modules.UpdateIPRouting,
				ViewFunc:   modules.ChosenIPRoutingView,
			},
			{
				Name:       "Check DNS",
				UpdateFunc: modules.UpdateDNS,
				ViewFunc:   modules.ChosenDNSView,
			},
			{
				Name:       "Check MTU",
				UpdateFunc: modules.UpdateMTU,
				ViewFunc:   modules.ChosenMTUView,
			},
			{
				Name:       "Frame analyzer",
				UpdateFunc: modules.UpdateFrameAnalyzer,
				ViewFunc:   modules.ChosenFrameAnalyzerView,
			},
			{
				Name:       "Check DHCP",
				UpdateFunc: modules.UpdateDHCP,
				ViewFunc:   modules.ChosenDHCPView,
			},
			{
				Name:       "Check ARP tables",
				UpdateFunc: modules.UpdateARP,
				ViewFunc:   modules.ChosenARPView,
			},
			{
				Name:       "Check routing tables",
				UpdateFunc: modules.UpdateRouting,
				ViewFunc:   modules.ChosenRoutingView,
			},
			{
				Name:       "Check firewall rules",
				UpdateFunc: modules.UpdateFirewall,
				ViewFunc:   modules.ChosenFirewallView,
			},
			{
				Name:       "Check open ports",
				UpdateFunc: modules.UpdateOpenPorts,
				ViewFunc:   modules.ChosenOpenPortsView,
			},
			{
				Name:       "Check traceroute",
				UpdateFunc: modules.UpdateTraceroute,
				ViewFunc:   modules.ChosenTracerouteView,
			},
			{
				Name:       "Check bandwidth",
				UpdateFunc: modules.UpdateBandwidth,
				ViewFunc:   modules.ChosenBandwidthView,
			},
			{
				Name:       "Check latency",
				UpdateFunc: modules.UpdateLatency,
				ViewFunc:   modules.ChosenLatencyView,
			},
			{
				Name:       "Check packet loss",
				UpdateFunc: modules.UpdatePacketLoss,
				ViewFunc:   modules.ChosenPacketLossView,
			},
			{
				Name:       "Check VPN status",
				UpdateFunc: modules.UpdateVPN,
				ViewFunc:   modules.ChosenVPNView,
			},
			{
				Name:       "Check Wi-Fi signal",
				UpdateFunc: modules.UpdateWiFi,
				ViewFunc:   modules.ChosenWiFiView,
			},
			{
				Name:       "Check network interfaces",
				UpdateFunc: modules.UpdateNetworkInterfaces,
				ViewFunc:   modules.ChosenNetworkInterfacesView,
			},
			{
				Name:       "Check proxy settings",
				UpdateFunc: modules.UpdateProxy,
				ViewFunc:   modules.ChosenProxyView,
			},
			{
				Name:       "Check NAT configuration",
				UpdateFunc: modules.UpdateNAT,
				ViewFunc:   modules.ChosenNATView,
			},
			{
				Name:       "Check QoS settings",
				UpdateFunc: modules.UpdateQoS,
				ViewFunc:   modules.ChosenQoSView,
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
