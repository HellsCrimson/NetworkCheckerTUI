package main

import (
	"context"
	"fmt"
	"network-check/utils"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// IP routing / ping worker + view (moved out of main.go)

func UpdateIPRouting(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		// start ping worker on first frame for this view
		if !m.Loaded && m.PingChan == nil {
			m.PingChan = make(chan PingResult, m.PingTotal)
			go func(ch chan<- PingResult, ip string, total int) {
				ctx := context.Background()
				for i := 1; i <= total; i++ {
					// run one ping attempt
					cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
					err := cmd.Run()
					ch <- PingResult{Index: i, Success: err == nil, Done: i == total}
				}
				close(ch)
			}(m.PingChan, m.PingIP, m.PingTotal)

			// continue polling frames to read channel
			return m, Frame()
		}

		// poll the ping channel without blocking and update progress
		if m.PingChan != nil {
			for {
				select {
				case r, ok := <-m.PingChan:
					if !ok {
						// channel closed, treat as finished
						m.Loaded = true
						return m, nil
					}
					// record detailed result
					status := "FAIL"
					if r.Success {
						status = "OK"
					}
					m.PingLog = append(m.PingLog, fmt.Sprintf("Ping %d: %s", r.Index, status))
					m.PingCount = r.Index
					if r.Success {
						m.PingSuccessCount++
					}
					if r.Done {
						m.Loaded = true
						return m, nil
					}
				default:
					// nothing to read right now
					return m, Frame()
				}
			}
		}
	case tickMsg:
		// removed automatic quitting — do nothing on ticks
		return m, nil
	}
	return m, nil
}

func ChosenIPRoutingView(m Model) string {
	header := utils.KeywordStyle.Render("Running:") + fmt.Sprintf(" ping %s (%d)\n\n", m.PingIP, m.PingTotal)

	// show progress of pings
	progressLine := fmt.Sprintf("Pings: %d/%d — Success: %d", m.PingCount, m.PingTotal, m.PingSuccessCount)
	output := utils.SubtleStyle.Render(progressLine)

	// when finished, print collected per-ping results instead of exiting
	if m.Loaded {
		if len(m.PingLog) > 0 {
			output = utils.SubtleStyle.Render(strings.Join(m.PingLog, "\n"))
		} else {
			output = utils.SubtleStyle.Render("No ping results collected.")
		}
	}

	label := "Running..."
	if m.Loaded {
		label = "Completed. Press esc to quit or b to go back."
	}
	return header + output + "\n\n" + label
}
