package modules

import (
	"bufio"
	"context"
	"fmt"
	"network-check/utils"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check Wi-Fi signal: try nmcli (preferred), then iw, then iwconfig.
// Streams lines into m.WiFiLog and, on completion, prepends a concise best-network summary.
func UpdateWiFi(msg tea.Msg, m utils.Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case utils.FrameMsg:
		if !m.Loaded && m.WiFiChan == nil {
			m.WiFiChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
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
						line := strings.TrimSpace(sc.Text())
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
						line := strings.TrimSpace(esc.Text())
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

				// nmcli: terse SSID:SIGNAL lines are easiest to parse
				if try("nmcli", "-t", "-f", "SSID,SIGNAL", "dev", "wifi") {
					time.Sleep(50 * time.Millisecond)
					return
				}
				// iw: link information per interface
				if try("iw", "dev") {
					time.Sleep(50 * time.Millisecond)
					// try reading current link for wlan interfaces (more likely to show signal)
					_ = try("iw", "dev", "wlan0", "link")
					_ = try("iw", "dev", "wlp2s0", "link")
					return
				}
				// iwconfig fallback
				if try("iwconfig") {
					return
				}

				// nothing produced
				select {
				case ch <- "no wifi binaries produced output (nmcli/iw/iwconfig missing or no wifi device)":
				default:
				}
			}(m.WiFiChan)
			return m, utils.Frame()
		}

		if m.WiFiChan != nil {
			for {
				select {
				case line, ok := <-m.WiFiChan:
					if !ok {
						// finished -> try to extract best signal summary
						summary := summarizeWiFi(m.WiFiLog)
						if summary != "" {
							m.WiFiLog = append([]string{summary}, m.WiFiLog...)
						}
						m.WiFiChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.WiFiLog = append(m.WiFiLog, trim)
					return m, utils.Frame()
				default:
					return m, utils.Frame()
				}
			}
		}
	}
	return m, nil
}

var nmcliLineRe = regexp.MustCompile(`^([^:]*):(\d{1,3})$`)
var iwSignalRe = regexp.MustCompile(`signal: *(-?\d+)\s*dBm`)
var iwconfigSignalRe = regexp.MustCompile(`Signal level[=|:]\s*(-?\d+)`)

func summarizeWiFi(lines []string) string {
	type entry struct {
		name  string
		score int
	}
	var best *entry
	for _, l := range lines {
		// try nmcli style "nmcli:SSID:SIGNAL" because we prefix with command name earlier
		if strings.HasPrefix(l, "nmcli:") {
			raw := strings.TrimPrefix(l, "nmcli:")
			if m := nmcliLineRe.FindStringSubmatch(raw); len(m) == 3 {
				ssid := strings.TrimSpace(m[1])
				sig, _ := strconv.Atoi(m[2])
				if best == nil || sig > best.score {
					best = &entry{name: ssid, score: sig}
				}
				continue
			}
			// nmcli sometimes produces lines with other separators, try split
			parts := strings.Split(raw, ":")
			if len(parts) >= 2 {
				last := strings.TrimSpace(parts[len(parts)-1])
				if v, err := strconv.Atoi(last); err == nil {
					ssid := strings.Join(parts[:len(parts)-1], ":")
					if best == nil || v > best.score {
						best = &entry{name: strings.TrimSpace(ssid), score: v}
					}
				}
			}
		}

		// iw outputs "signal: -60 dBm"
		if m := iwSignalRe.FindStringSubmatch(l); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				// convert dBm to a relative positive score (higher is better)
				score := v
				if best == nil || score > best.score {
					best = &entry{name: "interface", score: score}
				}
				continue
			}
		}
		// iwconfig "Signal level=-60 dBm" or "Signal level=60/70"
		if m := iwconfigSignalRe.FindStringSubmatch(l); len(m) == 2 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				score := v
				if best == nil || score > best.score {
					best = &entry{name: "interface", score: score}
				}
				continue
			}
		}
	}
	if best == nil {
		return ""
	}
	// If score is negative (dBm) show as dBm, else assume percentage
	if best.score < 0 {
		return fmt.Sprintf("Best Wi‑Fi: %s (%d dBm)", best.name, best.score)
	}
	return fmt.Sprintf("Best Wi‑Fi: %s (%d%%)", best.name, best.score)
}

func ChosenWiFiView(m utils.Model) string {
	header := utils.KeywordStyle.Render("Wi‑Fi signal:") + " nmcli/iw/iwconfig\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("probing Wi‑Fi signal...")
		if len(m.WiFiLog) > 0 {
			body = strings.Join(m.WiFiLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.WiFiLog) == 0 {
		return header + utils.SubtleStyle.Render("No Wi‑Fi output collected or no wireless device found.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.WiFiLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
