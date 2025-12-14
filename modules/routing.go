package modules

import (
	"bufio"
	"context"
	"network-check/utils"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check routing tables: runs "ip route show" (preferred) or falls back to "route -n".
// Streams lines into a channel from a goroutine and collects them into m.RouteLog.
func UpdateRouting(msg tea.Msg, m utils.Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case utils.FrameMsg:
		if !m.Loaded && m.RouteChan == nil {
			m.RouteChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				// try "ip route show" first
				cmd := exec.CommandContext(ctx, "ip", "route", "show")
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

				// fallback to "route -n"
				cmd = exec.CommandContext(ctx, "route", "-n")
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
				case ch <- "could not run 'ip route' or 'route -n' (permission or binary missing)":
				default:
				}
			}(m.RouteChan)

			return m, utils.Frame()
		}

		// poll route channel
		if m.RouteChan != nil {
			for {
				select {
				case line, ok := <-m.RouteChan:
					if !ok {
						// finished
						m.RouteChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.RouteLog = append(m.RouteLog, trim)
					return m, utils.Frame()
				default:
					return m, utils.Frame()
				}
			}
		}
	}
	return m, nil
}

func ChosenRoutingView(m utils.Model) string {
	header := utils.KeywordStyle.Render("Routing tables:") + " ip route / route -n\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("querying routing table...")
		if len(m.RouteLog) > 0 {
			body = strings.Join(m.RouteLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// finished: show collected routing entries or message
	if len(m.RouteLog) == 0 {
		return header + utils.SubtleStyle.Render("No routing entries collected or command failed.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.RouteLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
