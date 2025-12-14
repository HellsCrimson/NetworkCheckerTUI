package main

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check packet loss by running ping -c <count> and parsing the summary.
// Streams output into m.PacketLossLog and prepends a short summary line when done.

var pktLossRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)%\s*packet loss`)

func updatePacketLoss(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		// start worker on first frame for this view
		if !m.Loaded && m.PacketLossChan == nil {
			m.PacketLossChan = make(chan string, 512)
			count := 10
			target := m.PingIP
			if target == "" {
				target = "8.8.8.8"
			}
			go func(ch chan<- string, tgt string, cnt int) {
				defer close(ch)
				// overall timeout slightly larger than expected ping duration
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cnt*3)*time.Second)
				defer cancel()

				// prefer numeric output (-n) when available
				cmd := exec.CommandContext(ctx, "ping", "-c", fmt.Sprintf("%d", cnt), "-n", tgt)
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					// fallback without -n
					cmd = exec.CommandContext(ctx, "ping", "-c", fmt.Sprintf("%d", cnt), tgt)
					stdout, _ = cmd.StdoutPipe()
				}
				stderr, _ := cmd.StderrPipe()

				if err := cmd.Start(); err != nil {
					select {
					case ch <- fmt.Sprintf("failed to start ping: %v", err):
					default:
					}
					return
				}

				outScan := bufio.NewScanner(stdout)
				errScan := bufio.NewScanner(stderr)

				done := make(chan struct{})
				go func() {
					for outScan.Scan() {
						line := outScan.Text()
						select {
						case ch <- line:
						default:
						}
					}
					close(done)
				}()
				for errScan.Scan() {
					select {
					case ch <- errScan.Text():
					default:
					}
				}

				// wait for stdout goroutine to finish and command to exit
				<-done
				_ = cmd.Wait()
			}(m.PacketLossChan, target, count)

			return m, frame()
		}

		// poll channel and update model
		if m.PacketLossChan != nil {
			for {
				select {
				case line, ok := <-m.PacketLossChan:
					if !ok {
						// finished -> attempt to extract loss summary
						loss := extractPacketLoss(m.PacketLossLog)
						if loss == "" {
							loss = "unknown"
						}
						// prepend summary
						summary := fmt.Sprintf("Packet loss: %s", loss)
						m.PacketLossLog = append([]string{summary}, m.PacketLossLog...)
						m.PacketLossChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.PacketLossLog = append(m.PacketLossLog, trim)
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func extractPacketLoss(lines []string) string {
	// scan for "<num>% packet loss"
	for i := len(lines) - 1; i >= 0; i-- {
		if m := pktLossRe.FindStringSubmatch(lines[i]); len(m) >= 2 {
			return m[1] + "%"
		}
	}
	// some implementations report "0.0% packet loss" or "100% packet loss" covered above.
	return ""
}

func chosenPacketLossView(m model) string {
	header := keywordStyle.Render("Packet loss check:") + " ping\n\n"

	if !m.Loaded {
		body := subtleStyle.Render("measuring packet loss...")
		if len(m.PacketLossLog) > 0 {
			body = strings.Join(m.PacketLossLog, "\n")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.PacketLossLog) == 0 {
		return header + subtleStyle.Render("No packet loss output collected or command failed.") + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + subtleStyle.Render(strings.Join(m.PacketLossLog, "\n")) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
