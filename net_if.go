package main

import (
	"bufio"
	"context"
	"fmt"
	"network-check/utils"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check network interfaces: prefer `ip link` / `ip addr`, fallback to `ifconfig -a`.
// Streams output lines into m.NetIfLog and prepends a concise interface summary on completion.

var ipLinkRe = regexp.MustCompile(`^\d+:\s*([^:]+):\s*(?:<([^>]*)>)?.*mtu\s*(\d+)`)
var stateRe = regexp.MustCompile(`state\s+([A-Z]+)`)

func UpdateNetworkInterfaces(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.NetIfChan == nil {
			m.NetIfChan = make(chan string, 512)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				try := func(name string, args ...string) bool {
					cmd := exec.CommandContext(ctx, name, args...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						return false
					}
					stderr, _ := cmd.StderrPipe()
					if err := cmd.Start(); err != nil {
						return false
					}
					sc := bufio.NewScanner(stdout)
					had := false
					for sc.Scan() {
						line := strings.TrimRight(sc.Text(), "\r\n")
						if line == "" {
							continue
						}
						had = true
						select {
						case ch <- line:
						default:
						}
					}
					// capture stderr too in case the tool prints there
					esc := bufio.NewScanner(stderr)
					for esc.Scan() {
						line := strings.TrimRight(esc.Text(), "\r\n")
						if line == "" {
							continue
						}
						had = true
						select {
						case ch <- fmt.Sprintf("%s [err]: %s", name, line):
						default:
						}
					}
					_ = cmd.Wait()
					return had
				}

				// Try ip link and ip addr - both are useful
				if try("ip", "link", "show") {
					time.Sleep(40 * time.Millisecond)
				}
				_ = try("ip", "addr", "show")
				// fallback to ifconfig -a
				_ = try("ifconfig", "-a")

				// If nothing was produced, emit a helpful message
				select {
				case ch <- "no output from ip/ifconfig (binary missing or requires privileges)":
				default:
				}
			}(m.NetIfChan)

			return m, Frame()
		}

		// poll channel
		if m.NetIfChan != nil {
			for {
				select {
				case line, ok := <-m.NetIfChan:
					if !ok {
						// finished -> prepend summary and mark loaded
						if summary := summarizeNetIf(m.NetIfLog); summary != "" {
							m.NetIfLog = append([]string{summary}, m.NetIfLog...)
						}
						m.NetIfChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.NetIfLog = append(m.NetIfLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func summarizeNetIf(lines []string) string {
	type ifinfo struct {
		name  string
		state string
		mtu   string
		flags string
	}
	seen := map[string]*ifinfo{}
	for _, l := range lines {
		if m := ipLinkRe.FindStringSubmatch(l); len(m) >= 4 {
			name := strings.TrimSpace(m[1])
			flags := strings.TrimSpace(m[2])
			mtu := strings.TrimSpace(m[3])
			state := "UNKNOWN"
			if sm := stateRe.FindStringSubmatch(l); len(sm) >= 2 {
				state = sm[1]
			} else if flags != "" && strings.Contains(flags, "UP") {
				state = "UP"
			} else {
				state = "DOWN"
			}
			seen[name] = &ifinfo{name: name, state: state, mtu: mtu, flags: flags}
		}
		// also try to parse ifconfig-style headings: "eth0: flags=.. mtu .."
		if strings.Contains(l, "flags=") && strings.Contains(l, "mtu") {
			// try to extract name before colon
			parts := strings.SplitN(l, ":", 2)
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[0])
				flagsPart := ""
				mtuPart := ""
				if idx := strings.Index(parts[1], "flags="); idx >= 0 {
					flagsPart = parts[1][idx:]
				}
				if idx := strings.Index(parts[1], "mtu"); idx >= 0 {
					rest := parts[1][idx:]
					fields := strings.Fields(rest)
					if len(fields) >= 2 {
						mtuPart = fields[1]
					}
				}
				if name != "" {
					state := "UNKNOWN"
					if strings.Contains(flagsPart, "UP") {
						state = "UP"
					} else {
						state = "DOWN"
					}
					if _, ok := seen[name]; !ok {
						seen[name] = &ifinfo{name: name, state: state, mtu: mtuPart, flags: flagsPart}
					}
				}
			}
		}
	}

	if len(seen) == 0 {
		return ""
	}
	var out []string
	out = append(out, "Interfaces summary:")
	for _, v := range seen {
		flags := ""
		if v.flags != "" {
			flags = fmt.Sprintf(" flags=%s", v.flags)
		}
		mtu := ""
		if v.mtu != "" {
			mtu = fmt.Sprintf(" mtu=%s", v.mtu)
		}
		out = append(out, fmt.Sprintf("- %s: %s%s%s", v.name, v.state, mtu, flags))
	}
	return strings.Join(out, "\n")
}

func ChosenNetworkInterfacesView(m Model) string {
	header := utils.KeywordStyle.Render("Network interfaces:") + " ip link / ip addr / ifconfig\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("querying network interfaces...")
		if len(m.NetIfLog) > 0 {
			body = strings.Join(m.NetIfLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.NetIfLog) == 0 {
		return header + utils.SubtleStyle.Render("No network interface output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.NetIfLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
