package main

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Simple DHCP check using the system dhclient binary.
// This implementation starts dhclient (one-shot), captures its output and
// displays any offer/ack information. It is conservative: if dhclient is not
// available or requires privileges, the output/error is shown so user can
// interpret. Run with appropriate privileges for real DHCP exchange.
//
// The UI follows the same pattern as ip/mtu/dns: start worker on first frame,
// stream lines into DHCPLog, and mark Loaded when done.

func updateDHCP(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		// start worker on first frame for this view
		if !m.Loaded && m.DHCPChan == nil {
			m.DHCPChan = make(chan string, 256)
			go func(ch chan<- string, timeout int) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
				defer cancel()

				// Try dhclient (common on many Linux distros). Use -1 (one shot) and verbose.
				// If dhclient is not present or requires privileged execution, command will fail;
				// we still capture output to show to the user.
				cmd := exec.CommandContext(ctx, "dhclient", "-1", "-v")
				stdout, _ := cmd.StdoutPipe()
				stderr, _ := cmd.StderrPipe()

				if err := cmd.Start(); err != nil {
					// Could not start dhclient; return error line and finish.
					select {
					case ch <- fmt.Sprintf("failed to start dhclient: %v", err):
					default:
					}
					close(ch)
					return
				}

				// read both stdout and stderr concurrently
				scannerOut := bufio.NewScanner(stdout)
				scannerErr := bufio.NewScanner(stderr)
				outDone := make(chan struct{})
				go func() {
					for scannerOut.Scan() {
						select {
						case ch <- scannerOut.Text():
						default:
						}
					}
					close(outDone)
				}()
				go func() {
					for scannerErr.Scan() {
						select {
						case ch <- scannerErr.Text():
						default:
						}
					}
				}()

				// wait for process to finish or context timeout
				_ = cmd.Wait()
				// ensure any remaining out lines are processed
				<-outDone

				close(ch)
			}(m.DHCPChan, m.DHCPTimeout)

			return m, frame()
		}

		// poll channel and update model
		if m.DHCPChan != nil {
			for {
				select {
				case line, ok := <-m.DHCPChan:
					if !ok {
						// finished
						m.DHCPChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					// store line
					m.DHCPLog = append(m.DHCPLog, trim)

					// try to pick up useful info heuristically
					// look for "DHCPOFFER from", "DHCPACK from", "bound to <ip>", "lease of <ip>"
					lower := strings.ToLower(trim)
					switch {
					case strings.Contains(lower, "dhcpoffer"):
						m.DHCPFound = true
						m.DHCPInfo = trim
					case strings.Contains(lower, "dhcpack"):
						m.DHCPFound = true
						m.DHCPInfo = trim
					case strings.Contains(lower, "bound to"):
						m.DHCPFound = true
						m.DHCPInfo = trim
					case strings.Contains(lower, "lease of"):
						m.DHCPFound = true
						m.DHCPInfo = trim
					}
					// keep streaming until closed
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func chosenDHCPView(m model) string {
	header := keywordStyle.Render("DHCP check:") + " dhclient - one-shot\n\n"

	if !m.Loaded {
		// running
		body := fmt.Sprintf("Running DHCP discovery (timeout: %ds)...\n\n", m.DHCPTimeout)
		if len(m.DHCPLog) > 0 {
			body += strings.Join(m.DHCPLog, "\n")
		} else {
			body += subtleStyle.Render("waiting for dhclient output...")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show logs and summary
	var summary string
	if m.DHCPFound {
		summary = fmt.Sprintf("DHCP response detected: %s\n\n", m.DHCPInfo)
	} else {
		summary = "No DHCP offer/ack detected.\n\n"
	}
	logs := "Raw output:\n"
	if len(m.DHCPLog) > 0 {
		logs += strings.Join(m.DHCPLog, "\n")
	} else {
		logs += "no output captured"
	}

	body := summary + logs
	return header + subtleStyle.Render(body) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
