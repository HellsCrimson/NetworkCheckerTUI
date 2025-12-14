package main

import (
	"context"
	"fmt"
	"net"
	"network-check/utils"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Full network check: orchestrate ip -> mtu -> dns sequentially and show progress ---
func UpdateFullNetwork(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	const (
		stageNotStarted = 0
		stageIP         = 1
		stageMTU        = 2
		stageDNS        = 3
		stageDone       = 4
	)

	switch msg.(type) {
	case FrameMsg:
		// bootstrap the full run on first frame
		if m.FullStage == stageNotStarted {
			// reset logs/counters
			m.PingLog = []string{}
			m.MTULog = []string{}
			m.DNSLog = []string{}
			m.PingCount = 0
			m.PingSuccessCount = 0
			m.MTUIndex = 0
			m.MTUSuccessCount = 0
			m.DNSIndex = 0
			m.DNSSuccessCount = 0

			// compute total number of individual checks for the progress bar
			m.FullTotal = m.PingTotal + len(m.MTUTargets) + len(m.DNSTargets)
			m.FullCompleted = 0

			// start with IP pings
			m.FullStage = stageIP
			m.PingChan = make(chan PingResult, m.PingTotal)
			go func(ch chan<- PingResult, ip string, total int) {
				ctx := context.Background()
				for i := 1; i <= total; i++ {
					cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
					err := cmd.Run()
					ch <- PingResult{Index: i, Success: err == nil, Done: i == total}
				}
				close(ch)
			}(m.PingChan, m.PingIP, m.PingTotal)

			return m, Frame()
		}

		// handle IP stage
		if m.FullStage == stageIP && m.PingChan != nil {
			for {
				select {
				case r, ok := <-m.PingChan:
					if !ok {
						// channel closed unexpectedly — advance to MTU
						m.PingChan = nil
						m.FullStage = stageMTU
						// start MTU worker below in the outer logic
						break
					}
					status := "FAIL"
					if r.Success {
						status = "OK"
						m.PingSuccessCount++
					}
					m.PingLog = append(m.PingLog, fmt.Sprintf("Ping %d: %s", r.Index, status))
					m.PingCount = r.Index
					m.FullCompleted++
					m.Progress = float64(m.FullCompleted) / float64(m.FullTotal)
					if r.Done {
						// finished ip stage: prepare to start MTU
						m.PingChan = nil
						m.FullStage = stageMTU
						break
					}
				default:
					// nothing to read right now
					return m, Frame()
				}
				// loop back to potentially start next stage
				if m.FullStage != stageIP {
					break
				}
			}
		}

		// start MTU stage when requested
		if m.FullStage == stageMTU && m.MTUChan == nil {
			m.MTUChan = make(chan MtuResult, len(m.MTUTargets))
			go func(ch chan<- MtuResult, ip string, targets []int) {
				ctx := context.Background()
				for i, size := range targets {
					payload := size - 28
					if payload < 0 {
						payload = 0
					}
					args := []string{"-c", "1", "-M", "do", "-s", strconv.Itoa(payload), "-W", "1", ip}
					cmd := exec.CommandContext(ctx, "ping", args...)
					err := cmd.Run()
					ch <- MtuResult{Size: size, Success: err == nil, Done: i == len(targets)-1}
					time.Sleep(150 * time.Millisecond)
				}
				close(ch)
			}(m.MTUChan, m.PingIP, m.MTUTargets)

			return m, Frame()
		}

		// handle MTU stage
		if m.FullStage == stageMTU && m.MTUChan != nil {
			for {
				select {
				case r, ok := <-m.MTUChan:
					if !ok {
						m.MTUChan = nil
						m.FullStage = stageDNS
						break
					}
					m.MTUIndex++
					status := "FAIL"
					if r.Success {
						status = "OK"
						m.MTUSuccessCount++
					}
					m.MTULog = append(m.MTULog, fmt.Sprintf("MTU %d: %s", r.Size, status))
					m.FullCompleted++
					m.Progress = float64(m.FullCompleted) / float64(m.FullTotal)
					if r.Done {
						m.MTUChan = nil
						m.FullStage = stageDNS
						break
					}
				default:
					return m, Frame()
				}
				if m.FullStage != stageMTU {
					break
				}
			}
		}

		// start DNS stage when requested
		if m.FullStage == stageDNS && m.DNSChan == nil {
			m.DNSChan = make(chan DnsResult, len(m.DNSTargets))
			go func(ch chan<- DnsResult, targets []string) {
				for i, name := range targets {
					addrs, err := net.LookupHost(name)
					success := err == nil
					ch <- DnsResult{Name: name, Addrs: addrs, Success: success, Done: i == len(targets)-1}
					time.Sleep(150 * time.Millisecond)
				}
				close(ch)
			}(m.DNSChan, m.DNSTargets)

			return m, Frame()
		}

		// handle DNS stage
		if m.FullStage == stageDNS && m.DNSChan != nil {
			for {
				select {
				case r, ok := <-m.DNSChan:
					if !ok {
						m.DNSChan = nil
						m.FullStage = stageDone
						break
					}
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
					m.FullCompleted++
					m.Progress = float64(m.FullCompleted) / float64(m.FullTotal)
					if r.Done {
						m.DNSChan = nil
						m.FullStage = stageDone
						break
					}
				default:
					return m, Frame()
				}
				if m.FullStage != stageDNS {
					break
				}
			}
		}

		// finished all stages
		if m.FullStage == stageDone {
			m.Loaded = true
			m.Progress = 1
			// do not quit; user can press 'b' to go back
			return m, nil
		}

		return m, Frame()

	case tickMsg:
		// nothing to do on ticks for full flow
		return m, nil
	}

	return m, nil
}

func ChosenFullNetworkView(m Model) string {
	header := utils.KeywordStyle.Render("Full network check") + "\n\n"

	// status line depends on stage
	stageText := "Starting..."
	switch m.FullStage {
	case 1:
		stageText = fmt.Sprintf("Running IP routing tests (%d pings)...", m.PingTotal)
	case 2:
		stageText = fmt.Sprintf("Running MTU tests (%d sizes)...", len(m.MTUTargets))
	case 3:
		stageText = fmt.Sprintf("Running DNS tests (%d names)...", len(m.DNSTargets))
	case 4:
		stageText = "Completed all tests."
	}

	// show progress bar
	bar := utils.Progressbar(m.Progress)

	// when finished, show aggregated results
	var results string
	if m.Loaded {
		var parts []string
		if len(m.PingLog) > 0 {
			parts = append(parts, "IP results:\n"+strings.Join(m.PingLog, "\n"))
		}
		if len(m.MTULog) > 0 {
			parts = append(parts, "MTU results:\n"+strings.Join(m.MTULog, "\n"))
		}
		if len(m.DNSLog) > 0 {
			parts = append(parts, "DNS results:\n"+strings.Join(m.DNSLog, "\n"))
		}
		if len(parts) > 0 {
			results = "\n\n" + strings.Join(parts, "\n\n")
		} else {
			results = "\n\nNo results collected."
		}
	}

	label := fmt.Sprintf("%s\n\n%s", stageText, bar)
	if m.Loaded {
		label = fmt.Sprintf("%s\n\n%s\n\nCompleted. Press esc to quit or b to go back.", stageText, bar) + results
	}

	return header + label
}
