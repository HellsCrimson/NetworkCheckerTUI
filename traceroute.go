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

// Traceroute check: prefer `traceroute` then fall back to `tracepath`.
// Streams output lines into m.TraceChan and collects them into m.TraceLog.
// Expects model to have fields: TraceChan chan string, TraceLog []string, TraceTarget string.

func UpdateTraceroute(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		// start traceroute worker on first frame for this view
		if !m.Loaded && m.TraceChan == nil {
			m.TraceChan = make(chan string, 512)
			target := m.TraceTarget
			if target == "" {
				target = "8.8.8.8"
			}
			go func(ch chan<- string, tgt string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// try traceroute first
				cmd := exec.CommandContext(ctx, "traceroute", "-n", "-w", "1", "-q", "1", tgt)
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

				// fallback to tracepath (common on some distros)
				cmd = exec.CommandContext(ctx, "tracepath", "-n", tgt)
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
				case ch <- "could not run 'traceroute' or 'tracepath' (missing or requires privileges)":
				default:
				}
			}(m.TraceChan, target)

			return m, Frame()
		}

		// poll trace channel
		if m.TraceChan != nil {
			for {
				select {
				case line, ok := <-m.TraceChan:
					if !ok {
						// finished
						m.TraceChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.TraceLog = append(m.TraceLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenTracerouteView(m Model) string {
	header := utils.KeywordStyle.Render("Traceroute:") + " traceroute / tracepath\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("running traceroute...")
		if len(m.TraceLog) > 0 {
			body = strings.Join(m.TraceLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.TraceLog) == 0 {
		return header + utils.SubtleStyle.Render("No traceroute output collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.TraceLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
