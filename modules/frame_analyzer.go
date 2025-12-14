package modules

import (
	"fmt"
	"net"
	"network-check/utils"
	"regexp"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// A small tui "frame analyzer" that streams lines from tcpdump (if available)
// and displays them in a table. The file exposes a frameModel and wrapper
// functions so it can be used as a choice from main.go (updateFrameAnalyzer / chosenFrameAnalyzerView).

var faHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))

// sanitize helpers
var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func sanitizeString(s string) string {
	// remove ANSI escapes
	s = ansiRegexp.ReplaceAllString(s, "")
	if s == "" {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		// keep printable runes, escape control characters
		switch {
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 32 || r == 0x7f:
			// non-printable -> hex escape
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// frameModel is the component model that owns the table and capture channel.
type frameModel struct {
	table     table.Model
	packetCh  chan string
	rows      []table.Row
	rawRows   []string
	quitting  bool
	startedAt time.Time

	// detail view state
	detailMode bool
	detailRaw  string
	detailRow  table.Row
}

// packetMsg is the message produced for each incoming tcpdump line.
type packetMsg struct {
	row table.Row
	raw string
}

func newFrameAnalyzer() frameModel {
	cols := []table.Column{
		{Title: "Time", Width: 18},
		{Title: "Src", Width: 20},
		{Title: "Dst", Width: 20},
		{Title: "Proto", Width: 8},
		{Title: "Info", Width: 40},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(15),
		table.WithWidth(110),
	)
	t.SetStyles(table.DefaultStyles())
	return frameModel{
		table:    t,
		packetCh: make(chan string, 256),
		rows:     []table.Row{},
	}
}

func (m frameModel) Init() tea.Cmd {
	// start tcpdump (or fallback generator) in background, and start the read loop
	return tea.Batch(startCaptureCmd(m.packetCh), readLoopCmd(m.packetCh))
}

func startCaptureCmd(ch chan<- string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			// Try to open a live capture on the "any" device (works on Linux).
			handle, err := pcap.OpenLive("any", 65535, true, pcap.BlockForever)
			if err == nil {
				defer handle.Close()
				packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
				for packet := range packetSource.Packets() {
					md := packet.Metadata()
					var ts string
					if md != nil && !md.Timestamp.IsZero() {
						// use epoch seconds with microsecond precision so the parser picks up the timestamp
						ts = fmt.Sprintf("%d.%06d", md.Timestamp.Unix(), md.Timestamp.Nanosecond()/1000)
					} else {
						// fallback with high precision time string
						now := time.Now()
						ts = fmt.Sprintf("%d.%06d", now.Unix(), now.Nanosecond()/1000)
					}

					// Extract src/dst/proto/info using gopacket layers when available
					var proto, src, dst, info string

					// Network layer (IPv4 / IPv6)
					if ip4Layer := packet.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
						ip4 := ip4Layer.(*layers.IPv4)
						src = ip4.SrcIP.String()
						dst = ip4.DstIP.String()
						proto = ip4.Protocol.String()
					} else if ip6Layer := packet.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
						ip6 := ip6Layer.(*layers.IPv6)
						src = ip6.SrcIP.String()
						dst = ip6.DstIP.String()
						proto = ip6.NextHeader.String()
					} else if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
						arp := arpLayer.(*layers.ARP)
						src = net.HardwareAddr(arp.SourceHwAddress).String()
						dst = net.HardwareAddr(arp.DstHwAddress).String()
						// also include protocol addresses if present
						if len(arp.SourceProtAddress) >= 4 {
							src = net.IP(arp.SourceProtAddress).String()
						}
						if len(arp.DstProtAddress) >= 4 {
							dst = net.IP(arp.DstProtAddress).String()
						}
						proto = "ARP"
					}

					// Transport layer specifics
					if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
						tcp := tcpLayer.(*layers.TCP)
						// attach ports to src/dst if network addresses were found
						if src != "" && dst != "" {
							src = fmt.Sprintf("%s:%d", src, tcp.SrcPort)
							dst = fmt.Sprintf("%s:%d", dst, tcp.DstPort)
						} else {
							src = tcp.SrcPort.String()
							dst = tcp.DstPort.String()
						}
						proto = "TCP"
						// include brief flags/payload length
						var flags []string
						if tcp.SYN {
							flags = append(flags, "SYN")
						}
						if tcp.ACK {
							flags = append(flags, "ACK")
						}
						if tcp.FIN {
							flags = append(flags, "FIN")
						}
						if tcp.RST {
							flags = append(flags, "RST")
						}
						if tcp.PSH {
							flags = append(flags, "PSH")
						}
						if tcp.URG {
							flags = append(flags, "URG")
						}
						if tcp.ECE {
							flags = append(flags, "ECE")
						}
						if tcp.CWR {
							flags = append(flags, "CWR")
						}
						flagsStr := "-"
						if len(flags) > 0 {
							flagsStr = strings.Join(flags, "|")
						}
						info = fmt.Sprintf("flags=%s len=%d", flagsStr, len(tcp.Payload))
					} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
						udp := udpLayer.(*layers.UDP)
						if src != "" && dst != "" {
							src = fmt.Sprintf("%s:%d", src, udp.SrcPort)
							dst = fmt.Sprintf("%s:%d", dst, udp.DstPort)
						} else {
							src = udp.SrcPort.String()
							dst = udp.DstPort.String()
						}
						proto = "UDP"
						info = fmt.Sprintf("len=%d", len(udp.Payload))
					} else if icmp4 := packet.Layer(layers.LayerTypeICMPv4); icmp4 != nil {
						icmp := icmp4.(*layers.ICMPv4)
						proto = "ICMPv4"
						info = fmt.Sprintf("type=%d code=%d", icmp.TypeCode.Type(), icmp.TypeCode.Code())
					} else if icmp6 := packet.Layer(layers.LayerTypeICMPv6); icmp6 != nil {
						proto = "ICMPv6"
						// keep generic info for ICMPv6
						info = "icmpv6"
					} else if app := packet.ApplicationLayer(); app != nil {
						// application payload: include a short excerpt
						proto = "APP"
						payload := app.Payload()
						if len(payload) > 0 {
							// try to show printable prefix
							pl := string(payload)
							if len(pl) > 200 {
								pl = pl[:200] + "…"
							}
							info = pl
						}
					} else {
						// fallback: compose proto from available layers
						var parts []string
						for _, l := range packet.Layers() {
							parts = append(parts, l.LayerType().String())
						}
						if len(parts) > 0 {
							proto = strings.Join(parts, "/")
						}
					}

					// Info fallback: if not set above, use packet.String() trimmed
					if info == "" {
						info = packet.String()
						if len(info) > 200 {
							info = info[:200] + "…"
						}
					}

					// ensure sane defaults
					if ts == "" {
						ts = time.Now().Format("15:04:05")
					}
					if src == "" {
						src = "-"
					}
					if dst == "" {
						dst = "-"
					}
					if proto == "" {
						proto = "?"
					}

					// sanitize all fields before sending to the UI
					ts = sanitizeString(ts)
					proto = sanitizeString(proto)
					src = sanitizeString(src)
					dst = sanitizeString(dst)
					info = sanitizeString(info)

					line := fmt.Sprintf("%s %s %s > %s: %s", ts, proto, src, dst, info)
					select {
					case ch <- line:
					default:
					}
				}
				// when packet source ends, close channel
				close(ch)
				return
			} else {
				utils.LoggingFile.WriteString(fmt.Sprintf("tcpdump capture failed: %v\n", err))
			}
		}()
		return nil
	}
}

func readLoopCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		// block until a line is available or channel closed
		line, ok := <-ch
		if !ok {
			// channel closed -> no more packets
			return nil
		}
		row := parseTcpdumpLine(line)
		return packetMsg{row: row, raw: line}
	}
}

// UpdateFrameAnalyzer is the component update method. It uses a value receiver
// and returns the updated component as a tea.Model (this lets callers store
// the updated copy).
func (m frameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		k := msg.String()

		// If we're showing the detail view, handle closing it here
		if m.detailMode {
			if k == "b" || k == "esc" || k == "q" {
				m.detailMode = false
				return m, nil
			}
			// allow scrolling the detail text with arrow keys by falling through to table.Update
		}

		if k == "enter" || k == " " {
			// open detail view for the currently selected row
			idx := m.table.Cursor()
			if idx >= 0 && idx < len(m.rows) {
				m.detailMode = true
				m.detailRow = m.rows[idx]
				if idx < len(m.rawRows) {
					m.detailRaw = m.rawRows[idx]
				} else {
					m.detailRaw = ""
				}
				return m, nil
			}
		}

		if k == "q" || k == "esc" || k == "ctrl+c" {
			// quit the analyzer view (caller can switch views)
			m.quitting = true
			return m, tea.Quit
		}
		// forward navigation keys to the table when not in detail mode
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case packetMsg:
		// append new row to table and keep table rows within a limit
		m.rows = append(m.rows, msg.row)
		m.rawRows = append(m.rawRows, msg.raw)
		if len(m.rows) > 1000 {
			// drop oldest rows to keep memory bounded
			m.rows = m.rows[len(m.rows)-1000:]
			m.rawRows = m.rawRows[len(m.rawRows)-1000:]
		}
		m.table.SetRows(m.rows)
		// continue listening for the next line
		return m, readLoopCmd(m.packetCh)

	default:
		// let table update for other messages (mouse, resize, etc.)
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
}

func (m frameModel) View() string {
	if m.quitting {
		return ""
	}
	header := faHeaderStyle.Render("Frame analyzer (press b to go back, Enter to view details, q/esc to quit)") + "\n\n"
	if m.detailMode {
		return header + renderDetailView(m.detailRow, m.detailRaw)
	}
	return header + m.table.View()
}

// renderDetailView renders a simple multiline view showing parsed columns and raw packet.
func renderDetailView(r table.Row, raw string) string {
	var b strings.Builder
	// r expected: [Time, Src, Dst, Proto, Info]
	fmt.Fprintf(&b, "Time : %s\n", r[0])
	fmt.Fprintf(&b, "Src  : %s\n", r[1])
	fmt.Fprintf(&b, "Dst  : %s\n", r[2])
	fmt.Fprintf(&b, "Proto: %s\n\n", r[3])
	fmt.Fprintf(&b, "Info : %s\n\n", r[4])
	if raw != "" {
		fmt.Fprintf(&b, "Raw packet dump:\n%s\n", raw)
	}
	fmt.Fprintf(&b, "\nPress b or esc to return.")
	return utils.SubtleStyle.Render(b.String())
}

// parseTcpdumpLine tries to extract time, src, dst, proto and info from a tcpdump line.
// Parsing is intentionally lenient: unknown parts go into the Info column.
func parseTcpdumpLine(line string) table.Row {
	// typical tcpdump line starts with "1234567890.123456 IP ..." where first token is timestamp
	parts := strings.Fields(line)
	var ts, proto, src, dst, info string

	if len(parts) == 0 {
		return table.Row{"", "", "", "", line}
	}

	// timestamp
	first := parts[0]
	if matched, _ := regexp.MatchString(`^\d+\.\d+`, first); matched {
		ts = first
		// remove timestamp from parsing base
		parts = parts[1:]
	} else {
		ts = time.Now().Format("15:04:05")
	}

	// join remaining line for 'info' fallback
	rem := strings.Join(parts, " ")

	// try to detect "IP" or "IP6" proto token
	if len(parts) > 0 && (parts[0] == "IP" || parts[0] == "IP6") {
		proto = parts[0]
		// capture next token which commonly contains "src > dst:" for IP
		if len(parts) > 1 {
			arrowIdx := -1
			for i, tok := range parts {
				if strings.Contains(tok, ">") {
					arrowIdx = i
					break
				}
			}
			if arrowIdx >= 0 {
				// token looks like "src > dst:" or "src"
				// reconstruct a small window to extract src and dst
				window := strings.Join(parts[arrowIdx-1:minInt(arrowIdx+3, len(parts))], " ")
				// try splitting at '>'
				if strings.Contains(window, ">") {
					seg := strings.SplitN(window, ">", 2)
					src = strings.TrimSpace(seg[0])
					dst = strings.TrimSpace(strings.TrimRight(seg[1], ":"))
				}
			}
		}
	}

	// If we didn't manage to parse src/dst/proto more precisely, apply regex fallback
	if src == "" || dst == "" {
		// look for patterns like "A.B.C.D.port > E.F.G.H.port:"
		re := regexp.MustCompile(`([0-9a-fA-F\:\.]+(?:\.\d+)?)\s*>\s*([0-9a-fA-F\:\.]+(?:\.\d+)?)`)
		if m := re.FindStringSubmatch(rem); len(m) >= 3 {
			src = m[1]
			dst = strings.TrimRight(m[2], ":")
		}
	}

	// proto heuristic: look for "Flags", "ICMP", "UDP", "TCP"
	if proto == "" {
		proto = getProtoFromInfo(rem)
	}

	// Info: keep a short excerpt
	info = rem
	if len(info) > 200 {
		info = info[:200] + "…"
	}

	// ensure columns are sane
	if ts == "" {
		ts = time.Now().Format("15:04:05")
	}
	if src == "" {
		src = "-"
	}
	if dst == "" {
		dst = "-"
	}
	if proto == "" {
		proto = "?"
	}

	return table.Row{ts, src, dst, proto, info}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// faInstance holds the running analyzer component between Update calls.
var faInstance *frameModel

// updateFrameAnalyzer forwards messages to the frame analyzer component and
// returns an updated top-level model. If the user presses 'b' while the
// analyzer is active, we stop the analyzer and return to the choices view.
func UpdateFrameAnalyzer(msg tea.Msg, m utils.Model) (tea.Model, tea.Cmd) {
	// handle key to go back immediately here (top-level handles 'b' only when Loaded)
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "b" && m.Chosen {
			// If the analyzer component exists and is currently showing the detail
			// view, let the component handle 'b' (it will close the detail view).
			// Only when not in detailMode should 'b' return to the choices view.
			if faInstance != nil && faInstance.detailMode {
				// forward to component (do nothing here)
			} else {
				// stop analyzer and return to choices
				faInstance = nil
				m.Chosen = false
				m.Loaded = false
				return m, nil
			}
		}
	}

	// bootstrap analyzer on first frame
	if faInstance == nil {
		fa := newFrameAnalyzer()
		fa.startedAt = time.Now()
		faInstance = &fa
		// return the init command to start capture + read loop
		return m, faInstance.Init()
	}

	// forward message to component; Update returns updated component as tea.Model
	retModel, cmd := faInstance.Update(msg)
	if updated, ok := retModel.(frameModel); ok {
		// store updated copy back into the pointer
		faInstance = &updated
	}
	// do not mark m.Loaded true — analyzer is a live view; user uses 'b' to go back
	return m, cmd
}

// chosenFrameAnalyzerView renders the analyzer component (or a starting message).
func ChosenFrameAnalyzerView(m utils.Model) string {
	header := faHeaderStyle.Render("Frame analyzer") + "\n\n"
	if faInstance == nil {
		return header + utils.SubtleStyle.Render("Starting frame analyzer...")
	}
	return header + faInstance.View()
}

// getProtoFromInfo tries to heuristically determine the protocol from the info string.
func getProtoFromInfo(info string) string {
	info = strings.ToUpper(info)
	switch {
	case strings.Contains(info, "TCP"):
		return "TCP"
	case strings.Contains(info, "UDP"):
		return "UDP"
	case strings.Contains(info, "ICMP"):
		return "ICMP"
	case strings.Contains(info, "ARP"):
		return "ARP"
	case strings.Contains(info, "FLAGS"):
		return "TCP"
	default:
		utils.LoggingFile.WriteString(fmt.Sprintf("Could not determine protocol from info: %s\n", info))
		return "?"
	}
}
