package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DNS worker + view (moved out of main.go)

func updateDNS(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		// start dns worker on first frame for this view
		if !m.Loaded && m.DNSChan == nil {
			m.DNSChan = make(chan dnsResult, len(m.DNSTargets))
			go func(ch chan<- dnsResult, targets []string) {
				for i, name := range targets {
					addrs, err := net.LookupHost(name)
					success := err == nil
					ch <- dnsResult{Name: name, Addrs: addrs, Success: success, Done: i == len(targets)-1}
					// small pause so UI updates smoothly
					time.Sleep(150 * time.Millisecond)
				}
				close(ch)
			}(m.DNSChan, m.DNSTargets)

			return m, frame()
		}

		// poll the dns channel without blocking and update progress
		if m.DNSChan != nil {
			for {
				select {
				case r, ok := <-m.DNSChan:
					if !ok {
						// channel closed, treat as finished
						m.Loaded = true
						return m, nil
					}
					// record progress and detailed result
					m.DNSIndex++
					status := "FAIL"
					if r.Success {
						status = "OK"
						m.DNSSuccessCount++
					}
					addrs := "no addresses"
					if len(r.Addrs) > 0 {
						addrs = strings.Join(r.Addrs, ", ")
					}
					m.DNSLog = append(m.DNSLog, fmt.Sprintf("%s: %s (%s)", r.Name, status, addrs))
					// when Done, mark loaded and stop (do NOT quit)
					if r.Done {
						m.Loaded = true
						return m, nil
					}
				default:
					// nothing to read right now
					return m, frame()
				}
			}
		}
	case tickMsg:
		// do nothing on ticks
		return m, nil
	}
	return m, nil
}

func chosenDNSView(m model) string {
	header := keywordStyle.Render("DNS check:") + " dnsutils (resolve)\n\n"

	total := len(m.DNSTargets)
	progressLine := fmt.Sprintf("Tested: %d/%d — Successes: %d", m.DNSIndex, total, m.DNSSuccessCount)

	var body string
	if !m.Loaded {
		// show the next target being tested if any remain
		nextIdx := m.DNSIndex
		if nextIdx < total {
			body = fmt.Sprintf("%s • Resolving: %s", progressLine, m.DNSTargets[nextIdx])
		} else {
			body = fmt.Sprintf("%s • Finishing...", progressLine)
		}
	} else {
		// show full DNS log when finished
		if len(m.DNSLog) > 0 {
			body = strings.Join(m.DNSLog, "\n")
		} else {
			body = "No DNS results collected."
		}
	}

	output := subtleStyle.Render(body)

	label := "Running..."
	if m.Loaded {
		label = "Completed. Press esc to quit or b to go back."
	}
	return header + output + "\n\n" + label
}
