package modules

import (
	"bufio"
	"context"
	"network-check/utils"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Simple ARP table check: runs "ip neigh show" (preferred) or falls back to "arp -n".
// Streams lines into a channel from a goroutine and collects them into m.ARPLog.
func UpdateARP(msg tea.Msg, m utils.Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case utils.FrameMsg:
		if !m.Loaded && m.ARPChan == nil {
			m.ARPChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// try ip neigh first
				cmd := exec.CommandContext(ctx, "ip", "neigh", "show")
				stdout, err := cmd.StdoutPipe()
				if err == nil {
					if err := cmd.Start(); err == nil {
						scanner := bufio.NewScanner(stdout)
						for scanner.Scan() {
							line := strings.TrimSpace(scanner.Text())
							if line != "" {
								select {
								case ch <- line:
								default:
								}
							}
						}
						_ = cmd.Wait()
						return
					}
				}

				// fallback to arp -n
				cmd = exec.CommandContext(ctx, "arp", "-n")
				stdout, err = cmd.StdoutPipe()
				if err == nil {
					if err := cmd.Start(); err == nil {
						scanner := bufio.NewScanner(stdout)
						for scanner.Scan() {
							line := strings.TrimSpace(scanner.Text())
							if line != "" {
								select {
								case ch <- line:
								default:
								}
							}
						}
						_ = cmd.Wait()
						return
					}
				}

				// if both fail, emit a helpful message
				select {
				case ch <- "could not run 'ip neigh' or 'arp -n' (permission or binary missing)":
				default:
				}
			}(m.ARPChan)

			return m, utils.Frame()
		}

		// poll ARP channel
		if m.ARPChan != nil {
			for {
				select {
				case line, ok := <-m.ARPChan:
					if !ok {
						// channel closed -> finished
						m.ARPChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.ARPLog = append(m.ARPLog, trim)
					return m, utils.Frame()
				default:
					return m, utils.Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenARPView(m utils.Model) string {
	header := utils.KeywordStyle.Render("ARP tables:") + " ip neigh / arp -n\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("querying ARP/neighbour table...")
		if len(m.ARPLog) > 0 {
			body = strings.Join(m.ARPLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show collected ARP entries or message
	if len(m.ARPLog) == 0 {
		return header + utils.SubtleStyle.Render("No ARP entries collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.ARPLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
