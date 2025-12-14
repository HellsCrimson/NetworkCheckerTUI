package main

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check open ports: prefer `ss -lntu` then fall back to `netstat -tuln`.
// Streams lines into a channel from a goroutine and collects them into m.OpenPortsLog.
func updateOpenPorts(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		if !m.Loaded && m.OpenPortsChan == nil {
			m.OpenPortsChan = make(chan string, 512)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				cmds := [][]string{
					{"ss", "-lntu"},      // show listening TCP/UDP numeric
					{"netstat", "-tuln"}, // fallback
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
					first := true
					for scanner.Scan() {
						line := strings.TrimSpace(scanner.Text())
						// skip empty lines; keep header from first command
						if line == "" {
							continue
						}
						// skip the very first header line if it's unhelpful and we already have results
						if first {
							first = false
						}
						select {
						case ch <- line:
						default:
						}
					}
					_ = cmd.Wait()
					// if we got output from this command, don't try other backends
					// (small heuristic: if channel buffered some data)
					// we can't inspect channel length reliably across goroutines in all cases,
					// but assume success if we reached here with scanner output.
					// continue to next stage to allow multiple outputs if desired.
					// brief pause so UI picks up streamed lines
					time.Sleep(50 * time.Millisecond)
				}

				// if nothing written, emit helpful message
				select {
				case ch <- "no output from ss/netstat (missing or permission issue)":
				default:
				}
			}(m.OpenPortsChan)
			return m, frame()
		}

		// poll channel
		if m.OpenPortsChan != nil {
			for {
				select {
				case line, ok := <-m.OpenPortsChan:
					if !ok {
						m.OpenPortsChan = nil
						m.Loaded = true
						return m, nil
					}
					if strings.TrimSpace(line) == "" {
						continue
					}
					m.OpenPortsLog = append(m.OpenPortsLog, line)
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func chosenOpenPortsView(m model) string {
	header := keywordStyle.Render("Open ports:") + " ss -lntu / netstat -tuln\n\n"

	if !m.Loaded {
		body := subtleStyle.Render("scanning listening sockets...")
		if len(m.OpenPortsLog) > 0 {
			body = strings.Join(m.OpenPortsLog, "\n")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show collected open ports or message
	if len(m.OpenPortsLog) == 0 {
		return header + subtleStyle.Render("No listening sockets found or command failed.") + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + subtleStyle.Render(strings.Join(m.OpenPortsLog, "\n")) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
