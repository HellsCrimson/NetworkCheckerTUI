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

// Check QoS settings: probe `tc` for qdiscs/classes/filters and fall back to nft/iptables where sensible.
// Streams output into m.QoSLog and marks Loaded when finished.
func UpdateQoS(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.QoSChan == nil {
			m.QoSChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()

				run := func(name string, args ...string) bool {
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
						case ch <- fmt.Sprintf("%s %s", name, line):
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
						case ch <- fmt.Sprintf("%s [err] %s", name, line):
						default:
						}
					}
					_ = cmd.Wait()
					// small pause so UI can update progressively
					time.Sleep(30 * time.Millisecond)
					return had
				}

				// Prefer tc outputs
				if run("tc", "qdisc", "show", "dev", "all") {
					time.Sleep(20 * time.Millisecond)
				}
				_ = run("tc", "class", "show", "dev", "all")
				_ = run("tc", "filter", "show", "dev", "all")

				// also check for nft/iptables mangle tables which may be used for marking
				_ = run("nft", "list", "table", "inet")
				_ = run("iptables", "-t", "mangle", "-L", "-n", "-v")

				// if nothing produced, emit helpful message
				select {
				case ch <- "no QoS output produced (tc/nft/iptables missing or requires privileges)":
				default:
				}
			}(m.QoSChan)

			return m, Frame()
		}

		// poll QoS channel
		if m.QoSChan != nil {
			for {
				select {
				case line, ok := <-m.QoSChan:
					if !ok {
						m.QoSChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.QoSLog = append(m.QoSLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenQoSView(m Model) string {
	header := utils.KeywordStyle.Render("QoS settings:") + " tc / nft / iptables mangle\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("probing QoS configuration...")
		if len(m.QoSLog) > 0 {
			body = strings.Join(m.QoSLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.QoSLog) == 0 {
		return header + utils.SubtleStyle.Render("No QoS output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.QoSLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
