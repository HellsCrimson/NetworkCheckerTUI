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

// Check VPN status: tries multiple common checks (NetworkManager active connections,
// wg show, ip link for tun/wg devices, systemctl status for common VPN services,
// pgrep for openvpn/strongswan/etc). Streams lines into m.VPNLog.

func updateVPN(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		if !m.Loaded && m.VPNChan == nil {
			m.VPNChan = make(chan string, 256)
			go func(ch chan<- string) {
				defer close(ch)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				tryCmd := func(name string, args ...string) bool {
					cmd := exec.CommandContext(ctx, name, args...)
					stdout, err := cmd.StdoutPipe()
					if err != nil {
						return false
					}
					stderr, _ := cmd.StderrPipe()
					if err := cmd.Start(); err != nil {
						return false
					}
					sc := bufio.NewScanner(stdout)
					had := false
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
					// also capture stderr (some commands print to stderr)
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
					return had
				}

				// 1) NetworkManager active connections (if nmcli present)
				if tryCmd("nmcli", "-t", "-f", "NAME,DEVICE,TYPE,STATE", "connection", "show", "--active") {
					// give UI a chance to show these lines
					time.Sleep(50 * time.Millisecond)
				}

				// 2) WireGuard
				if tryCmd("wg", "show") {
					time.Sleep(50 * time.Millisecond)
				}
				// 3) check common devices
				tryCmd("ip", "link", "show", "dev", "tun0")
				tryCmd("ip", "link", "show", "dev", "wg0")

				// 4) systemctl statuses for common VPN services (may require privileges; capture output)
				tryCmd("systemctl", "status", "openvpn", "--no-pager")
				tryCmd("systemctl", "status", "openvpn@client", "--no-pager")
				tryCmd("systemctl", "status", "strongswan", "--no-pager")
				tryCmd("systemctl", "status", "wireguard", "--no-pager")
				tryCmd("systemctl", "status", "wg-quick@wg0", "--no-pager")

				// 5) check for processes
				tryCmd("pgrep", "-a", "openvpn")
				tryCmd("pgrep", "-a", "wireguard")
				tryCmd("pgrep", "-a", "strongswan")
				tryCmd("pgrep", "-a", "openconnect")

				// If nothing produced, emit a helpful message
				select {
				case ch <- "no VPN indicators found (binaries missing or not running)":
				default:
				}
			}(m.VPNChan)

			return m, frame()
		}

		// poll VPN channel
		if m.VPNChan != nil {
			for {
				select {
				case line, ok := <-m.VPNChan:
					if !ok {
						// finished
						m.VPNChan = nil
						m.Loaded = true
						return m, nil
					}
					trim := strings.TrimSpace(line)
					if trim == "" {
						continue
					}
					m.VPNLog = append(m.VPNLog, trim)
					return m, frame()
				default:
					return m, frame()
				}
			}
		}
	}
	return m, nil
}

func chosenVPNView(m model) string {
	header := keywordStyle.Render("VPN status:") + " common checks (nmcli/wg/systemctl/pgrep)\n\n"

	if !m.Loaded {
		body := subtleStyle.Render("probing VPN status...")
		if len(m.VPNLog) > 0 {
			body = strings.Join(m.VPNLog, "\n")
		}
		return header + body + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	if len(m.VPNLog) == 0 {
		return header + subtleStyle.Render("No VPN activity detected or commands failed.") + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
	}

	// show collected output
	return header + subtleStyle.Render(strings.Join(m.VPNLog, "\n")) + "\n\n" + subtleStyle.Render("Completed. Press esc to quit or b to go back.")
}
