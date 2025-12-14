package main

import (
	"bufio"
	"context"
	"fmt"
	"network-check/utils"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check NAT configuration: prefer nftables nat table, fall back to iptables nat or iptables-save.
// Also probe ip_forward via sysctl. Streams lines into m.NATChan and collects them into m.NATLog.
func UpdateNAT(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.NATChan == nil {
			m.NATChan = make(chan string, 512)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()

				sendCmd := func(name string, args ...string) bool {
					cmd := exec.CommandContext(ctx, name, args...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						return false
					}
					stderr, _ := cmd.StderrPipe()
					if err := cmd.Start(); err != nil {
						return false
					}
					had := false
					sc := bufio.NewScanner(stdout)
					for sc.Scan() {
						line := strings.TrimRight(sc.Text(), "\r\n")
						if line == "" {
							continue
						}
						had = true
						select {
						case ch <- fmt.Sprintf("%s: %s", name, line):
						default:
						}
					}
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
					// small pause to let UI pick up streamed lines
					time.Sleep(30 * time.Millisecond)
					return had
				}

				// try nft nat table
				if sendCmd("nft", "list", "table", "nat") {
					// prefer nft output; still probe ip_forward
				} else if sendCmd("iptables", "-t", "nat", "-L", "-n", "-v") {
					// got iptables nat output
				} else if sendCmd("iptables-save", "-t", "nat") {
					// fallback iptables-save
				}

				// probe ip_forward sysctl
				_ = sendCmd("sysctl", "-n", "net.ipv4.ip_forward")
				// also check nftables base chains if available
				_ = sendCmd("nft", "list", "ruleset")

				// if nothing produced at all, emit a helpful note
				select {
				case ch <- "no NAT info produced (nft/iptables/sysctl missing or requires privileges)":
				default:
				}
			}(m.NATChan)
			return m, Frame()
		}

		// poll NAT channel
		if m.NATChan != nil {
			for {
				select {
				case line, ok := <-m.NATChan:
					if !ok {
						m.NATChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.NATLog = append(m.NATLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenNATView(m Model) string {
	header := utils.KeywordStyle.Render("NAT configuration:") + " nft/iptables/sysctl\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("probing NAT configuration...")
		if len(m.NATLog) > 0 {
			body = strings.Join(m.NATLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.NATLog) == 0 {
		return header + utils.SubtleStyle.Render("No NAT output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.NATLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
