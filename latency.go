// filepath: /home/matthias/Documents/network-check/latency.go
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

// Check latency: run system `ping` to measure RTTs. Streams lines into m.LatencyLog.
// On completion tries to extract average latency from ping summary and appends a short summary.

var rttRegexp = regexp.MustCompile(`(?i)(?:rtt|round-trip).*= *([\d\.]+)/([\d\.]+)/([\d\.]+)/([\d\.]+) *ms`)

func updateLatency(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		if !m.Loaded && m.LatencyChan == nil {
			m.LatencyChan = make(chan string, 512)
			target := m.PingIP
			if target == "" {
				target = "8.8.8.8"
			}
			go func(ch chan<- string, tgt string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()

				try := func(args ...string) error {
					cmd := exec.CommandContext(ctx, args[0], args[1:]...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						return err
					}
					stderr, _ := cmd.StderrPipe()
					if err := cmd.Start(); err != nil {
						return err
					}
					sc := bufio.NewScanner(stdout)
					for sc.Scan() {
						select {
						case ch <- sc.Text():
						default:
						}
					}
					// also stream stderr
					esc := bufio.NewScanner(stderr)
					for esc.Scan() {
						select {
						case ch <- esc.Text():
						default:
						}
					}
					_ = cmd.Wait()
					return nil
				}

				// try numeric output (-n) and 5 pings (-c 5)
				if err := try("ping", "-c", "5", "-n", tgt); err == nil {
					return
				}
				// fallback without -n
				if err := try("ping", "-c", "5", tgt); err == nil {
					return
				}
				// nothing worked
				select {
				case ch <- "could not run 'ping' (missing or requires privileges)":
				default:
				}
			}(m.LatencyChan, target)

			return m, frame()
		}

		if m.LatencyChan != nil {
			for {
				select {
				case line, ok := <-m.LatencyChan:
					if !ok {
						// finished -> try to extract avg RTT from collected lines
						avg := extractAvgRTT(m.LatencyLog)
						if avg != "" {
							m.LatencyLog = append([]string{fmt.Sprintf("Average RTT: %s ms", avg)}, m.LatencyLog...)
						}
						m.LatencyChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.LatencyLog = append(m.LatencyLog, trim)
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func extractAvgRTT(lines []string) string {
	// Scan lines from end to start for rtt summary
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if m := rttRegexp.FindStringSubmatch(line); len(m) >= 3 {
			// m[2] is avg
			return m[2]
		}
	}
	return ""
}

func chosenLatencyView(m model) string {
	header := keywordStyle.Render("Latency check:") + " ping (5 samples)\n\n"

	if !m.Loaded {
		body := subtleStyle.Render("measuring latency...")
		if len(m.LatencyLog) > 0 {
			body = strings.Join(m.LatencyLog, "\n")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.LatencyLog) == 0 {
		return header + subtleStyle.Render("No latency output collected or command failed.") + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + subtleStyle.Render(strings.Join(m.LatencyLog, "\n")) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
