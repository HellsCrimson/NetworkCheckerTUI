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

// Check bandwidth: try `speedtest` (Ookla) then `speedtest-cli` (python) as fallback.
// Streams output into m.BandwidthLog and marks Loaded when finished.
// Conservative: uses a timeout so it won't hang indefinitely.

func UpdateBandwidth(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.BandwidthChan == nil {
			m.BandwidthChan = make(chan string, 512)
			go func(ch chan<- string) {
				defer close(ch)
				// give the check a reasonable overall timeout
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
					// stream stdout
					outScanner := bufio.NewScanner(stdout)
					go func() {
						for outScanner.Scan() {
							select {
							case ch <- outScanner.Text():
							default:
							}
						}
					}()
					// stream stderr too
					errScanner := bufio.NewScanner(stderr)
					for errScanner.Scan() {
						select {
						case ch <- errScanner.Text():
						default:
						}
					}
					_ = cmd.Wait()
					return true
				}

				// try known backends
				if try("speedtest", "--simple") {
					return
				}
				if try("speedtest-cli", "--simple") {
					return
				}
				if try("librespeed-cli", "--simple") {
					return
				}

				// nothing worked -- emit helpful message
				select {
				case ch <- "no speedtest binary available (tried: speedtest, speedtest-cli, fast) or they require privileges":
				default:
				}
			}(m.BandwidthChan)
			return m, Frame()
		}

		if m.BandwidthChan != nil {
			for {
				select {
				case line, ok := <-m.BandwidthChan:
					if !ok {
						m.BandwidthChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					// keep log, and also try to capture concise summary lines
					m.BandwidthLog = append(m.BandwidthLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenBandwidthView(m Model) string {
	header := utils.KeywordStyle.Render("Bandwidth check:") + " speedtest / speedtest-cli\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("running bandwidth test...")
		if len(m.BandwidthLog) > 0 {
			body = strings.Join(m.BandwidthLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.BandwidthLog) == 0 {
		return header + utils.SubtleStyle.Render("No bandwidth output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// show collected output (try to surface common summary lines at top)
	var summary []string
	for _, l := range m.BandwidthLog {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "download") || strings.Contains(ll, "upload") || strings.Contains(ll, "ping") || strings.Contains(ll, "download:") || strings.Contains(ll, "upload:") || strings.Contains(ll, "bytes/sec") || strings.Contains(ll, "mbit/s") || strings.Contains(ll, "mbps") {
			summary = append(summary, l)
		}
	}
	var body string
	if len(summary) > 0 {
		body = "Summary:\n" + strings.Join(summary, "\n") + "\n\nRaw output:\n" + strings.Join(m.BandwidthLog, "\n")
	} else {
		body = "Raw output:\n" + strings.Join(m.BandwidthLog, "\n")
	}
	return header + utils.SubtleStyle.Render(body) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
