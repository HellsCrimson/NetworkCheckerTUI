package main

import (
	"bufio"
	"context"
	"fmt"
	"network-check/utils"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Check proxy settings: gather env vars, git proxy, GNOME proxy (gsettings) and /etc/environment.
// Streams lines into m.ProxyLog and prepends a short summary on completion.
func UpdateProxy(msg tea.Msg, m Model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case FrameMsg:
		if !m.Loaded && m.ProxyChan == nil {
			m.ProxyChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()

				send := func(s string) {
					select {
					case ch <- s:
					default:
					}
				}

				// environment vars (both uppercase and lowercase)
				envVars := []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy", "ALL_PROXY", "all_proxy"}
				for _, v := range envVars {
					if val, ok := os.LookupEnv(v); ok && strings.TrimSpace(val) != "" {
						send(fmt.Sprintf("env %s=%s", v, val))
					}
				}

				// helper to run a command and stream output
				run := func(name string, args ...string) {
					cmd := exec.CommandContext(ctx, name, args...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						return
					}
					stderr, _ := cmd.StderrPipe()
					if err := cmd.Start(); err != nil {
						return
					}
					sc := bufio.NewScanner(stdout)
					for sc.Scan() {
						send(fmt.Sprintf("%s: %s", name, strings.TrimSpace(sc.Text())))
					}
					esc := bufio.NewScanner(stderr)
					for esc.Scan() {
						send(fmt.Sprintf("%s [err]: %s", name, strings.TrimSpace(esc.Text())))
					}
					_ = cmd.Wait()
				}

				// Try common probes (non-fatal if missing)
				// 1) env | grep -i proxy
				run("sh", "-c", "env | grep -i proxy || true")
				// 2) git proxy config
				run("git", "config", "--global", "--get", "http.proxy")
				run("git", "config", "--global", "--get", "https.proxy")
				// 3) GNOME proxy via gsettings
				run("gsettings", "list-recursively", "org.gnome.system.proxy")
				// 4) /etc/environment (may require read)
				run("sh", "-c", "if [ -r /etc/environment ]; then sed -n '1,200p' /etc/environment; fi")
				// 5) check common desktop env vars files
				run("sh", "-c", "if [ -r ~/.bashrc ]; then grep -i proxy ~/.bashrc || true; fi")
				run("sh", "-c", "if [ -r ~/.profile ]; then grep -i proxy ~/.profile || true; fi")

				// final helpful note if nothing was emitted
				// (since we already emitted env entries earlier, check channel not possible here;
				// emit a generic note so user isn't left with empty result)
				select {
				case ch <- "probe finished (see above). If empty, no proxy settings detected or access to system config was restricted.":
				default:
				}
			}(m.ProxyChan)

			return m, Frame()
		}

		// poll proxy channel
		if m.ProxyChan != nil {
			for {
				select {
				case line, ok := <-m.ProxyChan:
					if !ok {
						// finished -> try to add concise summary from collected lines
						if summary := summarizeProxy(m.ProxyLog); summary != "" {
							m.ProxyLog = append([]string{summary}, m.ProxyLog...)
						}
						m.ProxyChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.ProxyLog = append(m.ProxyLog, trim)
					return m, Frame()
				default:
					return m, Frame()
				}
			}
		}
	}
	return m, nil
}

func summarizeProxy(lines []string) string {
	// look for first explicit proxy setting in logs
	for _, l := range lines {
		ll := strings.ToLower(l)
		if strings.Contains(ll, "http_proxy") || strings.Contains(ll, "https_proxy") || strings.Contains(ll, "http.proxy") || strings.Contains(ll, "https.proxy") || strings.Contains(ll, "proxy") {
			// return a short human-friendly summary (first matching line)
			return "Proxy detected: " + l
		}
	}
	return ""
}

func ChosenProxyView(m Model) string {
	header := utils.KeywordStyle.Render("Proxy settings:") + " environment / git / desktop\n\n"

	if !m.Loaded {
		body := utils.SubtleStyle.Render("probing proxy settings...")
		if len(m.ProxyLog) > 0 {
			body = strings.Join(m.ProxyLog, "\n")
		}
		return header + body + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.ProxyLog) == 0 {
		return header + utils.SubtleStyle.Render("No proxy settings detected.") + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}
	return header + utils.SubtleStyle.Render(strings.Join(m.ProxyLog, "\n")) + "\n\n" + utils.SubtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
