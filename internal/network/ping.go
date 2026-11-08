package network

import (
	"fmt"
	"net"
	"time"
)

// PingResult is one ICMP echo attempt.
type PingResult struct {
	Seq       int    `json:"seq"`
	Address   string `json:"address"`
	RTTMillis int64  `json:"rttMs"`
	OK        bool   `json:"ok"`
	Status    string `json:"status"`
}

// Pinger sends one ICMP echo request.
type Pinger interface {
	Ping(ip net.IP, timeout time.Duration) (PingResult, error)
}

// ResolveIPv4 returns the first IPv4 address for a host name or literal.
func ResolveIPv4(host string) (net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
		return nil, fmt.Errorf("%s is not an IPv4 address; ping supports IPv4 only", host)
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	for _, a := range addrs {
		if v4 := a.To4(); v4 != nil {
			return v4, nil
		}
	}
	return nil, fmt.Errorf("%s has no IPv4 address", host)
}

// PingStats summarizes a series of results.
type PingStats struct {
	Sent, Received      int
	MinMS, AvgMS, MaxMS int64
}

// Summarize computes loss and round-trip statistics.
func Summarize(results []PingResult) PingStats {
	s := PingStats{Sent: len(results)}
	var total int64
	for _, r := range results {
		if !r.OK {
			continue
		}
		if s.Received == 0 || r.RTTMillis < s.MinMS {
			s.MinMS = r.RTTMillis
		}
		if r.RTTMillis > s.MaxMS {
			s.MaxMS = r.RTTMillis
		}
		total += r.RTTMillis
		s.Received++
	}
	if s.Received > 0 {
		s.AvgMS = total / int64(s.Received)
	}
	return s
}

// LossPercent is the share of requests that got no reply.
func (s PingStats) LossPercent() float64 {
	if s.Sent == 0 {
		return 0
	}
	return float64(s.Sent-s.Received) / float64(s.Sent) * 100
}
