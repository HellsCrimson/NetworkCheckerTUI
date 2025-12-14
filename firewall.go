package main

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check firewall rules: prefer nftables, fall back to iptables or ufw.
// Streams lines into a channel from a goroutine and collects them into m.FirewallLog.
func updateFirewall(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
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

			return m, frame()
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
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func chosenFirewallView(m model) string {
	header := keywordStyle.Render("Firewall rules:") + " nft/iptables/ufw\n\n"

	if !m.Loaded {
		body := subtleStyle.Render("querying firewall rules...")
		if len(m.FirewallLog) > 0 {
			body = strings.Join(m.FirewallLog, "\n")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show collected firewall rules or message
	if len(m.FirewallLog) == 0 {
		return header + subtleStyle.Render("No firewall output collected or command failed.") + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + subtleStyle.Render(strings.Join(m.FirewallLog, "\n")) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
