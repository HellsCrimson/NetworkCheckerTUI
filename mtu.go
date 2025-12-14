package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MTU probing worker + view

func updateMTU(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		// start mtu worker on first frame for this view
		if !m.Loaded && m.MTUChan == nil {
			// buffered channel sized to number of targets
			m.MTUChan = make(chan mtuResult, len(m.MTUTargets))
			go func(ch chan<- mtuResult, ip string, targets []int) {
				ctx := context.Background()
				for i, size := range targets {
					// compute payload size: common ping header overhead is ~28 bytes
					payload := size - 28
					if payload < 0 {
						payload = 0
					}
					// Use Don't Fragment (-M do) so a failure indicates MTU/path issue
					args := []string{"-c", "1", "-M", "do", "-s", strconv.Itoa(payload), "-W", "1", ip}
					cmd := exec.CommandContext(ctx, "ping", args...)
					err := cmd.Run()
					ch <- mtuResult{Size: size, Success: err == nil, Done: i == len(targets)-1}
					// small pause so UI updates smoothly
					time.Sleep(150 * time.Millisecond)
				}
				close(ch)
			}(m.MTUChan, m.PingIP, m.MTUTargets)

			return m, frame()
		}

		// poll the mtu channel without blocking and update progress
		if m.MTUChan != nil {
			for {
				select {
				case r, ok := <-m.MTUChan:
					if !ok {
						// channel closed, treat as finished
						m.Loaded = true
						return m, nil
					}
					// record progress and detailed result
					m.MTUIndex++
					status := "FAIL"
					if r.Success {
						status = "OK"
					}
					m.MTULog = append(m.MTULog, fmt.Sprintf("MTU %d: %s", r.Size, status))
					if r.Success {
						m.MTUSuccessCount++
					}
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
		// removed automatic quitting — do nothing on ticks
		return m, nil
	}
	return m, nil
}

func chosenMTUView(m model) string {
	header := keywordStyle.Render("MTU check:") + " mtuprobe\n\n"

	total := len(m.MTUTargets)
	progressLine := fmt.Sprintf("Tested: %d/%d — Successes: %d", m.MTUIndex, total, m.MTUSuccessCount)

	var body string
	if !m.Loaded {
		// show the next target being tested if any remain
		nextIdx := m.MTUIndex
		if nextIdx < total {
			body = fmt.Sprintf("%s • Testing size: %d bytes", progressLine, m.MTUTargets[nextIdx])
		} else {
			body = fmt.Sprintf("%s • Finishing...", progressLine)
		}
	} else {
		// show full MTU log when finished
		if len(m.MTULog) > 0 {
			body = strings.Join(m.MTULog, "\n")
		} else {
			body = "No MTU results collected."
		}
	}

	output := subtleStyle.Render(body)

	label := "Running..."
	if m.Loaded {
		label = "Completed. Press esc to quit or b to go back."
	}
	return header + output + "\n\n" + label
}
