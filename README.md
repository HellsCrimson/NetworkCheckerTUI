# network-check

A TUI for quick network diagnostics built with Bubble Tea and Lip Gloss. Each check runs common system tools (ip/ss/ping/traceroute/etc...) in goroutines and streams their output into the UI.

## Features

- Interactive menu with checks:
  - Full network check, IP/routing, DNS, MTU, frame analyzer, DHCP
  - ARP, routing tables, firewall, open ports, traceroute
  - Bandwidth (speedtest), latency (ping), packet loss
  - VPN status, Wi‑Fi signal, network interfaces, proxy settings
  - NAT configuration, QoS settings
- Non-blocking UI: commands stream output progressively.
- Lightweight heuristics and fallbacks for common tools.

## Requirements

- Go 1.20+
- Linux environment with typical networking utilities for best results (some checks have fallbacks). Example tools:
  - ip, ss, iptables, nft, arp, route, traceroute/tracepath, ping
  - speedtest / speedtest-cli / fast (optional)
  - nmcli / iw / iwconfig (Wi‑Fi)
  - tc (QoS)
- Some commands may require elevated privileges (root) to return full information.

## Build

From the project root:
```bash
make build
```

For cross compilation:
```bash
make cross-build
```

Run without building:
```bash
go run .
```

## Run

Launch the program:
```bash
./network-check
```

If a check needs root to be useful, run that check with sudo:
```bash
sudo ./network-check
```

## Controls

- j / down — move selection down
- k / up — move selection up
- enter / space — run selected check
- b — go back to menu after a check completes
- q / esc / Ctrl+C — quit

When a check runs, output is streamed to the view. After completion the view shows collected output and a completion note.

## Project layout

- main.go — TUI wiring, model, menu routing
- One file per check (e.g. arp.go, routing.go, firewall.go, traceroute.go, bandwidth.go, latency.go, packet_loss.go, vpn.go, wifi.go, netif.go, proxy.go, nat.go, qos.go)
- Each check follows the same pattern: spawn a goroutine to run commands, stream lines into a buffered channel, UI polls channel on frames and appends to a log slice.

## Extending

- Add a new check:
  - create a new file with update<CheckName>(msg, m) and chosen<CheckName>View(m)
  - add channel/log fields to `model` in main.go
  - add the choice text to initialModel.AvailableChoices
  - wire update/view into updateChosen and chosenView switches
- Keep timeouts and non-blocking streaming behavior consistent.

## Notes & troubleshooting

- Empty results typically mean the tool isn't installed, lacks privileges, or no relevant data is present (e.g., no Wi‑Fi device).
- Timeouts are conservative to avoid hanging the UI; adjust if needed for slow environments.
