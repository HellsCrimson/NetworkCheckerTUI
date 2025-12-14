package utils

type PingResult struct {
	Index   int
	Success bool
	Done    bool
}

type MtuResult struct {
	Size    int
	Success bool
	Done    bool
}

type DnsResult struct {
	Name    string
	Addrs   []string
	Success bool
	Done    bool
}
