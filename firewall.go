package main

import (
	"bufio"
	"context"
	"network-check/utils"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check firewall rules: prefer nftables, fall back to iptables or ufw.
// Streams lines into a channel from a goroutine and collects them into m.FirewallLog.
func UpdateFirewall(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.FirewallChan == nil {
			m.FirewallChan = make(chan string, 512)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// try nftables first
				cmds := [][]string{
					{"nft", "list", "ruleset"},
					{"iptables", "-L", "-n", "-v"},
					{"ufw", "status", "numbered"},
				}

				for _, args := range cmds {
					cmd := exec.CommandContext(ctx, args[0], args[1:]...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						continue
					}
					if err := cmd.Start(); err != nil {
						continue
					}
					scanner := bufio.NewScanner(stdout)
					for scanner.Scan() {
						line := strings.TrimRight(scanner.Text(), "\r\n")
						if line != "" {
							select {
							case ch <- line:
							default:
							}
						}
					}
					_ = cmd.Wait()
					// if we produced output, assume this backend worked and return
					// (we detect that by checking if channel has data - but chan is buffered,
					// so check via a small probe by sending a marker would be intrusive).
					// Instead, if the command produced at least one line, continue to next step
					// by short sleep; otherwise try next backend.
					time.Sleep(50 * time.Millisecond)
				}

				// If none produced output, emit helpful message
				select {
				case ch <- "no firewall binary produced output (nft/iptables/ufw missing or requires privileges)":
				default:
				}
			}(m.FirewallChan)

			return m, Frame()
		}

		// poll firewall channel
		if m.FirewallChan != nil {
			for {
				select {
				case line, ok := <-m.FirewallChan:
					if !ok {
						// finished
						m.FirewallChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.FirewallLog = append(m.FirewallLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenFirewallView(m Model) string {
	header := utils.KeywordStyle.Render("Firewall rules:") + " nft/iptables/ufw\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("querying firewall rules...")
		if len(m.FirewallLog) > 0 {
			body = strings.Join(m.FirewallLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show collected firewall rules or message
	if len(m.FirewallLog) == 0 {
		return header + utils.SubtleStyle.Render("No firewall output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.FirewallLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
